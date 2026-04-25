// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/capture/frames"
)

// EnvelopeHeaderSize is the fixed-size prefix on every stdout frame: 4 bytes
// body length + 8 bytes PTS in microseconds, big-endian.
const EnvelopeHeaderSize = 12

// NoPTS is the sentinel PTS value signalling "the sidecar did not attach a
// timestamp" — the Go host substitutes time.Since(startedAt) in that case.
const NoPTS int64 = -1

// MaxEnvelopeBody caps a single envelope's body length. Real 1080p H.264 IDR
// frames rarely exceed 1 MiB; 16 MiB is a generous ceiling that still bounds
// memory growth on a malformed stream.
const MaxEnvelopeBody uint32 = 16 * 1024 * 1024

// ErrEnvelopeTooLarge is returned by ReadEnvelope when the announced body
// length exceeds MaxEnvelopeBody.
var ErrEnvelopeTooLarge = errors.New("linux/capture: envelope body exceeds MaxEnvelopeBody")

// Envelope is one frame as emitted by a capture sidecar: PTS + payload.
type Envelope struct {
	PTSMicros int64
	Body      []byte
}

// EncodeEnvelope is the inverse of ReadEnvelope; primarily used by tests and
// by sidecar reference implementations that are written in Go.
func EncodeEnvelope(ptsMicros int64, body []byte) []byte {
	out := make([]byte, EnvelopeHeaderSize+len(body))
	binary.BigEndian.PutUint32(out[0:4], uint32(len(body)))
	binary.BigEndian.PutUint64(out[4:12], encodePTS(ptsMicros))
	copy(out[EnvelopeHeaderSize:], body)
	return out
}

func encodePTS(pts int64) uint64 {
	if pts == NoPTS {
		return ^uint64(0)
	}
	return uint64(pts)
}

// ReadEnvelope decodes one envelope from r.
// Returns io.EOF only when the stream is cleanly closed at a boundary.
func ReadEnvelope(r io.Reader) (Envelope, error) {
	var hdr [EnvelopeHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		// io.ReadFull returns io.EOF if *zero* bytes were read; we preserve
		// that semantic so callers can detect clean shutdown.
		if errors.Is(err, io.EOF) {
			return Envelope{}, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return Envelope{}, io.ErrUnexpectedEOF
		}
		return Envelope{}, fmt.Errorf("linux/capture: read envelope header: %w", err)
	}
	bodyLen := binary.BigEndian.Uint32(hdr[0:4])
	ptsRaw := binary.BigEndian.Uint64(hdr[4:12])
	if bodyLen > MaxEnvelopeBody {
		return Envelope{}, fmt.Errorf("%w: len=%d", ErrEnvelopeTooLarge, bodyLen)
	}
	env := Envelope{}
	if ptsRaw == ^uint64(0) {
		env.PTSMicros = NoPTS
	} else {
		env.PTSMicros = int64(ptsRaw)
	}
	if bodyLen > 0 {
		env.Body = make([]byte, bodyLen)
		if _, err := io.ReadFull(r, env.Body); err != nil {
			return Envelope{}, fmt.Errorf("linux/capture: read envelope body: %w", err)
		}
	}
	return env, nil
}

// Cmd is the minimal handle a SidecarRunner needs on a spawned child. The
// production ExecRunner wraps *exec.Cmd; tests inject fakes.
type Cmd interface {
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser
	// Wait blocks until the process exits.
	Wait() error
	// Kill terminates the process. Safe to call more than once.
	Kill() error
}

// Runner abstracts the process-spawning side of SidecarRunner so that tests
// can inject a fake that returns a pipe-backed Cmd.
type Runner interface {
	Start(ctx context.Context, bin string, args []string, extras []*os.File) (Cmd, error)
}

// ExecRunner is the production Runner — a thin os/exec wrapper.
type ExecRunner struct{}

