// Package gst provides GStreamer-based video frame extraction
package gst

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Frame represents an extracted video frame
type Frame struct {
	Data      []byte
	Width     int
	Height    int
	Format    PixelFormat
	Timestamp time.Time
	PTS       int64 // Presentation timestamp
	Duration  time.Duration
}

// PixelFormat represents supported pixel formats
type PixelFormat string

const (
	FormatRGB   PixelFormat = "RGB"
	FormatBGR   PixelFormat = "BGR"
	FormatRGBA  PixelFormat = "RGBA"
	FormatBGRA  PixelFormat = "BGRA"
	FormatGRAY8 PixelFormat = "GRAY8"
	FormatNV12  PixelFormat = "NV12"
	FormatI420  PixelFormat = "I420"
)

// SourceType represents the type of video source
type SourceType string

const (
	SourceRTSP   SourceType = "rtsp"
	SourceWebRTC SourceType = "webrtc"
	SourceFile   SourceType = "file"
	SourceDevice SourceType = "device"
	SourceTest   SourceType = "test"
)

// ExtractorConfig configures the frame extractor
type ExtractorConfig struct {
	SourceURL     string
	SourceType    SourceType
	Width         int
	Height        int
	FPS           int
	Format        PixelFormat
	BufferSize    int           // Frame buffer size
	Timeout       time.Duration // Connection timeout
	RetryAttempts int
}

// DefaultExtractorConfig returns default configuration
func DefaultExtractorConfig(sourceURL string) ExtractorConfig {
	return ExtractorConfig{
		SourceURL:     sourceURL,
		SourceType:    SourceRTSP,
		Width:         1920,
		Height:        1080,
		FPS:           30,
		Format:        FormatRGB,
		BufferSize:    30,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}
}

// FrameExtractor extracts frames from video streams using GStreamer
type FrameExtractor struct {
	config    ExtractorConfig
	cmd       *exec.Cmd
	frameChan chan *Frame
	errChan   chan error
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	statsMu   sync.RWMutex // guards stats fields
	running   bool
	stats     ExtractionStats
}

// ExtractionStats holds extraction statistics.
// It is a pure data struct with no embedded mutex; callers must hold
// the parent FrameExtractor.statsMu lock while reading/writing fields.
type ExtractionStats struct {
	FramesExtracted uint64
	FramesDropped   uint64
	BytesProcessed  uint64
	StartTime       time.Time
	Errors          uint64
}

// NewFrameExtractor creates a new frame extractor
func NewFrameExtractor(config ExtractorConfig) *FrameExtractor {
	ctx, cancel := context.WithCancel(context.Background())

	return &FrameExtractor{
		config:    config,
		frameChan: make(chan *Frame, config.BufferSize),
		errChan:   make(chan error, 10),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins frame extraction
func (fe *FrameExtractor) Start() error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if fe.running {
		return fmt.Errorf("extractor already running")
	}

	// Build GStreamer pipeline
	pipeline, err := fe.buildPipeline()
	if err != nil {
		return fmt.Errorf("failed to build pipeline: %w", err)
	}

	// Start GStreamer process
	cmd := exec.CommandContext(fe.ctx, "gst-launch-1.0", pipeline...)

	// Get stdout pipe for frame data
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GStreamer: %w", err)
	}

	fe.cmd = cmd
	fe.running = true
	fe.stats.StartTime = time.Now()

	// Start frame reader goroutine
	fe.wg.Add(1)
	go fe.readFrames(stdout)

	// Start monitor goroutine
	fe.wg.Add(1)
	go fe.monitor()

	return nil
}

// Stop stops frame extraction
func (fe *FrameExtractor) Stop() error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if !fe.running {
		return nil
	}

	fe.running = false
	fe.cancel()

	// Kill GStreamer process if still running
	if fe.cmd != nil && fe.cmd.Process != nil {
		fe.cmd.Process.Kill()
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		fe.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
	case <-time.After(5 * time.Second):
		// Timeout - force close
	}

	close(fe.frameChan)
	close(fe.errChan)

	return nil
}

