// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewX11GrabFactory_ArgShape(t *testing.T) {
	var env bytes.Buffer
	env.Write(EncodeEnvelope(1_500_000, []byte{0xAB, 0xCD}))
	sr := &stubStdoutRunner{stdout: &env, stderr: bytes.NewReader(nil)}

	factory := NewX11GrabFactory(X11GrabConfig{
		SidecarBinary: "helixqa-x11grab",
		Display:       ":99",
		FPS:           60,
		ExtraArgs:     []string{"--region", "0,0,1920,1080"},
		Runner:        sr,
	})
	cfg := Config{
		BackendOverride: BackendX11Grab,
		Width:           1920, Height: 1080,
		X11GrabFactory: factory,
	}
	src, err := NewSource(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if src.Backend() != BackendX11Grab {
		t.Errorf("Backend = %v", src.Backend())
	}
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	if sr.lastBin != "helixqa-x11grab" {
		t.Errorf("bin = %q", sr.lastBin)
	}
	// Expected argv: --display :99 --fps 60 --region 0,0,1920,1080
	want := []string{"--display", ":99", "--fps", "60", "--region", "0,0,1920,1080"}
	if len(sr.lastArgs) != len(want) {
		t.Fatalf("argv = %v, want %v", sr.lastArgs, want)
	}
	for i := range want {
		if sr.lastArgs[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q (full %v)", i, sr.lastArgs[i], want[i], sr.lastArgs)
		}
	}
	got := collectFrames(t, src.Frames(), 1, time.Second)
	if len(got) != 1 || got[0].Source != "x11grab" {
		t.Errorf("frame: %+v", got)
	}
}

func TestNewX11GrabFactory_DefaultsDisplay(t *testing.T) {
	sr := &stubStdoutRunner{stdout: bytes.NewReader(nil), stderr: bytes.NewReader(nil)}
	factory := NewX11GrabFactory(X11GrabConfig{Runner: sr})
	cfg := Config{
		Width: 1, Height: 1, BackendOverride: BackendX11Grab, X11GrabFactory: factory,
	}
	src, _ := NewSource(cfg)
	if err := src.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Stop() })
	// Expect --display :0 (default) and no --fps (zero).
	want := []string{"--display", DefaultX11GrabDisplay}
	if len(sr.lastArgs) != 2 {
		t.Fatalf("argv = %v, want %v (no --fps when FPS==0)", sr.lastArgs, want)
	}
	for i := range want {
		if sr.lastArgs[i] != want[i] {
			t.Errorf("arg[%d] = %q, want %q", i, sr.lastArgs[i], want[i])
		}
	}
	if sr.lastBin != DefaultX11GrabSidecarBinary {
		t.Errorf("bin = %q, want %q", sr.lastBin, DefaultX11GrabSidecarBinary)
	}
}

func TestNewX11GrabFactory_BadDimensions(t *testing.T) {
	factory := NewX11GrabFactory(X11GrabConfig{})
	if _, err := factory(Config{Width: 0, Height: 1}); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("want ErrInvalidConfig, got %v", err)
	}
}

func TestNewX11GrabFactory_MissingBinary_PropagatesRunnerError(t *testing.T) {
	boom := errors.New("exec: \"helixqa-x11grab\": executable file not found in $PATH")
	sr := &stubStdoutRunner{err: boom}
	factory := NewX11GrabFactory(X11GrabConfig{Runner: sr})
	cfg := Config{Width: 1, Height: 1, BackendOverride: BackendX11Grab, X11GrabFactory: factory}
	src, err := NewSource(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Start(context.Background()); !errors.Is(err, boom) {
		t.Errorf("want missing-binary error, got %v", err)
	}
}
