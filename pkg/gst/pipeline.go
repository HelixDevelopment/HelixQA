// Package gst provides GStreamer pipeline building and management
package gst

import (
	"fmt"
	"strings"
)

// PipelineBuilder helps construct GStreamer pipelines
type PipelineBuilder struct {
	elements []string
	caps     []string
}

// NewPipelineBuilder creates a new pipeline builder
func NewPipelineBuilder() *PipelineBuilder {
	return &PipelineBuilder{
		elements: make([]string, 0),
		caps:     make([]string, 0),
	}
}

// AddElement adds an element to the pipeline
func (pb *PipelineBuilder) AddElement(name string, properties ...string) *PipelineBuilder {
	if len(properties) > 0 {
		props := strings.Join(properties, " ")
		pb.elements = append(pb.elements, fmt.Sprintf("%s %s", name, props))
	} else {
		pb.elements = append(pb.elements, name)
	}
	return pb
}

// AddCaps adds caps between elements
func (pb *PipelineBuilder) AddCaps(caps string) *PipelineBuilder {
	pb.caps = append(pb.caps, caps)
	return pb
}

// AddVideoCaps adds video/x-raw caps
func (pb *PipelineBuilder) AddVideoCaps(format PixelFormat, width, height, fps int) *PipelineBuilder {
	caps := fmt.Sprintf("video/x-raw,format=%s,width=%d,height=%d,framerate=%d/1",
		format, width, height, fps)
	pb.caps = append(pb.caps, caps)
	return pb
}

// Build constructs the final pipeline string
func (pb *PipelineBuilder) Build() string {
	var parts []string
	
	for i, elem := range pb.elements {
		parts = append(parts, elem)
		// Add caps after element if available
		if i < len(pb.caps) {
			parts = append(parts, pb.caps[i])
		}
	}
	
	return strings.Join(parts, " ! ")
}

// BuildArgs constructs the pipeline as command-line arguments
func (pb *PipelineBuilder) BuildArgs() []string {
	var args []string
	
	for i, elem := range pb.elements {
		// Split element into parts (name + properties)
		parts := strings.Fields(elem)
		args = append(args, parts...)
		args = append(args, "!")
		
		// Add caps if available
		if i < len(pb.caps) {
			args = append(args, pb.caps[i])
			args = append(args, "!")
		}
	}
	
	// Remove trailing "!"
	if len(args) > 0 {
		args = args[:len(args)-1]
	}
	
	return args
}

// Common pipeline presets

// RTSPSource creates an RTSP source pipeline
func RTSPSource(url string) *PipelineBuilder {
	return NewPipelineBuilder().
		AddElement("rtspsrc", fmt.Sprintf("location=%s", url), "latency=0", "buffer-mode=auto")
}

// FileSource creates a file source pipeline
func FileSource(path string) *PipelineBuilder {
	return NewPipelineBuilder().
		AddElement("filesrc", fmt.Sprintf("location=%s", path))
}

// DeviceSource creates a video device source pipeline
func DeviceSource(device string) *PipelineBuilder {
	return NewPipelineBuilder().
		AddElement("v4l2src", fmt.Sprintf("device=%s", device))
}

// TestSource creates a test source pipeline
func TestSource(pattern string) *PipelineBuilder {
	if pattern == "" {
		pattern = "smpte"
	}
	return NewPipelineBuilder().
		AddElement("videotestsrc", fmt.Sprintf("pattern=%s", pattern), "is-live=true")
}

// ScreenSourceLinux creates a Linux screen capture pipeline
func ScreenSourceLinux(display string) *PipelineBuilder {
	if display == "" {
		display = ":0"
	}
	return NewPipelineBuilder().
		AddElement("ximagesrc", fmt.Sprintf("display-name=%s", display), "use-damage=false", "startx=0", "starty=0")
}

// ScreenSourcePipeWire creates a PipeWire screen capture pipeline
func ScreenSourcePipeWire() *PipelineBuilder {
	return NewPipelineBuilder().
		AddElement("pipewiresrc")
}

// Decoder adds a decoder
func (pb *PipelineBuilder) Decoder() *PipelineBuilder {
	return pb.AddElement("decodebin")
}

// VideoConvert adds videoconvert
func (pb *PipelineBuilder) VideoConvert() *PipelineBuilder {
	return pb.AddElement("videoconvert")
}

// VideoScale adds videoscale
func (pb *PipelineBuilder) VideoScale() *PipelineBuilder {
	return pb.AddElement("videoscale")
}

// VideoRate adds videorate for frame rate control
func (pb *PipelineBuilder) VideoRate(fps int) *PipelineBuilder {
	return pb.AddElement("videorate", fmt.Sprintf("max-rate=%d", fps))
}

