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
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"digital.vasic.helixqa/pkg/capture/linux"
)

// exitHealthOK matches the sidecar health contract documented by
// pkg/bridge/sidecarutil.HealthProbe.
const exitHealthOK = "ok\n"

// runConfig is the validated subset of argv that drives run().
type runConfig struct {
	Display      string
	Width        int
	Height       int
	FPS          int
	FFmpegPath   string
	ExtraArgs    []string
	Ctx          context.Context
	Stdout       io.Writer
	Stderr       io.Writer
	Cmd          CommandFactory
	EmitEnvelope func(ptsMicros int64, body []byte) []byte
	Clock        func() time.Time
}

// CommandFactory returns an exec.Cmd-shaped child that run() controls.
// Tests inject a fake that produces a pre-loaded stdout stream without
// actually running ffmpeg.
type CommandFactory interface {
	Start(ctx context.Context, bin string, args []string) (ChildProcess, error)
}

// ChildProcess is the minimal shape the wrapper uses on a spawned child.
type ChildProcess interface {
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	Wait() error
	Signal(sig os.Signal) error
	Kill() error
}

// osExecFactory is the production CommandFactory.
type osExecFactory struct{}

func (osExecFactory) Start(ctx context.Context, bin string, args []string) (ChildProcess, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("helixqa-x11grab: StdoutPipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("helixqa-x11grab: StderrPipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("helixqa-x11grab: exec %s: %w", bin, err)
	}
	return &osExecChild{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

type osExecChild struct {
	cmd      *exec.Cmd
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	killOnce sync.Once
}

func (c *osExecChild) Stdout() io.ReadCloser { return c.stdout }
func (c *osExecChild) Stderr() io.ReadCloser { return c.stderr }
func (c *osExecChild) Wait() error           { return c.cmd.Wait() }
func (c *osExecChild) Signal(sig os.Signal) error {
	if c.cmd.Process == nil {
		return errors.New("helixqa-x11grab: no process to signal")
	}
	return c.cmd.Process.Signal(sig)
}
func (c *osExecChild) Kill() error {
	var err error
	c.killOnce.Do(func() {
		if c.cmd.Process != nil {
			err = c.cmd.Process.Kill()
		}
	})
	return err
}

// ffmpegArgs builds the argv passed to ffmpeg for an x11grab invocation.
// Exposed for testing — we assert the argument order byte-for-byte.
func ffmpegArgs(cfg runConfig) []string {
	args := []string{
		"-loglevel", "error",
		"-f", "x11grab",
		"-video_size", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
		"-framerate", fmt.Sprintf("%d", cfg.FPS),
	}
	args = append(args, cfg.ExtraArgs...)
	args = append(args,
		"-i", cfg.Display,
		"-c:v", "libx264",
		"-tune", "zerolatency",
		"-preset", "ultrafast",
		"-pix_fmt", "yuv420p",
		"-f", "h264",
		"pipe:1",
	)
	return args
}

// run performs the full capture loop: spawn ffmpeg, split NALs on its stdout,
// emit envelopes on cfg.Stdout. Returns nil on clean shutdown, non-nil on
// any error. Stdout is flushed best-effort on early exit.
func run(cfg runConfig) error {
	if cfg.Cmd == nil {
		cfg.Cmd = osExecFactory{}
	}
	if cfg.EmitEnvelope == nil {
		cfg.EmitEnvelope = linux.EncodeEnvelope
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	args := ffmpegArgs(cfg)
	child, err := cfg.Cmd.Start(cfg.Ctx, cfg.FFmpegPath, args)
	if err != nil {
		return err
	}
	defer func() {
		// Best-effort cleanup — if ffmpeg is still running, interrupt,
		// wait up to 5s, then kill.
		_ = child.Signal(syscall.SIGINT)
		done := make(chan struct{})
		go func() {
			_ = child.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = child.Kill()
			<-done
		}
	}()

	// Tee stderr so ffmpeg diagnostics land in our stderr (SidecarRunner
	// captures this into the session archive).
	if cfg.Stderr != nil && child.Stderr() != nil {
		go func() { _, _ = io.Copy(cfg.Stderr, child.Stderr()) }()
	}

	startedAt := cfg.Clock()
	err = SplitNALs(child.Stdout(), func(nal []byte) error {
		pts := int64(cfg.Clock().Sub(startedAt) / time.Microsecond)
		env := cfg.EmitEnvelope(pts, nal)
		_, werr := cfg.Stdout.Write(env)
		return werr
	})
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func validateConfig(cfg runConfig) error {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("helixqa-x11grab: bad dimensions %dx%d", cfg.Width, cfg.Height)
	}
	if cfg.FPS <= 0 {
		return fmt.Errorf("helixqa-x11grab: bad fps %d", cfg.FPS)
	}
	if strings.TrimSpace(cfg.Display) == "" {
		return errors.New("helixqa-x11grab: empty display")
	}
	if cfg.FFmpegPath == "" {
		return errors.New("helixqa-x11grab: empty ffmpeg path")
	}
	return nil
}

// parseArgv parses argv into a runConfig (with ctx/stdout/stderr still unset).
func parseArgv(argv []string, lookup func(string) (string, bool)) (runConfig, bool, error) {
	fs := flag.NewFlagSet("helixqa-x11grab", flag.ContinueOnError)
	var (
		display  = fs.String("display", "", "X11 DISPLAY (default $DISPLAY, then :0)")
		width    = fs.Int("width", 0, "capture width in pixels (required)")
		height   = fs.Int("height", 0, "capture height in pixels (required)")
		fps      = fs.Int("fps", 30, "capture framerate in Hz")
		ffmpeg   = fs.String("ffmpeg", "ffmpeg", "ffmpeg binary path")
		extra    = fs.String("extra", "", "additional ffmpeg argv (space-separated) placed before -i")
		health   = fs.Bool("health", false, "print 'ok' and exit 0")
	)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(argv); err != nil {
		return runConfig{}, false, err
	}
	if *health {
		return runConfig{}, true, nil
	}
	if *display == "" {
		if v, ok := lookup("DISPLAY"); ok && v != "" {
			*display = v
		} else {
			*display = ":0"
		}
	}
	cfg := runConfig{
		Display:    *display,
		Width:      *width,
		Height:     *height,
		FPS:        *fps,
		FFmpegPath: *ffmpeg,
	}
	if strings.TrimSpace(*extra) != "" {
		cfg.ExtraArgs = strings.Fields(*extra)
	}
	return cfg, false, nil
}

func main() {
	cfg, healthMode, err := parseArgv(os.Args[1:], os.LookupEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "helixqa-x11grab: %v\n", err)
		os.Exit(2)
	}
	if healthMode {
		_, _ = os.Stdout.WriteString(exitHealthOK)
		return
	}
	// Signal handling: SIGINT/SIGTERM -> cancel ctx -> run() tears down ffmpeg.
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()
	cfg.Ctx = ctx
	cfg.Stdout = os.Stdout
	cfg.Stderr = os.Stderr
	if err := run(cfg); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "helixqa-x11grab: %v\n", err)
		os.Exit(1)
	}
}
