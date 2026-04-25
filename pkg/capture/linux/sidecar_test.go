// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/capture/frames"
)

// --- Envelope encode / decode ---

func TestEncodeEnvelope_Layout(t *testing.T) {
	body := []byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x42}
	buf := EncodeEnvelope(123456, body)
	if got := binary.BigEndian.Uint32(buf[0:4]); int(got) != len(body) {
		t.Errorf("body length = %d, want %d", got, len(body))
	}
	if got := int64(binary.BigEndian.Uint64(buf[4:12])); got != 123456 {
		t.Errorf("pts = %d", got)
	}
	if !bytes.Equal(buf[EnvelopeHeaderSize:], body) {
		t.Errorf("body mismatch: %v", buf[EnvelopeHeaderSize:])
	}
}

func TestEncodeEnvelope_NoPTSSentinel(t *testing.T) {
	buf := EncodeEnvelope(NoPTS, []byte{1})
	got := binary.BigEndian.Uint64(buf[4:12])
	if got != ^uint64(0) {
		t.Errorf("NoPTS should serialize as ^0, got %x", got)
	}
}

func TestReadEnvelope_Roundtrip(t *testing.T) {
	in := Envelope{PTSMicros: 987654321, Body: []byte("hello")}
	buf := EncodeEnvelope(in.PTSMicros, in.Body)
	out, err := ReadEnvelope(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if out.PTSMicros != in.PTSMicros {
		t.Errorf("pts: got %d want %d", out.PTSMicros, in.PTSMicros)
	}
	if !bytes.Equal(out.Body, in.Body) {
		t.Errorf("body: got %v", out.Body)
	}
}

func TestReadEnvelope_NoPTS(t *testing.T) {
	buf := EncodeEnvelope(NoPTS, []byte{0xAA})
	out, err := ReadEnvelope(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if out.PTSMicros != NoPTS {
		t.Errorf("NoPTS should round-trip; got %d", out.PTSMicros)
	}
}

func TestReadEnvelope_EmptyBody(t *testing.T) {
	buf := EncodeEnvelope(0, nil)
	out, err := ReadEnvelope(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Body) != 0 {
		t.Errorf("empty body should decode to empty slice, got %v", out.Body)
	}
}

func TestReadEnvelope_CleanEOF(t *testing.T) {
	_, err := ReadEnvelope(bytes.NewReader(nil))
	if !errors.Is(err, io.EOF) {
		t.Errorf("empty reader should return io.EOF, got %v", err)
	}
}

func TestReadEnvelope_ShortHeader(t *testing.T) {
	_, err := ReadEnvelope(bytes.NewReader([]byte{0, 0, 0, 5, 0, 0, 0, 0})) // only 8 of 12 header bytes
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("short header should be ErrUnexpectedEOF, got %v", err)
	}
}

func TestReadEnvelope_ShortBody(t *testing.T) {
	var hdr [EnvelopeHeaderSize]byte
	binary.BigEndian.PutUint32(hdr[0:4], 10)
	// pts zero; body advertises 10 bytes, supply only 3.
	r := bytes.NewReader(append(hdr[:], []byte{1, 2, 3}...))
	_, err := ReadEnvelope(r)
	if err == nil {
		t.Fatal("want error on short body")
	}
}

func TestReadEnvelope_TooLarge(t *testing.T) {
	var hdr [EnvelopeHeaderSize]byte
	binary.BigEndian.PutUint32(hdr[0:4], MaxEnvelopeBody+1)
	_, err := ReadEnvelope(bytes.NewReader(hdr[:]))
	if !errors.Is(err, ErrEnvelopeTooLarge) {
		t.Errorf("want ErrEnvelopeTooLarge, got %v", err)
	}
}

// --- SidecarConfig validation ---

func TestSidecarConfig_Validate(t *testing.T) {
	valid := SidecarConfig{
		Binary: "helixqa-capture-linux",
		Source: "pipewire",
		Width:  1920, Height: 1080,
		Format: frames.FormatH264AnnexB,
	}
	if _, err := NewSidecarRunner(valid); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
	bad := []struct {
		name string
		cfg  SidecarConfig
		sub  string
	}{
		{"empty-binary", SidecarConfig{Source: "x", Width: 1, Height: 1, Format: frames.FormatNV12}, "Binary empty"},
		{"empty-source", SidecarConfig{Binary: "x", Width: 1, Height: 1, Format: frames.FormatNV12}, "Source empty"},
		{"zero-dims", SidecarConfig{Binary: "x", Source: "y", Format: frames.FormatNV12}, "bad dimensions"},
		{"bad-format", SidecarConfig{Binary: "x", Source: "y", Width: 1, Height: 1, Format: frames.FormatUnknown}, "format"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSidecarRunner(tc.cfg)
			if err == nil || !strings.Contains(err.Error(), tc.sub) {
				t.Errorf("got %v, want substring %q", err, tc.sub)
			}
		})
	}
}

// --- Fake Runner / Cmd ---

type fakeCmd struct {
	stdoutR, stderrR io.ReadCloser
	waitCh           chan struct{}
	waitErr          error
	killed           bool
	killOnce         sync.Once
}

func (c *fakeCmd) Stdout() io.ReadCloser { return c.stdoutR }
func (c *fakeCmd) Stderr() io.ReadCloser { return c.stderrR }
func (c *fakeCmd) Wait() error {
	<-c.waitCh
	return c.waitErr
}
func (c *fakeCmd) Kill() error {
	c.killOnce.Do(func() {
		c.killed = true
		close(c.waitCh)
	})
	return nil
}

