// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	capturelinux "digital.vasic.helixqa/pkg/capture/linux"
	"digital.vasic.helixqa/pkg/capture/frames"
)

func TestParseArgv_Health(t *testing.T) {
	_, health, err := parseArgv([]string{"--health"})
	if err != nil {
		t.Fatal(err)
	}
	if !health {
		t.Fatal("--health not detected")
	}
}

func TestParseArgv_RequiredFlags(t *testing.T) {
	cfg, health, err := parseArgv([]string{"--width", "1920", "--height", "1080", "--duration", "2s"})
	if err != nil {
		t.Fatal(err)
	}
	if health {
		t.Fatal("unexpected health mode")
	}
	if cfg.Width != 1920 || cfg.Height != 1080 || cfg.Duration != 2*time.Second {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Platform != "linux" {
		t.Errorf("default platform = %q, want linux", cfg.Platform)
	}
}

func TestParseArgv_AllFlags(t *testing.T) {
	cfg, _, err := parseArgv([]string{
		"--platform", "LINUX",
		"--backend", "x11grab",
		"--width", "1280",
		"--height", "720",
		"--fps", "60",
		"--display", ":7",
		"--duration", "10s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Platform != "linux" {
		t.Errorf("platform should be lowercased: %q", cfg.Platform)
	}
	if cfg.Backend != "x11grab" || cfg.FPS != 60 || cfg.Display != ":7" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestParseArgv_Invalid(t *testing.T) {
	_, _, err := parseArgv([]string{"--nonexistent"})
	if err == nil {
		t.Error("unknown flag should error")
	}
}

func TestValidateConfig(t *testing.T) {
	ok := demoConfig{Platform: "linux", Width: 1, Height: 1, Duration: time.Second}
	if err := validateConfig(ok); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
	bad := []struct {
		name string
		cfg  demoConfig
	}{
		{"wrong-platform", demoConfig{Platform: "mac", Width: 1, Height: 1, Duration: time.Second}},
		{"zero-width", demoConfig{Platform: "linux", Height: 1, Duration: time.Second}},
		{"zero-height", demoConfig{Platform: "linux", Width: 1, Duration: time.Second}},
		{"zero-duration", demoConfig{Platform: "linux", Width: 1, Height: 1}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateConfig(tc.cfg); err == nil {
				t.Error("expected error")
			}
		})
	}
}

// --- fakeSource (implements capturelinux.Source) ---

type fakeSource struct {
	ch       chan frames.Frame
	backend  capturelinux.Backend
	startErr error
	started  bool
	stopped  bool
}

func (s *fakeSource) Start(ctx context.Context) error {
	if s.startErr != nil {
		return s.startErr
	}
	s.started = true
	return nil
}
func (s *fakeSource) Frames() <-chan frames.Frame   { return s.ch }
func (s *fakeSource) Stop() error                   { s.stopped = true; return nil }
func (s *fakeSource) Backend() capturelinux.Backend { return s.backend }

func TestRun_HappyPath_PrintsPerFrameLines(t *testing.T) {
	ch := make(chan frames.Frame, 2)
	f1, _ := frames.New(100*time.Millisecond, 1920, 1080, frames.FormatH264AnnexB, "pipewire", []byte{0xAA, 0xBB})
	f2, _ := frames.New(200*time.Millisecond, 1920, 1080, frames.FormatH264AnnexB, "pipewire", []byte{0xCC, 0xDD, 0xEE})
	ch <- f1
	ch <- f2
	close(ch)
	fake := &fakeSource{ch: ch, backend: capturelinux.BackendPortal}

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := run(ctx, demoConfig{
		Platform: "linux", Width: 1920, Height: 1080, Duration: 2 * time.Second,
	}, &stdout, &stderr, func(capturelinux.ServiceConfig) (capturelinux.Source, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "frame 0") || !strings.Contains(out, "frame 1") {
		t.Errorf("missing per-frame lines in stdout: %s", out)
	}
	if !strings.Contains(out, "pts=100ms") {
		t.Errorf("missing PTS in stdout: %s", out)
	}
	if !strings.Contains(out, "source=pipewire") {
		t.Errorf("missing source= in stdout: %s", out)
	}
	if !strings.Contains(out, "payload=2") {
		t.Errorf("missing payload size in stdout: %s", out)
	}
	if !strings.Contains(stderr.String(), "frames=2") {
		t.Errorf("missing summary in stderr: %s", stderr.String())
	}
	if !fake.started || !fake.stopped {
		t.Errorf("Source not started/stopped: started=%v stopped=%v", fake.started, fake.stopped)
	}
}

func TestRun_ContextDeadlinePrintsSummary(t *testing.T) {
	ch := make(chan frames.Frame) // never delivers
	fake := &fakeSource{ch: ch, backend: capturelinux.BackendPortal}
	var stdout, stderr bytes.Buffer
	// Very short duration so we exercise the ctx.Done branch.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := run(ctx, demoConfig{
		Platform: "linux", Width: 1, Height: 1, Duration: 50 * time.Millisecond,
	}, &stdout, &stderr, func(capturelinux.ServiceConfig) (capturelinux.Source, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stderr.String(), "frames=0") {
		t.Errorf("missing zero-frames summary: %s", stderr.String())
	}
}

func TestRun_OpenError(t *testing.T) {
	boom := errors.New("no bus")
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), demoConfig{
		Platform: "linux", Width: 1, Height: 1, Duration: time.Second,
	}, &stdout, &stderr, func(capturelinux.ServiceConfig) (capturelinux.Source, error) {
		return nil, boom
	})
	if !errors.Is(err, boom) {
		t.Errorf("want boom wrapped, got %v", err)
	}
}

func TestRun_StartError(t *testing.T) {
	boom := errors.New("start fail")
	fake := &fakeSource{ch: make(chan frames.Frame), startErr: boom}
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), demoConfig{
		Platform: "linux", Width: 1, Height: 1, Duration: time.Second,
	}, &stdout, &stderr, func(capturelinux.ServiceConfig) (capturelinux.Source, error) {
		return fake, nil
	})
	if !errors.Is(err, boom) {
		t.Errorf("want boom wrapped, got %v", err)
	}
}

func TestRun_BadConfigRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), demoConfig{Platform: "mac"}, &stdout, &stderr,
		func(capturelinux.ServiceConfig) (capturelinux.Source, error) { return nil, nil })
	if err == nil {
		t.Error("bad config should error")
	}
}

func TestHealth_ContractString(t *testing.T) {
	if exitHealthOK != "ok\n" {
		t.Errorf("exitHealthOK = %q, want %q", exitHealthOK, "ok\n")
	}
}
