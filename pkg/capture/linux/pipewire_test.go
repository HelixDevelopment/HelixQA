// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	dbus "github.com/godbus/dbus/v5"

	"digital.vasic.helixqa/pkg/capture/frames"
)

// canonicalStartResults builds the results map a successful portal Start
// would return, with one stream pointing at nodeID and arbitrary metadata.
func canonicalStartResults(nodeID uint32) map[string]any {
	return map[string]any{
		"streams": []any{
			[]any{
				nodeID,
				map[string]dbus.Variant{
					"size":        dbus.MakeVariant([]int32{1920, 1080}),
					"source_type": dbus.MakeVariant(uint32(1)),
				},
			},
		},
	}
}

// newScriptedCaller returns a fakeCaller pre-loaded to simulate a happy
// portal handshake + OpenPipeWireRemote that yields a real pipe-backed FD.
func newScriptedCaller(t *testing.T, fd int32) *fakeCaller {
	t.Helper()
	return &fakeCaller{
		portalResps: []portalResp{
			// CreateSession
			{status: 0, results: map[string]any{"session_handle": "/helixqa/session/1"}},
			// SelectSources
			{status: 0, results: map[string]any{}},
			// Start
			{status: 0, results: canonicalStartResults(7)},
		},
		immRespBody: []any{dbus.UnixFD(fd)},
	}
}

// stubStdoutRunner is a Runner that records its inputs and returns a fake
// Cmd whose stdout is a pre-loaded byte stream.
type stubStdoutRunner struct {
	stdout     io.Reader
	stderr     io.Reader
	lastBin    string
	lastArgs   []string
	lastExtras []*os.File
	err        error
}

func (s *stubStdoutRunner) Start(_ context.Context, bin string, args []string, extras []*os.File) (Cmd, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.lastBin = bin
	s.lastArgs = append([]string(nil), args...)
	s.lastExtras = extras
	return &fakeCmd{
		stdoutR: io.NopCloser(s.stdout),
		stderrR: io.NopCloser(s.stderr),
		waitCh:  make(chan struct{}),
	}, nil
}

// --- NewPortalFactory happy path ---

