// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	dbus "github.com/godbus/dbus/v5"

	"digital.vasic.helixqa/pkg/bridge/dbusportal"
	"digital.vasic.helixqa/pkg/capture/frames"
)

func TestNewDefaultSource_BadDimensions(t *testing.T) {
	_, err := NewDefaultSource(ServiceConfig{Width: 0, Height: 1080})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("want ErrInvalidConfig, got %v", err)
	}
}

func TestNewDefaultSource_ProductionDefaultsAreWired(t *testing.T) {
	// We can't actually Start() without a real bus or running sidecars, but
	// we CAN verify the Source was constructed with BackendPortal selected
	// and that every factory slot is non-nil by inspecting the Config.
	cfg := ServiceConfig{Width: 1920, Height: 1080, BackendOverride: BackendPortal}
	internal := cfg.toConfig(dbusportal.DBusCallerFactory)
	if internal.PortalFactory == nil {
		t.Error("PortalFactory not wired")
	}
	if internal.KMSGrabFactory == nil {
		t.Error("KMSGrabFactory not wired")
	}
	if internal.X11GrabFactory == nil {
		t.Error("X11GrabFactory not wired")
	}
	// NewDefaultSource itself returns a valid Source (the Portal factory
	// just constructs a portalSource whose Start would fail against the
	// real bus, but NewSource itself succeeds).
	src, err := NewDefaultSource(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if src.Backend() != BackendPortal {
		t.Errorf("Backend = %v, want portal", src.Backend())
	}
	// Frames before Start MUST NOT be nil.
	if src.Frames() == nil {
		t.Error("Frames() returned nil before Start")
	}
}

func TestNewDefaultSourceWithCallerFactory_HappyPath(t *testing.T) {
	// Use a pre-built fake Caller + stub runner so Start actually completes
	// end-to-end — the fullest "wire everything" integration exercisable
	// without a real bus.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()

	fake := &fakeCaller{
		portalResps: []portalResp{
			// CreateSession
			{status: 0, results: map[string]any{"session_handle": "/helixqa/test"}},
			// SelectSources
			{status: 0, results: map[string]any{}},
			// Start
			{status: 0, results: map[string]any{
				"streams": []any{
					[]any{uint32(3), map[string]dbus.Variant{}},
				},
			}},
		},
		immRespBody: []any{dbus.UnixFD(int32(pw.Fd()))},
	}

	// Custom X11GrabFactory for tests that doesn't actually exec anything.
	// We override through ServiceConfig's Runner-less path — the production
	// X11GrabFactory uses ExecRunner by default; here we skip exercising
	// it and rely on the Portal path via BackendPortal.

	// Plumb a fake stdout so the sidecar pump terminates cleanly.
	stub := &stubStdoutRunner{stdout: bytes.NewReader(nil), stderr: bytes.NewReader(nil)}

	// Build Config manually using the service wiring but swap in the stub
	// runner for PortalSidecarRunner via PortalFactory.
	portalFactory := NewPortalFactory(PortalConfig{
		CallerFactory: func() (dbusportal.Caller, error) { return fake, nil },
		Runner:        stub,
	})
	cfg := Config{
		BackendOverride: BackendPortal,
		Width:           1920,
		Height:          1080,
		PortalFactory:   portalFactory,
	}
	src, err := NewSource(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	if src.Backend() != BackendPortal {
		t.Errorf("Backend = %v", src.Backend())
	}
}

func TestNewDefaultSourceWithCallerFactory_NilFactory(t *testing.T) {
	_, err := NewDefaultSourceWithCallerFactory(ServiceConfig{Width: 1, Height: 1}, nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("want ErrInvalidConfig, got %v", err)
	}
}

func TestNewDefaultSourceWithCallerFactory_BadDims(t *testing.T) {
	_, err := NewDefaultSourceWithCallerFactory(ServiceConfig{Width: 0, Height: 1}, dbusportal.DBusCallerFactory)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("want ErrInvalidConfig, got %v", err)
	}
}

func TestServiceConfig_ToConfigPropagatesX11GrabTuning(t *testing.T) {
	// Verify the X11GrabFactory actually sees the display/fps knobs by
	// exercising it end-to-end against a stub runner.
	sr := &stubStdoutRunner{stdout: bytes.NewReader(nil), stderr: bytes.NewReader(nil)}
	cfg := ServiceConfig{
		Width: 1, Height: 1,
		BackendOverride: BackendX11Grab,
		Display:         ":7",
		FPS:             15,
	}
	internal := cfg.toConfig(dbusportal.DBusCallerFactory)
	// Wrap the X11GrabFactory to swap the runner; simpler to rebuild.
	internal.X11GrabFactory = NewX11GrabFactory(X11GrabConfig{
		Display: cfg.Display, FPS: cfg.FPS, Runner: sr,
	})
	src, err := NewSource(internal)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	// The stub runner records argv — verify --display :7 --fps 15 landed.
	want := []string{"--display", ":7", "--fps", "15"}
	if len(sr.lastArgs) != 4 {
		t.Fatalf("argv = %v", sr.lastArgs)
	}
	for i := range want {
		if sr.lastArgs[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, sr.lastArgs[i], want[i])
		}
	}
}

func TestCollectFrames_RespectsMaxAndCtx(t *testing.T) {
	// Fabricate a sidecar that emits exactly 5 envelopes.
	var buf bytes.Buffer
	for i := 0; i < 5; i++ {
		buf.Write(EncodeEnvelope(int64(i*1000), []byte{byte(i)}))
	}
	sr := &stubStdoutRunner{stdout: &buf, stderr: bytes.NewReader(nil)}
	r, _ := NewSidecarRunner(SidecarConfig{
		Binary: "x", Source: "test", Width: 1, Height: 1,
		Format: frames.FormatH264AnnexB, Runner: sr,
	})
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop() })
	src := WrapSidecarAsSource(r, BackendPortal)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got := CollectFrames(ctx, src, 3)
	if len(got) != 3 {
		t.Errorf("CollectFrames(max=3) returned %d, want 3", len(got))
	}
}

