// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	capturelinux "digital.vasic.helixqa/pkg/capture/linux"
)

// exitHealthOK matches the sidecar health contract documented by
// pkg/bridge/sidecarutil.HealthProbe.
const exitHealthOK = "ok\n"

// demoConfig is the parsed argv + env state that drives run().
type demoConfig struct {
	Platform string // only "linux" for now
	Backend  string // auto / portal / kmsgrab / x11grab
	Width    int
	Height   int
	FPS      int
	Display  string
	Duration time.Duration
}

// sourceOpener abstracts the call into pkg/capture/linux so run() is
// testable without needing real sidecars or a real bus.
type sourceOpener func(capturelinux.ServiceConfig) (capturelinux.Source, error)

// run performs the actual capture loop. Broken out so tests can drive
// it with fake Sources.
func run(ctx context.Context, cfg demoConfig, stdout, stderr io.Writer, open sourceOpener) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	backend := capturelinux.ParseBackend(cfg.Backend)
	src, err := open(capturelinux.ServiceConfig{
		Width:           cfg.Width,
		Height:          cfg.Height,
		BackendOverride: backend,
		Display:         cfg.Display,
		FPS:             cfg.FPS,
	})
	if err != nil {
		return fmt.Errorf("capture-demo: NewDefaultSource: %w", err)
	}
	if err := src.Start(ctx); err != nil {
		return fmt.Errorf("capture-demo: Start: %w", err)
	}
	defer func() { _ = src.Stop() }()
	fmt.Fprintf(stderr, "capture-demo: started backend=%s width=%d height=%d duration=%v\n",
		src.Backend(), cfg.Width, cfg.Height, cfg.Duration)

	frameCount := 0
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return writeSummary(stderr, frameCount, time.Since(start))
		case f, ok := <-src.Frames():
			if !ok {
				return writeSummary(stderr, frameCount, time.Since(start))
			}
			payloadBytes := len(f.Data)
			if f.HasFD() {
				payloadBytes = f.DataLen
			}
			fmt.Fprintf(stdout, "frame %d pts=%v source=%s %dx%d format=%s payload=%d bytes\n",
				frameCount, f.PTS, f.Source, f.Width, f.Height, f.Format, payloadBytes)
			// Release any memfd ownership the Frame carries.
			_ = f.Close()
			frameCount++
		}
	}
}

func writeSummary(w io.Writer, frames int, elapsed time.Duration) error {
	fps := float64(frames) / elapsed.Seconds()
	if elapsed <= 0 {
		fps = 0
	}
	fmt.Fprintf(w, "capture-demo: done frames=%d elapsed=%v fps=%.1f\n", frames, elapsed, fps)
	return nil
}

func validateConfig(cfg demoConfig) error {
	if cfg.Platform != "linux" {
		return fmt.Errorf("capture-demo: unsupported --platform %q (only 'linux' today)", cfg.Platform)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("capture-demo: --width and --height required (%dx%d)", cfg.Width, cfg.Height)
	}
	if cfg.Duration <= 0 {
		return fmt.Errorf("capture-demo: --duration must be > 0 (%v)", cfg.Duration)
	}
	return nil
}

// parseArgv parses argv into a demoConfig. Returns health=true when --health
// is the only meaningful flag set.
func parseArgv(argv []string) (demoConfig, bool, error) {
	fs := flag.NewFlagSet("helixqa-capture-demo", flag.ContinueOnError)
	var (
		platform = fs.String("platform", "linux", "capture platform (currently: linux)")
		backend  = fs.String("backend", "", "linux backend override: auto|portal|kmsgrab|x11grab (default auto)")
		width    = fs.Int("width", 0, "capture width in pixels (required)")
		height   = fs.Int("height", 0, "capture height in pixels (required)")
		fps      = fs.Int("fps", 0, "x11grab-only: framerate (default 30)")
		display  = fs.String("display", "", "x11grab-only: X11 DISPLAY (default $DISPLAY -> :0)")
		duration = fs.Duration("duration", 5*time.Second, "how long to capture for")
		health   = fs.Bool("health", false, "print 'ok' and exit 0")
	)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(argv); err != nil {
		return demoConfig{}, false, err
	}
	if *health {
		return demoConfig{}, true, nil
	}
	return demoConfig{
		Platform: strings.ToLower(*platform),
		Backend:  *backend,
		Width:    *width,
		Height:   *height,
		FPS:      *fps,
		Display:  *display,
		Duration: *duration,
	}, false, nil
}

func main() {
	cfg, healthMode, err := parseArgv(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "capture-demo: %v\n", err)
		os.Exit(2)
	}
	if healthMode {
		_, _ = os.Stdout.WriteString(exitHealthOK)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	err = run(ctx, cfg, os.Stdout, os.Stderr, func(sc capturelinux.ServiceConfig) (capturelinux.Source, error) {
		return capturelinux.NewDefaultSource(sc)
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "capture-demo: %v\n", err)
		os.Exit(1)
	}
}