// GetFrameChan returns the frame channel
func (fe *FrameExtractor) GetFrameChan() <-chan *Frame {
	return fe.frameChan
}

// GetErrorChan returns the error channel
func (fe *FrameExtractor) GetErrorChan() <-chan error {
	return fe.errChan
}

// IsRunning returns true if the extractor is running
func (fe *FrameExtractor) IsRunning() bool {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.running
}

// GetStats returns a snapshot of extraction statistics.
// The returned value is a plain copy with no mutex — safe to read
// without a lock after the function returns.
func (fe *FrameExtractor) GetStats() ExtractionStats {
	fe.statsMu.RLock()
	snap := ExtractionStats{
		FramesExtracted: fe.stats.FramesExtracted,
		FramesDropped:   fe.stats.FramesDropped,
		BytesProcessed:  fe.stats.BytesProcessed,
		StartTime:       fe.stats.StartTime,
		Errors:          fe.stats.Errors,
	}
	fe.statsMu.RUnlock()
	return snap
}

// buildPipeline builds the GStreamer pipeline arguments
func (fe *FrameExtractor) buildPipeline() ([]string, error) {
	var pipeline string

	switch fe.config.SourceType {
	case SourceRTSP:
		pipeline = fe.buildRTSPPipeline()
	case SourceWebRTC:
		pipeline = fe.buildWebRTCPipeline()
	case SourceFile:
		pipeline = fe.buildFilePipeline()
	case SourceDevice:
		pipeline = fe.buildDevicePipeline()
	case SourceTest:
		pipeline = fe.buildTestPipeline()
	default:
		return nil, fmt.Errorf("unsupported source type: %s", fe.config.SourceType)
	}

	// Split pipeline string into arguments
	// GStreamer pipeline syntax uses '!' as separators
	parts := strings.Split(pipeline, "!")
	var args []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			args = append(args, part)
			args = append(args, "!")
		}
	}

	// Remove trailing "!"
	if len(args) > 0 {
		args = args[:len(args)-1]
	}

	// Add fdsink to output to stdout
	args = append(args, "!", "fdsink")

	return args, nil
}

// buildRTSPPipeline builds pipeline for RTSP sources
func (fe *FrameExtractor) buildRTSPPipeline() string {
	return fmt.Sprintf(
		"rtspsrc location=%s latency=0 buffer-mode=auto ! "+
			"decodebin ! "+
			"videoconvert ! "+
			"videoscale ! "+
			"video/x-raw,format=%s,width=%d,height=%d,framerate=%d/1 ! "+
			"appsink name=sink max-buffers=%d drop=true",
		fe.config.SourceURL,
		fe.config.Format,
		fe.config.Width,
		fe.config.Height,
		fe.config.FPS,
		fe.config.BufferSize,
	)
}

// buildWebRTCPipeline builds pipeline for WebRTC sources
func (fe *FrameExtractor) buildWebRTCPipeline() string {
	// WebRTC typically uses webrtcbin, but for extraction we assume
	// the stream is already being received and we connect to it
	return fmt.Sprintf(
		"webrtcbin name=webrtc ! "+
			"queue ! "+
			"decodebin ! "+
			"videoconvert ! "+
			"video/x-raw,format=%s,width=%d,height=%d ! "+
			"appsink name=sink max-buffers=%d drop=true",
		fe.config.Format,
		fe.config.Width,
		fe.config.Height,
		fe.config.BufferSize,
	)
}

// buildFilePipeline builds pipeline for file sources
func (fe *FrameExtractor) buildFilePipeline() string {
	return fmt.Sprintf(
		"filesrc location=%s ! "+
			"decodebin ! "+
			"videoconvert ! "+
			"videoscale ! "+
			"video/x-raw,format=%s,width=%d,height=%d,framerate=%d/1 ! "+
			"appsink name=sink max-buffers=%d drop=true",
		fe.config.SourceURL,
		fe.config.Format,
		fe.config.Width,
		fe.config.Height,
		fe.config.FPS,
		fe.config.BufferSize,
	)
}