type fakeRunner struct {
	stdoutFeed  io.Reader
	stderrFeed  io.Reader
	lastBin     string
	lastArgs    []string
	lastExtras  []*os.File
	startError  error
	cmd         *fakeCmd
}

func (f *fakeRunner) Start(_ context.Context, bin string, args []string, extras []*os.File) (Cmd, error) {
	if f.startError != nil {
		return nil, f.startError
	}
	f.lastBin = bin
	f.lastArgs = append([]string(nil), args...)
	f.lastExtras = extras
	so := io.NopCloser(f.stdoutFeed)
	se := io.NopCloser(f.stderrFeed)
	f.cmd = &fakeCmd{
		stdoutR: so,
		stderrR: se,
		waitCh:  make(chan struct{}),
	}
	return f.cmd, nil
}

// --- SidecarRunner tests ---

func TestSidecarRunner_FramePump(t *testing.T) {
	// Stdout: three envelopes emitted back-to-back.
	var buf bytes.Buffer
	buf.Write(EncodeEnvelope(1_000_000, []byte{0x67, 0x01}))
	buf.Write(EncodeEnvelope(2_000_000, []byte{0x42, 0x02}))
	buf.Write(EncodeEnvelope(NoPTS, []byte{0x00, 0x03}))
	// A zero-length envelope is legal (body=empty, PTS=0). Skipped because
	// frames.New rejects empty Data, so the pump discards it silently.
	buf.Write(EncodeEnvelope(0, nil))

	fr := &fakeRunner{stdoutFeed: &buf, stderrFeed: bytes.NewReader(nil)}
	fixed := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	cfg := SidecarConfig{
		Binary: "helixqa-capture-linux",
		Args:   []string{"--node", "42"},
		Source: "pipewire",
		Width:  1920, Height: 1080,
		Format: frames.FormatH264AnnexB,
		Runner: fr,
		Clock:  func() time.Time { return fixed },
	}
	r, err := NewSidecarRunner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop() })

	got := collect(t, r.Frames(), 3, 2*time.Second)
	if len(got) != 3 {
		t.Fatalf("got %d frames, want 3", len(got))
	}
	if got[0].PTS != time.Second {
		t.Errorf("frame 0 pts = %v, want 1s", got[0].PTS)
	}
	if got[1].PTS != 2*time.Second {
		t.Errorf("frame 1 pts = %v, want 2s", got[1].PTS)
	}
	if got[2].PTS != 0 {
		// NoPTS sidecar is replaced with Clock() - startedAt, which is 0 with a fixed clock.
		t.Errorf("frame 2 pts (NoPTS fallback) = %v, want 0", got[2].PTS)
	}
	if got[0].Source != "pipewire" || got[0].Width != 1920 || got[0].Height != 1080 {
		t.Errorf("frame 0 metadata wrong: %+v", got[0])
	}
	if fr.lastBin != "helixqa-capture-linux" || len(fr.lastArgs) != 2 {
		t.Errorf("runner saw wrong argv: bin=%q args=%v", fr.lastBin, fr.lastArgs)
	}
}

func TestSidecarRunner_StopIdempotent(t *testing.T) {
	fr := &fakeRunner{stdoutFeed: bytes.NewReader(nil), stderrFeed: bytes.NewReader(nil)}
	r, err := NewSidecarRunner(SidecarConfig{
		Binary: "x", Source: "y", Width: 1, Height: 1,
		Format: frames.FormatNV12, Runner: fr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Stop(); err != nil {
		t.Errorf("first stop: %v", err)
	}
	if err := r.Stop(); err != nil {
		t.Errorf("second stop: %v", err)
	}
	if !fr.cmd.killed {
		t.Error("Kill was not called")
	}
}

func TestSidecarRunner_DoubleStart(t *testing.T) {
	fr := &fakeRunner{stdoutFeed: bytes.NewReader(nil), stderrFeed: bytes.NewReader(nil)}
	r, err := NewSidecarRunner(SidecarConfig{
		Binary: "x", Source: "y", Width: 1, Height: 1,
		Format: frames.FormatNV12, Runner: fr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop() })
	if err := r.Start(context.Background()); err == nil {
		t.Error("double Start should error")
	}
}

func TestSidecarRunner_RunnerStartError(t *testing.T) {
	boom := errors.New("no such binary")
	fr := &fakeRunner{startError: boom}
	r, _ := NewSidecarRunner(SidecarConfig{
		Binary: "x", Source: "y", Width: 1, Height: 1,
		Format: frames.FormatNV12, Runner: fr,
	})
	if err := r.Start(context.Background()); !errors.Is(err, boom) {
		t.Errorf("want boom, got %v", err)
	}
}

func TestSidecarRunner_ContextCancel(t *testing.T) {
	// Stdout never closes — rely on ctx or Stop to terminate.
	pr, pw := io.Pipe()
	fr := &fakeRunner{stdoutFeed: pr, stderrFeed: bytes.NewReader(nil)}
	r, _ := NewSidecarRunner(SidecarConfig{
		Binary: "x", Source: "y", Width: 1, Height: 1,
		Format: frames.FormatNV12, Runner: fr,
	})
	ctx, cancel := context.WithCancel(context.Background())
	if err := r.Start(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()
	_ = pw.Close()
	select {
	case <-r.Frames():
	case <-time.After(2 * time.Second):
		t.Fatal("Frames() channel did not close after cancel")
	}
	_ = r.Stop()
}

// --- helpers ---

func collect(t *testing.T, ch <-chan frames.Frame, n int, timeout time.Duration) []frames.Frame {
	t.Helper()
	out := make([]frames.Frame, 0, n)
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case f, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, f)
		case <-deadline:
			return out
		}
	}
	return out
}
