// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package vaapi is the OCU P5.5 VAAPI hardware-accelerated H.264 encoder
// backend. It spawns an ffmpeg subprocess using the h264_vaapi codec to
// encode raw BGRA8 frames piped on stdin, writing the resulting fragmented
// MP4 stream to the io.Writer supplied at construction time.
//
// ffmpeg invocation:
//
//	ffmpeg -init_hw_device vaapi=intel:<device> -filter_hw_device intel \
//	  -f rawvideo -pix_fmt bgra -s WxH -r FR -i pipe:0 \
//	  -vf format=nv12,hwupload \
//	  -c:v h264_vaapi -qp 23 \
//	  -f mp4 -movflags frag_keyframe+empty_moov pipe:1
//
// Device node: /dev/dri/renderD128 by default; configurable via
// cfg.DeviceNode or the HELIXQA_VAAPI_DEVICE environment variable.
//
// Kill-switch: HELIXQA_RECORD_VAAPI_STUB=1 forces ErrNotWired regardless of
// whether ffmpeg or the device node is present, useful for tests that must not
// spawn a real process.
//
// Fallback: if no ffmpeg binary is found on PATH, or the configured device
// node does not exist, ErrNotWired is returned so the caller can degrade
// gracefully.
package vaapi

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
	encoder.Register("vaapi", func() encoder.Encoder {
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

// defaultDeviceNode is the VA-API render node used when no override is
// configured.
const defaultDeviceNode = "/dev/dri/renderD128"

// RecordConfig mirrors the fields needed from contracts.RecordConfig without
// importing it directly (the factory func signature must match encoder.Encoder).
type RecordConfig struct {
	Width      int
	Height     int
	FrameRate  int
	DeviceNode string // optional; falls back to HELIXQA_VAAPI_DEVICE then defaultDeviceNode
}

// liveEncoder is the real ffmpeg-backed encoder returned by NewProductionEncoder.
type liveEncoder struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	once   sync.Once
	closed bool
	mu     sync.Mutex
}

// NewProductionEncoder constructs and starts an ffmpeg h264_vaapi subprocess.
//
//   - cfg supplies width, height, frame rate, and an optional device node.
//   - out is the sink that receives the MP4 stream from ffmpeg stdout.
//
// Returns ErrNotWired when the kill-switch is active, ffmpeg is not on PATH,
// or the VA-API device node is absent.
func NewProductionEncoder(cfg RecordConfig, out io.Writer) (encoder.Encoder, error) {
	if stubActive() {
		return nil, encoder.ErrNotWired
	}
	ffmpegPath, err := resolveFFmpeg()
	if err != nil {
		return nil, encoder.ErrNotWired
	}
	device, err := resolveDevice(cfg.DeviceNode)
	if err != nil {
		return nil, encoder.ErrNotWired
	}

	args := BuildFFmpegArgs(cfg, ffmpegPath, device)
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
	cmd.Stdout = out

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("vaapi: stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("vaapi: start ffmpeg: %w", err)
	}
	return &liveEncoder{cmd: cmd, stdin: stdin}, nil
}

// Encode writes a single BGRA8 frame to ffmpeg stdin.
func (e *liveEncoder) Encode(frame contracts.Frame) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("vaapi: encoder already closed")
	}
	if frame.Data == nil {
		return nil
	}
	b, err := frame.Data.AsBytes()
	if err != nil {
		return fmt.Errorf("vaapi: frame.Data.AsBytes: %w", err)
	}
	if _, err := e.stdin.Write(b); err != nil {
		return fmt.Errorf("vaapi: write to ffmpeg stdin: %w", err)
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
			closeErr = fmt.Errorf("vaapi: close stdin: %w", err)
			return
		}
		if err := e.cmd.Wait(); err != nil {
			closeErr = fmt.Errorf("vaapi: ffmpeg wait: %w", err)
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
	return "", fmt.Errorf("vaapi: no ffmpeg binary found on PATH")
}

// resolveDevice returns the device node path to use. Priority:
//  1. cfg.DeviceNode (if non-empty)
//  2. HELIXQA_VAAPI_DEVICE env var (if non-empty)
//  3. defaultDeviceNode
//
// Returns ErrNotWired (wrapped) when the chosen node does not exist on disk.
func resolveDevice(cfgNode string) (string, error) {
	node := cfgNode
	if node == "" {
		node = os.Getenv("HELIXQA_VAAPI_DEVICE")
	}
	if node == "" {
		node = defaultDeviceNode
	}
	if _, err := os.Stat(node); err != nil {
		return "", fmt.Errorf("vaapi: device node %q not found: %w", node, err)
	}
	return node, nil
}

func stubActive() bool {
	return os.Getenv("HELIXQA_RECORD_VAAPI_STUB") == "1"
}

// BuildFFmpegArgs returns the full argv for the ffmpeg h264_vaapi subprocess.
// The first element is the ffmpeg executable path.
// Exported so tests can inspect the generated argument list without spawning
// a real process.
func BuildFFmpegArgs(cfg RecordConfig, ffmpegPath, device string) []string {
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
		"-init_hw_device", fmt.Sprintf("vaapi=intel:%s", device),
		"-filter_hw_device", "intel",
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"-s", fmt.Sprintf("%dx%d", w, h),
		"-r", fmt.Sprintf("%d", fr),
		"-i", "pipe:0",
		"-vf", "format=nv12,hwupload",
		"-c:v", "h264_vaapi",
		"-qp", "23",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov",
		"pipe:1",
	}
}