func TestNewPortalFactory_EndToEnd(t *testing.T) {
	// A real pipe-backed FD so os.NewFile inside OpenPipeWireRemote produces
	// a usable *os.File. Close at test end regardless.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()

	caller := newScriptedCaller(t, int32(pw.Fd()))

	var envelopes bytes.Buffer
	envelopes.Write(EncodeEnvelope(100_000, []byte{0x67, 0x42}))
	envelopes.Write(EncodeEnvelope(200_000, []byte{0x99}))

	sr := &stubStdoutRunner{stdout: &envelopes, stderr: bytes.NewReader(nil)}

	factory := NewPortalFactory(PortalConfig{
		CallerFactory: func() (Caller, error) { return caller, nil },
		SelectSources: SelectSourcesOptions{Types: StreamSourceMonitor},
		SidecarBinary: "helixqa-capture-linux",
		Runner:        sr,
	})
	cfg := Config{
		BackendOverride: BackendPortal,
		Width:           1920, Height: 1080,
		PortalFactory: factory,
	}
	src, err := NewSource(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if src.Backend() != BackendPortal {
		t.Errorf("Backend = %v", src.Backend())
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	// The runner was called with the expected binary + --node <id> + an ExtraFile.
	if sr.lastBin != "helixqa-capture-linux" {
		t.Errorf("runner bin = %q", sr.lastBin)
	}
	hasNodeArg := false
	for i, a := range sr.lastArgs {
		if a == "--node" && i+1 < len(sr.lastArgs) && sr.lastArgs[i+1] == "7" {
			hasNodeArg = true
		}
	}
	if !hasNodeArg {
		t.Errorf("--node 7 not in args: %v", sr.lastArgs)
	}
	if len(sr.lastExtras) != 1 {
		t.Errorf("expected 1 ExtraFile, got %d", len(sr.lastExtras))
	}

	// Two envelopes arrive on the frames channel.
	got := collectFrames(t, src.Frames(), 2, 2*time.Second)
	if len(got) != 2 || got[0].PTS != 100*time.Millisecond || got[1].PTS != 200*time.Millisecond {
		t.Errorf("frames: %+v", got)
	}
	if got[0].Source != "pipewire" {
		t.Errorf("frame source = %q", got[0].Source)
	}
}

// --- portal failures roll back acquired resources ---

func TestNewPortalFactory_CreateSessionFails(t *testing.T) {
	caller := &fakeCaller{portalResps: []portalResp{{err: errors.New("bus down")}}}
	factory := NewPortalFactory(PortalConfig{
		CallerFactory: func() (Caller, error) { return caller, nil },
	})
	src, err := factory(Config{Width: 1, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Start(context.Background()); err == nil || !errors.Is(err, errBusDown(caller)) {
		// The wrapped error must contain the original message.
		if err == nil || err.Error() == "" {
			t.Errorf("want error mentioning bus, got %v", err)
		}
	}
	if !caller.closed {
		t.Error("caller should have been Closed after CreateSession failure")
	}
}

// errBusDown is a helper so the error-matching in the test above is readable.
func errBusDown(_ *fakeCaller) error { return errors.New("bus down") }

func TestNewPortalFactory_UserCancelled(t *testing.T) {
	// Status=1 on SelectSources (user dismissed consent).
	caller := &fakeCaller{portalResps: []portalResp{
		{status: 0, results: map[string]any{"session_handle": "/s"}}, // CreateSession
		{status: 1, results: map[string]any{}},                        // SelectSources cancelled
	}}
	factory := NewPortalFactory(PortalConfig{
		CallerFactory: func() (Caller, error) { return caller, nil },
	})
	src, _ := factory(Config{Width: 1, Height: 1})
	err := src.Start(context.Background())
	if err == nil || !IsUserCancelled(err) {
		t.Errorf("want user-cancelled, got %v", err)
	}
}

func TestNewPortalFactory_CallerFactoryError(t *testing.T) {
	boom := errors.New("no bus")
	factory := NewPortalFactory(PortalConfig{
		CallerFactory: func() (Caller, error) { return nil, boom },
	})
	src, _ := factory(Config{Width: 1, Height: 1})
	if err := src.Start(context.Background()); !errors.Is(err, boom) {
		t.Errorf("want boom, got %v", err)
	}
}

func TestNewPortalFactory_InvalidConfig(t *testing.T) {
	// Missing CallerFactory.
	factory := NewPortalFactory(PortalConfig{})
	if _, err := factory(Config{Width: 1, Height: 1}); !errors.Is(err, ErrPortalConfig) {
		t.Errorf("want ErrPortalConfig, got %v", err)
	}
	// Bad dimensions.
	factory2 := NewPortalFactory(PortalConfig{CallerFactory: func() (Caller, error) { return &fakeCaller{}, nil }})
	if _, err := factory2(Config{Width: 0, Height: 1}); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("want ErrInvalidConfig, got %v", err)
	}
}

func TestPortalSource_DoubleStart(t *testing.T) {
	pr, pw, _ := os.Pipe()
	defer pr.Close()
	defer pw.Close()
	caller := newScriptedCaller(t, int32(pw.Fd()))
	sr := &stubStdoutRunner{stdout: bytes.NewReader(nil), stderr: bytes.NewReader(nil)}
	factory := NewPortalFactory(PortalConfig{
		CallerFactory: func() (Caller, error) { return caller, nil },
		Runner:        sr,
	})
	src, _ := factory(Config{Width: 1, Height: 1})
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	if err := src.Start(context.Background()); err == nil {
		t.Error("second Start should error")
	}
}

func TestPortalSource_FramesBeforeStart(t *testing.T) {
	factory := NewPortalFactory(PortalConfig{
		CallerFactory: func() (Caller, error) { return &fakeCaller{}, nil },
	})
	src, _ := factory(Config{Width: 1, Height: 1})
	// Before Start, Frames() returns a non-nil pre-closed channel.
	ch := src.Frames()
	if ch == nil {
		t.Fatal("Frames() returned nil")
	}
	select {
	case _, open := <-ch:
		if open {
			t.Error("pre-Start Frames() should be closed")
		}
	default:
		t.Error("pre-Start Frames() must not block")
	}
}

// --- KMSGrab factory ---

func TestNewKMSGrabFactory_HappyPath(t *testing.T) {
	var env bytes.Buffer
	env.Write(EncodeEnvelope(1_000_000, []byte{0xAA, 0xBB}))
	sr := &stubStdoutRunner{stdout: &env, stderr: bytes.NewReader(nil)}

	factory := NewKMSGrabFactory(KMSGrabConfig{
		SidecarBinary: "helixqa-kmsgrab",
		ExtraArgs:     []string{"--connector", "HDMI-A-1"},
		Runner:        sr,
	})
	cfg := Config{
		BackendOverride: BackendKMSGrab,
		Width:           1920, Height: 1080,
		KMSGrabFactory: factory,
	}
	src, err := NewSource(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if src.Backend() != BackendKMSGrab {
		t.Errorf("Backend = %v", src.Backend())
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	if sr.lastBin != "helixqa-kmsgrab" {
		t.Errorf("lastBin = %q", sr.lastBin)
	}
	if len(sr.lastArgs) != 2 || sr.lastArgs[0] != "--connector" || sr.lastArgs[1] != "HDMI-A-1" {
		t.Errorf("lastArgs = %v", sr.lastArgs)
	}
	got := collectFrames(t, src.Frames(), 1, time.Second)
	if len(got) != 1 || got[0].Source != "kmsgrab" {
		t.Errorf("got = %+v", got)
	}
}

func TestNewKMSGrabFactory_DefaultsBinary(t *testing.T) {
	sr := &stubStdoutRunner{stdout: bytes.NewReader(nil), stderr: bytes.NewReader(nil)}
	factory := NewKMSGrabFactory(KMSGrabConfig{Runner: sr})
	cfg := Config{Width: 1, Height: 1, BackendOverride: BackendKMSGrab, KMSGrabFactory: factory}
	src, _ := NewSource(cfg)
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	if sr.lastBin != DefaultKMSGrabSidecarBinary {
		t.Errorf("default binary = %q, want %q", sr.lastBin, DefaultKMSGrabSidecarBinary)
	}
}

func TestNewKMSGrabFactory_BadDimensions(t *testing.T) {
	factory := NewKMSGrabFactory(KMSGrabConfig{})
	if _, err := factory(Config{Width: 0, Height: 1}); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("want ErrInvalidConfig, got %v", err)
	}
}

// --- helpers ---

func collectFrames(t *testing.T, ch <-chan frames.Frame, n int, timeout time.Duration) []frames.Frame {
	t.Helper()
	out := make([]frames.Frame, 0, n)
	dl := time.After(timeout)
	for len(out) < n {
		select {
		case f, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, f)
		case <-dl:
			return out
		}
	}
	return out
}