// Start implements Runner.
func (ExecRunner) Start(ctx context.Context, bin string, args []string, extras []*os.File) (Cmd, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if len(extras) > 0 {
		cmd.ExtraFiles = extras
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("linux/capture: StdoutPipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("linux/capture: StderrPipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("linux/capture: exec %s: %w", bin, err)
	}
	return &execCmd{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

type execCmd struct {
	cmd      *exec.Cmd
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	killOnce sync.Once
}

func (e *execCmd) Stdout() io.ReadCloser { return e.stdout }
func (e *execCmd) Stderr() io.ReadCloser { return e.stderr }
func (e *execCmd) Wait() error           { return e.cmd.Wait() }
func (e *execCmd) Kill() error {
	var err error
	e.killOnce.Do(func() {
		if e.cmd.Process != nil {
			err = e.cmd.Process.Kill()
		}
	})
	return err
}

// SidecarConfig describes a capture sidecar invocation.
type SidecarConfig struct {
	// Binary is the sidecar executable name or absolute path.
	Binary string
	// Args are the arguments following Binary.
	Args []string
	// ExtraFiles are additional FDs passed to the child (used by portal.go
	// to hand the PipeWire socket to helixqa-capture-linux). Ownership
	// transfers on successful Start.
	ExtraFiles []*os.File
	// Source is attached to every emitted frames.Frame as Frame.Source.
	Source string
	// Width, Height are the encoded frame dimensions; passed through to
	// every frames.Frame.
	Width, Height int
	// Format is the on-wire pixel/codec format (typically H264AnnexB).
	Format frames.Format
	// ChannelBuffer sizes the internal frames.Frame channel; 32 by default.
	ChannelBuffer int
	// Runner is the process spawner. Nil means ExecRunner{}.
	Runner Runner
	// Clock returns "now" — tests freeze time by supplying a stub.
	Clock func() time.Time
}

// ErrInvalidConfig is returned for any malformed SidecarConfig.
var ErrInvalidConfig = errors.New("linux/capture: invalid SidecarConfig")

func (c *SidecarConfig) validate() error {
	if c.Binary == "" {
		return fmt.Errorf("%w: Binary empty", ErrInvalidConfig)
	}
	if c.Source == "" {
		return fmt.Errorf("%w: Source empty", ErrInvalidConfig)
	}
	if c.Width <= 0 || c.Height <= 0 {
		return fmt.Errorf("%w: bad dimensions %dx%d", ErrInvalidConfig, c.Width, c.Height)
	}
	if !c.Format.Valid() {
		return fmt.Errorf("%w: format=%v", ErrInvalidConfig, c.Format)
	}
	return nil
}

// SidecarRunner spawns a capture sidecar and publishes its envelope stream as
// frames.Frame values on a Go channel. Safe for concurrent Start/Stop calls:
// Start is single-shot, Stop is idempotent via sync.Once.
type SidecarRunner struct {
	cfg       SidecarConfig
	cmd       Cmd
	started   bool
	startedAt time.Time
	frameCh   chan frames.Frame
	stopCh    chan struct{}
	stopOnce  sync.Once
	waitErr   error
	waitDone  chan struct{}
}

// NewSidecarRunner validates cfg and returns a runner ready for Start.
func NewSidecarRunner(cfg SidecarConfig) (*SidecarRunner, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if cfg.ChannelBuffer <= 0 {
		cfg.ChannelBuffer = 32
	}
	if cfg.Runner == nil {
		cfg.Runner = ExecRunner{}
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &SidecarRunner{
		cfg:      cfg,
		frameCh:  make(chan frames.Frame, cfg.ChannelBuffer),
		stopCh:   make(chan struct{}),
		waitDone: make(chan struct{}),
	}, nil
}

// Start launches the sidecar. Returns after the process is spawned (the frame
// pump runs in a goroutine). Calling Start more than once returns an error.
func (r *SidecarRunner) Start(ctx context.Context) error {
	if r.started {
		return errors.New("linux/capture: SidecarRunner.Start already called")
	}
	cmd, err := r.cfg.Runner.Start(ctx, r.cfg.Binary, r.cfg.Args, r.cfg.ExtraFiles)
	if err != nil {
		return err
	}
	r.cmd = cmd
	r.startedAt = r.cfg.Clock()
	r.started = true
	go r.pump(ctx)
	go r.wait()
	return nil
}

// Frames returns the read-only frame channel. Closed when the sidecar exits
// or Stop() is called.
func (r *SidecarRunner) Frames() <-chan frames.Frame { return r.frameCh }

// StartedAt reports the timestamp Start captured. Zero time before Start.
func (r *SidecarRunner) StartedAt() time.Time { return r.startedAt }

// Stop terminates the sidecar and waits for the pump goroutine to exit.
// Idempotent; safe from any goroutine.
func (r *SidecarRunner) Stop() error {
	var killErr error
	r.stopOnce.Do(func() {
		close(r.stopCh)
		if r.cmd != nil {
			killErr = r.cmd.Kill()
		}
		// Wait for the frame pump to close the channel.
		<-r.waitDone
	})
	return killErr
}

// Wait blocks until the sidecar process exits and returns its wait error (or
// nil on clean exit). Callers that need to observe natural exit should call
// Wait; callers that want to terminate use Stop.
func (r *SidecarRunner) Wait() error {
	<-r.waitDone
	return r.waitErr
}

// pump reads envelopes from the sidecar's stdout and publishes Frames.
// Returns when stdout hits EOF, the envelope reader errors, ctx is cancelled,
// or stopCh fires.
func (r *SidecarRunner) pump(ctx context.Context) {
	defer close(r.frameCh)
	stdout := r.cmd.Stdout()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		default:
		}
		env, err := ReadEnvelope(stdout)
		if err != nil {
			// EOF / closed reader / malformed stream all terminate the pump.
			return
		}
		f, ferr := r.envelopeToFrame(env)
		if ferr != nil {
			// Skip malformed envelopes but keep pumping — one bad frame must
			// not terminate a whole session.
			continue
		}
		select {
		case r.frameCh <- f:
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		}
	}
}

// wait reaps the child process and publishes its exit error via waitErr.
func (r *SidecarRunner) wait() {
	defer close(r.waitDone)
	if r.cmd == nil {
		return
	}
	r.waitErr = r.cmd.Wait()
}

func (r *SidecarRunner) envelopeToFrame(env Envelope) (frames.Frame, error) {
	pts := time.Duration(env.PTSMicros) * time.Microsecond
	if env.PTSMicros == NoPTS {
		pts = r.cfg.Clock().Sub(r.startedAt)
	}
	return frames.New(pts, r.cfg.Width, r.cfg.Height, r.cfg.Format, r.cfg.Source, env.Body)
}