// buildDevicePipeline builds pipeline for device sources (v4l2, etc.)
func (fe *FrameExtractor) buildDevicePipeline() string {
	// Default to v4l2src for video devices
	return fmt.Sprintf(
		"v4l2src device=%s ! "+
			"video/x-raw,format=YUY2,width=%d,height=%d,framerate=%d/1 ! "+
			"videoconvert ! "+
			"video/x-raw,format=%s ! "+
			"appsink name=sink max-buffers=%d drop=true",
		fe.config.SourceURL,
		fe.config.Width,
		fe.config.Height,
		fe.config.FPS,
		fe.config.Format,
		fe.config.BufferSize,
	)
}

// buildTestPipeline builds pipeline for test sources
func (fe *FrameExtractor) buildTestPipeline() string {
	return fmt.Sprintf(
		"videotestsrc pattern=smpte is-live=true ! "+
			"video/x-raw,format=%s,width=%d,height=%d,framerate=%d/1 ! "+
			"appsink name=sink max-buffers=%d drop=true",
		fe.config.Format,
		fe.config.Width,
		fe.config.Height,
		fe.config.FPS,
		fe.config.BufferSize,
	)
}

// readFrames reads frames from GStreamer's stdout
func (fe *FrameExtractor) readFrames(stdout interface{}) {
	defer fe.wg.Done()

	// This is a simplified implementation
	// In production, you'd parse the raw bytes properly based on format
	// For now, we'll emit placeholder frames

	ticker := time.NewTicker(time.Second / time.Duration(fe.config.FPS))
	defer ticker.Stop()

	for {
		select {
		case <-fe.ctx.Done():
			return
		case <-ticker.C:
			if !fe.running {
				return
			}

			// Create a frame (placeholder - real implementation would read from stdout)
			frameSize := fe.config.Width * fe.config.Height * 3 // RGB = 3 bytes per pixel
			frame := &Frame{
				Data:      make([]byte, frameSize),
				Width:     fe.config.Width,
				Height:    fe.config.Height,
				Format:    fe.config.Format,
				Timestamp: time.Now(),
				PTS:       time.Since(fe.stats.StartTime).Milliseconds(),
				Duration:  time.Second / time.Duration(fe.config.FPS),
			}

			select {
			case fe.frameChan <- frame:
				fe.statsMu.Lock()
				fe.stats.FramesExtracted++
				fe.stats.BytesProcessed += uint64(frameSize)
				fe.statsMu.Unlock()
			default:
				// Buffer full, drop frame
				fe.statsMu.Lock()
				fe.stats.FramesDropped++
				fe.statsMu.Unlock()
			}
		}
	}
}

// monitor monitors the GStreamer process
func (fe *FrameExtractor) monitor() {
	defer fe.wg.Done()

	if fe.cmd == nil {
		return
	}

	err := fe.cmd.Wait()
	if err != nil && fe.running {
		fe.statsMu.Lock()
		fe.stats.Errors++
		fe.statsMu.Unlock()

		select {
		case fe.errChan <- fmt.Errorf("gstreamer process exited: %w", err):
		default:
		}

		// Attempt restart if configured
		if fe.config.RetryAttempts > 0 {
			fe.attemptRestart()
		}
	}
}

// attemptRestart attempts to restart the extractor
func (fe *FrameExtractor) attemptRestart() {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if !fe.running {
		return
	}

	time.Sleep(time.Second)

	// Build new pipeline
	pipeline, err := fe.buildPipeline()
	if err != nil {
		fe.errChan <- fmt.Errorf("restart failed: %w", err)
		return
	}

	// Start new process
	cmd := exec.CommandContext(fe.ctx, "gst-launch-1.0", pipeline...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fe.errChan <- fmt.Errorf("restart pipe failed: %w", err)
		return
	}

	if err := cmd.Start(); err != nil {
		fe.errChan <- fmt.Errorf("restart start failed: %w", err)
		return
	}

	fe.cmd = cmd

	// Restart frame reader
	fe.wg.Add(1)
	go fe.readFrames(stdout)
}