// Queue adds a queue element
func (pb *PipelineBuilder) Queue(name string, maxBuffers, maxBytes, maxTime int) *PipelineBuilder {
	props := []string{
		fmt.Sprintf("max-size-buffers=%d", maxBuffers),
		fmt.Sprintf("max-size-bytes=%d", maxBytes),
		fmt.Sprintf("max-size-time=%d", maxTime),
	}
	if name != "" {
		props = append([]string{fmt.Sprintf("name=%s", name)}, props...)
	}
	return pb.AddElement("queue", props...)
}

// Tee adds a tee element for splitting streams
func (pb *PipelineBuilder) Tee(name string) *PipelineBuilder {
	if name != "" {
		return pb.AddElement("tee", fmt.Sprintf("name=%s", name))
	}
	return pb.AddElement("tee")
}

// AppSink adds an appsink for frame extraction
func (pb *PipelineBuilder) AppSink(name string, maxBuffers int, drop bool) *PipelineBuilder {
	props := []string{
		fmt.Sprintf("name=%s", name),
		fmt.Sprintf("max-buffers=%d", maxBuffers),
	}
	if drop {
		props = append(props, "drop=true")
	} else {
		props = append(props, "drop=false")
	}
	return pb.AddElement("appsink", props...)
}

// FDSink adds an fdsink for stdout output
func (pb *PipelineBuilder) FDSink() *PipelineBuilder {
	return pb.AddElement("fdsink")
}

// RTMPSink adds an RTMP sink
func (pb *PipelineBuilder) RTMPSink(url string) *PipelineBuilder {
	return pb.AddElement("rtmpsink", fmt.Sprintf("location=%s", url))
}

// RTSPSink adds an RTSP sink
func (pb *PipelineBuilder) RTSPSink(host string, port int, path string) *PipelineBuilder {
	return pb.AddElement("rtspclientsink", 
		fmt.Sprintf("location=rtsp://%s:%d/%s", host, port, path))
}

// TCPServerSink adds a TCP server sink
func (pb *PipelineBuilder) TCPServerSink(host string, port int) *PipelineBuilder {
	return pb.AddElement("tcpserversink", 
		fmt.Sprintf("host=%s", host),
		fmt.Sprintf("port=%d", port))
}

// FileSink adds a file sink
func (pb *PipelineBuilder) FileSink(path string) *PipelineBuilder {
	return pb.AddElement("filesink", fmt.Sprintf("location=%s", path))
}

// H264Encoder adds H.264 encoder
func (pb *PipelineBuilder) H264Encoder(preset string, tune string) *PipelineBuilder {
	props := []string{"x264enc"}
	if preset != "" {
		props = append(props, fmt.Sprintf("speed-preset=%s", preset))
	}
	if tune != "" {
		props = append(props, fmt.Sprintf("tune=%s", tune))
	}
	return pb.AddElement(strings.Join(props, " "))
}

// H265Encoder adds H.265/HEVC encoder
func (pb *PipelineBuilder) H265Encoder() *PipelineBuilder {
	return pb.AddElement("x265enc")
}

// VP8Encoder adds VP8 encoder
func (pb *PipelineBuilder) VP8Encoder() *PipelineBuilder {
	return pb.AddElement("vp8enc")
}

// VP9Encoder adds VP9 encoder
func (pb *PipelineBuilder) VP9Encoder() *PipelineBuilder {
	return pb.AddElement("vp9enc")
}

// Mux adds a muxer (mp4mux, matroskamux, etc.)
func (pb *PipelineBuilder) Mux(muxer string) *PipelineBuilder {
	return pb.AddElement(muxer)
}

// Parse adds a parser
func (pb *PipelineBuilder) Parse(codec string) *PipelineBuilder {
	parserName := fmt.Sprintf("%sparse", codec)
	return pb.AddElement(parserName)
}

// CapFilter adds a capsfiter
func (pb *PipelineBuilder) CapFilter(caps string) *PipelineBuilder {
	return pb.AddElement("capsfilter", fmt.Sprintf("caps=%s", caps))
}

// Common pipeline templates

// FrameExtractionPipeline creates a complete frame extraction pipeline
func FrameExtractionPipeline(sourceURL string, sourceType SourceType, format PixelFormat, width, height, fps int) string {
	var pb *PipelineBuilder
	
	switch sourceType {
	case SourceRTSP:
		pb = RTSPSource(sourceURL)
	case SourceFile:
		pb = FileSource(sourceURL)
	case SourceDevice:
		pb = DeviceSource(sourceURL)
	case SourceTest:
		pb = TestSource("smpte")
	default:
		pb = TestSource("smpte")
	}
	
	return pb.
		Decoder().
		VideoConvert().
		VideoScale().
		AddVideoCaps(format, width, height, fps).
		AppSink("sink", 30, true).
		Build()
}

