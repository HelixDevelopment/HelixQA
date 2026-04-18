// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package x264 is the OCU P5.5 software H.264 encoder backend.
// It spawns an ffmpeg subprocess with libx264 to encode raw BGRA8 frames
// piped on stdin, writing the resulting MP4 stream to the io.Writer supplied
// at construction time.
//
// Kill-switch: HELIXQA_RECORD_X264_STUB=1 forces ErrNotWired regardless of
// whether ffmpeg is installed, useful for tests that must not spawn a real
// process.
//
// Fallback: if no ffmpeg binary is found on PATH, ErrNotWired is returned
// so the caller can degrade gracefully.
package x264

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/record/encoder"
)

func init() {
	encoder.Register("x264", func() encoder.Encoder {
		return newEncoder()
	})
}

// newEncoder is package-level injectable; tests replace it with a mock.
var newEncoder = func() encoder.Encoder {
	return &productionEncoder{}
}

// productionEncoder is the not-yet-started state. Encode() on this instance
// returns ErrNotWired (consistent with P5 stub behaviour). Callers who need a
// real encoding session use NewProductionEncoder.
type productionEncoder struct{}

// Encode implements encoder.Encoder. Returns ErrNotWired until
// NewProductionEncoder is used.
func (e *productionEncoder) Encode(_ contracts.Frame) error {
	return encoder.ErrNotWired
}

// Close implements encoder.Encoder.
func (e *productionEncoder) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// Real encoder — constructed by NewProductionEncoder
// ---------------------------------------------------------------------------

// RecordConfig mirrors the fields needed from contracts.RecordConfig without
// importing it directly (the factory func signature must match encoder.Encoder).
type RecordConfig struct {
	Width     int
	Height    int
	FrameRate int
}

// liveEncoder is the real ffmpeg-backed encoder returned by NewProductionEncoder.
type liveEncoder struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	once   sync.Once
	closed bool
	mu     sync.Mutex
}

// NewProductionEncoder constructs and starts an ffmpeg libx264 subprocess.
//
//   - cfg supplies width, height, and frame rate.
//   - out is the sink that receives the MP4 stream from ffmpeg stdout.
//
// Returns ErrNotWired when the kill-switch is active or ffmpeg is not on PATH.
func NewProductionEncoder(cfg RecordConfig, out io.Writer) (encoder.Encoder, error) {
	if stubActive() {
		return nil, encoder.ErrNotWired
	}
	ffmpegPath, err := resolveFFmpeg()
	if err != nil {
		return nil, encoder.ErrNotWired
	}

	args := BuildFFmpegArgs(cfg, ffmpegPath)
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
	cmd.Stdout = out

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("x264: stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("x264: start ffmpeg: %w", err)
	}
	return &liveEncoder{cmd: cmd, stdin: stdin}, nil
}

// Encode writes a single BGRA8 frame to ffmpeg stdin.
func (e *liveEncoder) Encode(frame contracts.Frame) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("x264: encoder already closed")
	}
	if frame.Data == nil {
		return nil
	}
	b, err := frame.Data.AsBytes()
	if err != nil {
		return fmt.Errorf("x264: frame.Data.AsBytes: %w", err)
	}
	if _, err := e.stdin.Write(b); err != nil {
		return fmt.Errorf("x264: write to ffmpeg stdin: %w", err)
	}
	return nil
}

// Close closes stdin (signals EOF to ffmpeg) and waits for the process to exit.
func (e *liveEncoder) Close() error {
	var closeErr error
	e.once.Do(func() {
		e.mu.Lock()
		e.closed = true
		e.mu.Unlock()

		if err := e.stdin.Close(); err != nil {
			closeErr = fmt.Errorf("x264: close stdin: %w", err)
			return
		}
		if err := e.cmd.Wait(); err != nil {
			closeErr = fmt.Errorf("x264: ffmpeg wait: %w", err)
		}
	})
	return closeErr
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// FFmpegCandidates is the ordered list of executable names tried via LookPath.
// Exported so tests can override it to simulate a missing ffmpeg.
var FFmpegCandidates = []string{"ffmpeg", "ffmpeg.exe"}

func resolveFFmpeg() (string, error) {
	for _, name := range FFmpegCandidates {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("x264: no ffmpeg binary found on PATH")
}

func stubActive() bool {
	return os.Getenv("HELIXQA_RECORD_X264_STUB") == "1"
}

// BuildFFmpegArgs returns the full argv for the ffmpeg libx264 subprocess.
// The first element is the ffmpeg executable path.
// Exported so tests can inspect the generated argument list without spawning
// a real process.
func BuildFFmpegArgs(cfg RecordConfig, ffmpegPath string) []string {
	w := cfg.Width
	h := cfg.Height
	if w <= 0 {
		w = 1920
	}
	if h <= 0 {
		h = 1080
	}
	fr := cfg.FrameRate
	if fr <= 0 {
		fr = 30
	}
	return []string{
		ffmpegPath,
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"-s", fmt.Sprintf("%dx%d", w, h),
		"-r", fmt.Sprintf("%d", fr),
		"-i", "pipe:0",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov",
		"pipe:1",
	}
}