// ToImage converts a frame to an image.Image
func (f *Frame) ToImage() (image.Image, error) {
	switch f.Format {
	case FormatRGB:
		return &rgbImage{
			Pix:    f.Data,
			Stride: f.Width * 3,
			Rect:   image.Rect(0, 0, f.Width, f.Height),
		}, nil
	case FormatRGBA:
		return &image.RGBA{
			Pix:    f.Data,
			Stride: f.Width * 4,
			Rect:   image.Rect(0, 0, f.Width, f.Height),
		}, nil
	case FormatGRAY8:
		return &image.Gray{
			Pix:    f.Data,
			Stride: f.Width,
			Rect:   image.Rect(0, 0, f.Width, f.Height),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported format for image conversion: %s", f.Format)
	}
}

// rgbImage implements image.Image for RGB format
type rgbImage struct {
	Pix    []byte
	Stride int
	Rect   image.Rectangle
}

func (r *rgbImage) ColorModel() color.Model {
	return color.RGBAModel
}

func (r *rgbImage) Bounds() image.Rectangle {
	return r.Rect
}

func (r *rgbImage) At(x, y int) color.Color {
	if x < 0 || x >= r.Rect.Max.X || y < 0 || y >= r.Rect.Max.Y {
		return color.Black
	}
	offset := y*r.Stride + x*3
	return color.RGBA{
		R: r.Pix[offset],
		G: r.Pix[offset+1],
		B: r.Pix[offset+2],
		A: 255,
	}
}

// CheckGStreamer checks if GStreamer is installed
func CheckGStreamer() error {
	cmd := exec.Command("gst-launch-1.0", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gstreamer not found: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// GetGStreamerVersion returns the installed GStreamer version
func GetGStreamerVersion() (string, error) {
	cmd := exec.Command("gst-launch-1.0", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Parse version from output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "GStreamer") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}

	return "unknown", nil
}

// ListAvailableElements lists available GStreamer elements
func ListAvailableElements() ([]string, error) {
	cmd := exec.Command("gst-inspect-1.0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var elements []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "") {
			// Extract element name (first word)
			parts := strings.Fields(line)
			if len(parts) > 0 {
				elements = append(elements, parts[0])
			}
		}
	}

	return elements, nil
}

// CheckElement checks if a specific GStreamer element is available
func CheckElement(element string) bool {
	cmd := exec.Command("gst-inspect-1.0", element)
	err := cmd.Run()
	return err == nil
}

// ParseCaps parses GStreamer capabilities string
func ParseCaps(caps string) (map[string]string, error) {
	result := make(map[string]string)

	// Parse "video/x-raw,format=RGB,width=1920,height=1080"
	parts := strings.Split(caps, ",")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}

	return result, nil
}

// FormatSize calculates the buffer size for a given format
func FormatSize(width, height int, format PixelFormat) int {
	switch format {
	case FormatRGB, FormatBGR:
		return width * height * 3
	case FormatRGBA, FormatBGRA:
		return width * height * 4
	case FormatGRAY8:
		return width * height
	case FormatNV12:
		return width * height * 3 / 2 // Y plane + UV plane
	case FormatI420:
		return width * height * 3 / 2 // Y plane + U plane + V plane
	default:
		return width * height * 3 // Default to RGB
	}
}

// Resolution represents video resolution
type Resolution struct {
	Width  int
	Height int
}

// String returns resolution as string
func (r Resolution) String() string {
	return fmt.Sprintf("%dx%d", r.Width, r.Height)
}

// ParseResolution parses a resolution string like "1920x1080"
func ParseResolution(s string) (Resolution, error) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return Resolution{}, fmt.Errorf("invalid resolution format: %s", s)
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid width: %s", parts[0])
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid height: %s", parts[1])
	}

	return Resolution{Width: width, Height: height}, nil
}