// RecordingPipeline creates a recording pipeline
func RecordingPipeline(sourceURL string, outputPath string, duration int) string {
	return RTSPSource(sourceURL).
		Decoder().
		VideoConvert().
		H264Encoder("ultrafast", "zerolatency").
		Parse("h264").
		Mux("mp4mux").
		FileSink(outputPath).
		Build()
}

// StreamingPipeline creates a streaming pipeline
func StreamingPipeline(sourceURL string, outputURL string, codec string) string {
	pb := RTSPSource(sourceURL).
		Decoder().
		VideoConvert().
		Queue("", 100, 0, 0)
	
	switch codec {
	case "h264":
		pb = pb.H264Encoder("veryfast", "")
	case "h265":
		pb = pb.H265Encoder()
	case "vp8":
		pb = pb.VP8Encoder()
	case "vp9":
		pb = pb.VP9Encoder()
	}
	
	return pb.
		Parse(codec).
		Mux("matroskamux").
		TCPServerSink("0.0.0.0", 8080).
		Build()
}

// ScreenCapturePipelineLinux creates a Linux screen capture pipeline
func ScreenCapturePipelineLinux(display string, outputURL string) string {
	return ScreenSourceLinux(display).
		VideoConvert().
		VideoScale().
		AddVideoCaps(FormatRGB, 1920, 1080, 30).
		Queue("", 100, 0, 0).
		H264Encoder("ultrafast", "zerolatency").
		Parse("h264").
		RTSPSink("localhost", 8554, outputURL).
		Build()
}

// MultiOutputPipeline creates a pipeline with multiple outputs (tee)
func MultiOutputPipeline(sourceURL string) string {
	return RTSPSource(sourceURL).
		Decoder().
		VideoConvert().
		Tee("t").
		Build()
}

// ProcessingPipeline creates a pipeline for frame processing with OpenCV
func ProcessingPipeline(sourceURL string, format PixelFormat, width, height int) string {
	return RTSPSource(sourceURL).
		Decoder().
		VideoConvert().
		VideoScale().
		AddVideoCaps(format, width, height, 30).
		AppSink("processsink", 10, true).
		Build()
}

// ValidatePipeline validates a GStreamer pipeline by checking element availability
func ValidatePipeline(pipeline string) error {
	// Extract element names from pipeline
	// This is a simplified check - full validation would require parsing
	elements := ExtractElements(pipeline)
	
	for _, elem := range elements {
		if !CheckElement(elem) {
			return fmt.Errorf("element not available: %s", elem)
		}
	}
	
	return nil
}

// ExtractElements extracts element names from a pipeline string
func ExtractElements(pipeline string) []string {
	var elements []string
	parts := strings.Split(pipeline, "!")
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasPrefix(part, "video/") || strings.HasPrefix(part, "audio/") {
			continue
		}
		
		// Get element name (first word)
		fields := strings.Fields(part)
		if len(fields) > 0 {
			elemName := fields[0]
			// Remove any leading/trailing whitespace
			elemName = strings.TrimSpace(elemName)
			if elemName != "" {
				elements = append(elements, elemName)
			}
		}
	}
	
	return elements
}

// EstimateBitrate estimates the bitrate for a given resolution and fps
func EstimateBitrate(width, height, fps int, quality string) int {
	// Base bitrate calculation (bits per pixel)
	bpp := 0.1 // Default bits per pixel
	
	switch quality {
	case "low":
		bpp = 0.05
	case "medium":
		bpp = 0.1
	case "high":
		bpp = 0.2
	case "ultra":
		bpp = 0.4
	}
	
	// Calculate: width * height * fps * bpp
	bitrate := width * height * fps * int(bpp*1000000) / 1000000
	
	// Apply resolution-specific adjustments
	pixels := width * height
	switch {
	case pixels <= 640*480: // SD
		bitrate = min(bitrate, 2000000)
	case pixels <= 1280*720: // HD
		bitrate = min(bitrate, 4000000)
	case pixels <= 1920*1080: // Full HD
		bitrate = min(bitrate, 8000000)
	case pixels <= 2560*1440: // 2K
		bitrate = min(bitrate, 12000000)
	case pixels <= 3840*2160: // 4K
		bitrate = min(bitrate, 25000000)
	}
	
	return bitrate
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
