// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"digital.vasic.helixqa/pkg/capture/frames"
)

// DefaultPortalSidecarBinary is the name the operator is expected to install
// (either via the HelixQA release image or by building cmd/helixqa-capture-linux).
const DefaultPortalSidecarBinary = "helixqa-capture-linux"

// CallerFactory constructs a Caller on demand. Production wires
// NewDBusCaller (portal_dbus.go, lands in a future commit); tests inject
// a closure that returns a fakeCaller.
type CallerFactory func() (Caller, error)

// PortalConfig drives NewPortalFactory. Only CallerFactory is strictly
// required; the rest have sensible defaults.
type PortalConfig struct {
	// CallerFactory returns the Caller used to drive the portal handshake.
	// Required.
	CallerFactory CallerFactory

	// SelectSources tunes the portal's SelectSources call — what kind of
	// source (monitor/window/virtual), cursor mode, persist strategy.
	// Zero values are valid: SelectSources defaults to Monitor + Hidden.
	SelectSources SelectSourcesOptions

	// ParentWindow is passed to portal Start. Empty is fine for headless
	// QA runs where no parent UI exists.
	ParentWindow string

	// SidecarBinary defaults to DefaultPortalSidecarBinary when empty.
	SidecarBinary string

	// ExtraSidecarArgs are prepended before HelixQA-constructed args
	// (--node <id>). Useful for per-deployment flags like --bitrate.
	ExtraSidecarArgs []string

	// Runner is the sidecar process spawner. Nil defaults to ExecRunner.
	Runner Runner
}

// ErrPortalConfig is returned for malformed PortalConfig.
var ErrPortalConfig = errors.New("linux/capture: invalid PortalConfig")

// NewPortalFactory returns a BackendFactory that wires the ScreenCast portal
// handshake to a SidecarRunner. The returned factory is suitable for
// Config.PortalFactory — the real handshake happens at Source.Start, not at
// construction time, so NewSource stays cheap.
func NewPortalFactory(pc PortalConfig) BackendFactory {
	return func(cfg Config) (Source, error) {
		if pc.CallerFactory == nil {
			return nil, fmt.Errorf("%w: CallerFactory required", ErrPortalConfig)
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return nil, fmt.Errorf("%w: bad dimensions (%dx%d)", ErrInvalidConfig, cfg.Width, cfg.Height)
		}
		return newPortalSource(pc, cfg), nil
	}
}

// portalSource is a Source that performs the full portal+sidecar chain on Start.
type portalSource struct {
	pc  PortalConfig
	cfg Config

	// Populated in Start; guarded by startMu so concurrent Stop during
	// Start cannot observe a partially-populated state.
	startMu  sync.Mutex
	portal   *Portal
	runner   *SidecarRunner
	fdFile   *os.File
	started  bool
	stopOnce sync.Once
	empty    chan frames.Frame // used as Frames() before Start
}

func newPortalSource(pc PortalConfig, cfg Config) *portalSource {
	e := make(chan frames.Frame)
	close(e) // pre-closed so callers who poll before Start don't block
	return &portalSource{pc: pc, cfg: cfg, empty: e}
}

// Start performs: CallerFactory → NewPortal → CreateSession → SelectSources →
// Start → OpenPipeWireRemote → SidecarRunner.Start. On any failure the
// resources acquired up to that point are released.
func (s *portalSource) Start(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.started {
		return errors.New("linux/capture: portalSource.Start already called")
	}
	caller, err := s.pc.CallerFactory()
	if err != nil {
		return fmt.Errorf("linux/capture: CallerFactory: %w", err)
	}
	portal := NewPortal(caller)

	sessPath, err := portal.CreateSession(ctx)
	if err != nil {
		_ = portal.Close()
		return err
	}
	if err := portal.SelectSources(ctx, sessPath, s.pc.SelectSources); err != nil {
		_ = portal.Close()
		return err
	}
	startRes, err := portal.Start(ctx, sessPath, s.pc.ParentWindow)
	if err != nil {
		_ = portal.Close()
		return err
	}
	fd, err := portal.OpenPipeWireRemote(ctx, sessPath)
	if err != nil {
		_ = portal.Close()
		return err
	}
	// Node id is streams[0].NodeID; SelectSources with Multiple=false guarantees
	// exactly one stream. Multiple=true callers must pick the right stream
	// themselves via a higher-level API.
	nodeID := startRes.Streams[0].NodeID

	bin := s.pc.SidecarBinary
	if bin == "" {
		bin = DefaultPortalSidecarBinary
	}
	args := append([]string(nil), s.pc.ExtraSidecarArgs...)
	args = append(args, "--node", fmt.Sprintf("%d", nodeID))

	runnerCfg := SidecarConfig{
		Binary:        bin,
		Args:          args,
		ExtraFiles:    []*os.File{fd},
		Source:        "pipewire",
		Width:         s.cfg.Width,
		Height:        s.cfg.Height,
		Format:        frames.FormatH264AnnexB,
		ChannelBuffer: s.cfg.ChannelBuffer,
		Runner:        s.pc.Runner,
	}
	runner, err := NewSidecarRunner(runnerCfg)
	if err != nil {
		_ = fd.Close()
		_ = portal.Close()
		return err
	}
	if err := runner.Start(ctx); err != nil {
		_ = fd.Close()
		_ = portal.Close()
		return err
	}
	s.portal = portal
	s.runner = runner
	s.fdFile = fd
	s.started = true
	return nil
}

// Frames returns the runner's frame channel, or a pre-closed channel if Start
// has not been called. The contract is never "nil channel" — callers can
// always range over the result.
func (s *portalSource) Frames() <-chan frames.Frame {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if s.runner == nil {
		return s.empty
	}
	return s.runner.Frames()
}

// Stop tears down the runner + portal. Idempotent.
func (s *portalSource) Stop() error {
	var firstErr error
	s.stopOnce.Do(func() {
		s.startMu.Lock()
		runner, portal := s.runner, s.portal
		s.startMu.Unlock()
		if runner != nil {
			if err := runner.Stop(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		// runner.Stop closes fdFile via exec.Cmd teardown; not our job.
		if portal != nil {
			if err := portal.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	})
	return firstErr
}

// Backend returns BackendPortal.
func (s *portalSource) Backend() Backend { return BackendPortal }
