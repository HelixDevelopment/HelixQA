package gst

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPipelineBuilder(t *testing.T) {
	pb := NewPipelineBuilder()
	assert.NotNil(t, pb)
	assert.NotNil(t, pb.elements)
	assert.NotNil(t, pb.caps)
	assert.Empty(t, pb.elements)
	assert.Empty(t, pb.caps)
}

func TestPipelineBuilder_AddElement(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AddElement("videotestsrc")
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "videotestsrc", pb.elements[0])
}

func TestPipelineBuilder_AddElementWithProperties(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AddElement("videotestsrc", "pattern=smpte", "is-live=true")
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "videotestsrc pattern=smpte is-live=true", pb.elements[0])
}

func TestPipelineBuilder_AddCaps(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AddElement("videoconvert")
	pb.AddCaps("video/x-raw,format=RGB")
	
	assert.Len(t, pb.elements, 1)
	assert.Len(t, pb.caps, 1)
	assert.Equal(t, "video/x-raw,format=RGB", pb.caps[0])
}

func TestPipelineBuilder_AddVideoCaps(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AddElement("videoscale")
	pb.AddVideoCaps(FormatRGB, 1920, 1080, 30)
	
	assert.Len(t, pb.caps, 1)
	assert.Equal(t, "video/x-raw,format=RGB,width=1920,height=1080,framerate=30/1", pb.caps[0])
}

func TestPipelineBuilder_Build(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AddElement("videotestsrc")
	pb.AddElement("videoconvert")
	
	pipeline := pb.Build()
	
	assert.Equal(t, "videotestsrc ! videoconvert", pipeline)
}

func TestPipelineBuilder_BuildWithCaps(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AddElement("videoscale")
	pb.AddVideoCaps(FormatRGB, 1920, 1080, 30)
	pb.AddElement("appsink")
	
	pipeline := pb.Build()
	
	expected := "videoscale ! video/x-raw,format=RGB,width=1920,height=1080,framerate=30/1 ! appsink"
	assert.Equal(t, expected, pipeline)
}

func TestPipelineBuilder_BuildArgs(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AddElement("videotestsrc")
	pb.AddElement("videoconvert")
	
	args := pb.BuildArgs()
	
	expected := []string{"videotestsrc", "!", "videoconvert"}
	assert.Equal(t, expected, args)
}

func TestRTSPSource(t *testing.T) {
	pb := RTSPSource("rtsp://localhost:8554/test")
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "rtspsrc")
	assert.Contains(t, pipeline, "rtsp://localhost:8554/test")
	assert.Contains(t, pipeline, "latency=0")
	assert.Contains(t, pipeline, "buffer-mode=auto")
}

func TestFileSource(t *testing.T) {
	pb := FileSource("/tmp/test.mp4")
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "filesrc")
	assert.Contains(t, pipeline, "/tmp/test.mp4")
}

func TestDeviceSource(t *testing.T) {
	pb := DeviceSource("/dev/video0")
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "v4l2src")
	assert.Contains(t, pipeline, "/dev/video0")
}

func TestTestSource(t *testing.T) {
	pb := TestSource("smpte")
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "videotestsrc")
	assert.Contains(t, pipeline, "pattern=smpte")
	assert.Contains(t, pipeline, "is-live=true")
}

func TestTestSourceDefault(t *testing.T) {
	pb := TestSource("")
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "pattern=smpte")
}

func TestScreenSourceLinux(t *testing.T) {
	pb := ScreenSourceLinux(":0")
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "ximagesrc")
	assert.Contains(t, pipeline, "display-name=:0")
}

func TestScreenSourceLinuxDefault(t *testing.T) {
	pb := ScreenSourceLinux("")
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "display-name=:0")
}

func TestScreenSourcePipeWire(t *testing.T) {
	pb := ScreenSourcePipeWire()
	pipeline := pb.Build()
	
	assert.Contains(t, pipeline, "pipewiresrc")
}

func TestPipelineBuilder_Decoder(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.Decoder()
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "decodebin", pb.elements[0])
}

func TestPipelineBuilder_VideoConvert(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.VideoConvert()
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "videoconvert", pb.elements[0])
}

func TestPipelineBuilder_VideoScale(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.VideoScale()
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "videoscale", pb.elements[0])
}

func TestPipelineBuilder_VideoRate(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.VideoRate(30)
	
	assert.Len(t, pb.elements, 1)
	assert.Contains(t, pb.elements[0], "videorate")
	assert.Contains(t, pb.elements[0], "max-rate=30")
}

func TestPipelineBuilder_Queue(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.Queue("myqueue", 100, 0, 0)
	
	assert.Len(t, pb.elements, 1)
	assert.Contains(t, pb.elements[0], "queue")
	assert.Contains(t, pb.elements[0], "name=myqueue")
	assert.Contains(t, pb.elements[0], "max-size-buffers=100")
}

func TestPipelineBuilder_Tee(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.Tee("t")
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "tee name=t", pb.elements[0])
}

func TestPipelineBuilder_TeeNoName(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.Tee("")
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "tee", pb.elements[0])
}

func TestPipelineBuilder_AppSink(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.AppSink("sink", 30, true)
	
	assert.Len(t, pb.elements, 1)
	assert.Contains(t, pb.elements[0], "appsink")
	assert.Contains(t, pb.elements[0], "name=sink")
	assert.Contains(t, pb.elements[0], "max-buffers=30")
	assert.Contains(t, pb.elements[0], "drop=true")
}

func TestPipelineBuilder_FDSink(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.FDSink()
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "fdsink", pb.elements[0])
}

func TestPipelineBuilder_TCPServerSink(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.TCPServerSink("0.0.0.0", 8080)
	
	assert.Len(t, pb.elements, 1)
	assert.Contains(t, pb.elements[0], "tcpserversink")
	assert.Contains(t, pb.elements[0], "host=0.0.0.0")
	assert.Contains(t, pb.elements[0], "port=8080")
}

func TestPipelineBuilder_FileSink(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.FileSink("/tmp/output.mp4")
	
	assert.Len(t, pb.elements, 1)
	assert.Contains(t, pb.elements[0], "filesink")
	assert.Contains(t, pb.elements[0], "/tmp/output.mp4")
}

func TestPipelineBuilder_H264Encoder(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.H264Encoder("ultrafast", "zerolatency")
	
	assert.Len(t, pb.elements, 1)
	assert.Contains(t, pb.elements[0], "x264enc")
	assert.Contains(t, pb.elements[0], "speed-preset=ultrafast")
	assert.Contains(t, pb.elements[0], "tune=zerolatency")
}

func TestPipelineBuilder_H265Encoder(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.H265Encoder()
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "x265enc", pb.elements[0])
}

func TestPipelineBuilder_VP8Encoder(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.VP8Encoder()
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "vp8enc", pb.elements[0])
}

func TestPipelineBuilder_VP9Encoder(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.VP9Encoder()
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "vp9enc", pb.elements[0])
}

func TestPipelineBuilder_Mux(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.Mux("mp4mux")
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "mp4mux", pb.elements[0])
}

func TestPipelineBuilder_Parse(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.Parse("h264")
	
	assert.Len(t, pb.elements, 1)
	assert.Equal(t, "h264parse", pb.elements[0])
}

func TestPipelineBuilder_CapFilter(t *testing.T) {
	pb := NewPipelineBuilder()
	pb.CapFilter("video/x-raw,format=RGB")
	
	assert.Len(t, pb.elements, 1)
	assert.Contains(t, pb.elements[0], "capsfilter")
	assert.Contains(t, pb.elements[0], "caps=video/x-raw,format=RGB")
}

func TestFrameExtractionPipeline(t *testing.T) {
	pipeline := FrameExtractionPipeline(
		"rtsp://localhost:8554/test",
		SourceRTSP,
		FormatRGB,
		1920, 1080, 30,
	)
	
	assert.Contains(t, pipeline, "rtspsrc")
	assert.Contains(t, pipeline, "decodebin")
	assert.Contains(t, pipeline, "videoconvert")
	assert.Contains(t, pipeline, "videoscale")
	assert.Contains(t, pipeline, "1920")
	assert.Contains(t, pipeline, "1080")
	assert.Contains(t, pipeline, "30/1")
	assert.Contains(t, pipeline, "appsink")
}

func TestRecordingPipeline(t *testing.T) {
	pipeline := RecordingPipeline(
		"rtsp://localhost:8554/test",
		"/tmp/recording.mp4",
		60,
	)
	
	assert.Contains(t, pipeline, "rtspsrc")
	assert.Contains(t, pipeline, "x264enc")
	assert.Contains(t, pipeline, "mp4mux")
	assert.Contains(t, pipeline, "/tmp/recording.mp4")
}

func TestStreamingPipeline(t *testing.T) {
	pipeline := StreamingPipeline(
		"rtsp://localhost:8554/test",
		"rtmp://localhost/live/stream",
		"h264",
	)
	
	assert.Contains(t, pipeline, "rtspsrc")
	assert.Contains(t, pipeline, "x264enc")
	assert.Contains(t, pipeline, "tcpserversink")
}

func TestScreenCapturePipelineLinux(t *testing.T) {
	pipeline := ScreenCapturePipelineLinux(":0", "screen")
	
	assert.Contains(t, pipeline, "ximagesrc")
	assert.Contains(t, pipeline, "x264enc")
	assert.Contains(t, pipeline, "rtspclientsink")
}

func TestMultiOutputPipeline(t *testing.T) {
	pipeline := MultiOutputPipeline("rtsp://localhost:8554/test")
	
	assert.Contains(t, pipeline, "rtspsrc")
	assert.Contains(t, pipeline, "tee")
}

func TestProcessingPipeline(t *testing.T) {
	pipeline := ProcessingPipeline(
		"rtsp://localhost:8554/test",
		FormatRGB,
		1920, 1080,
	)
	
	assert.Contains(t, pipeline, "rtspsrc")
	assert.Contains(t, pipeline, "processsink")
}

func TestExtractElements(t *testing.T) {
	pipeline := "videotestsrc ! video/x-raw,format=RGB ! videoconvert ! appsink"
	elements := ExtractElements(pipeline)
	
	assert.Contains(t, elements, "videotestsrc")
	assert.Contains(t, elements, "videoconvert")
	assert.Contains(t, elements, "appsink")
	// Should not contain caps
	assert.NotContains(t, elements, "video/x-raw,format=RGB")
}

func TestExtractElementsEmpty(t *testing.T) {
	elements := ExtractElements("")
	assert.Empty(t, elements)
}

func TestEstimateBitrate(t *testing.T) {
	tests := []struct {
		width    int
		height   int
		fps      int
		quality  string
		maxValue int // We check it's <= this value
	}{
		{640, 480, 30, "low", 2000000},      // SD
		{1280, 720, 30, "medium", 4000000},  // HD
		{1920, 1080, 30, "high", 8000000},   // Full HD
		{2560, 1440, 60, "ultra", 12000000}, // 2K
		{3840, 2160, 60, "ultra", 25000000}, // 4K
	}
	
	for _, tt := range tests {
		bitrate := EstimateBitrate(tt.width, tt.height, tt.fps, tt.quality)
		assert.Greater(t, bitrate, 0)
		assert.LessOrEqual(t, bitrate, tt.maxValue, 
			"Bitrate %d for %dx%d@%d should be <= %d", 
			bitrate, tt.width, tt.height, tt.fps, tt.maxValue)
	}
}

func TestEstimateBitrateDifferentQualities(t *testing.T) {
	// Use a low resolution and fps to avoid hitting the max caps
	// This ensures the bpp difference is visible in the output
	width, height, fps := 640, 480, 10 // SD @ 10fps
	
	low := EstimateBitrate(width, height, fps, "low")
	medium := EstimateBitrate(width, height, fps, "medium")
	high := EstimateBitrate(width, height, fps, "high")
	ultra := EstimateBitrate(width, height, fps, "ultra")
	
	// All should be different
	assert.Less(t, low, medium)
	assert.Less(t, medium, high)
	assert.Less(t, high, ultra)
}

func TestValidatePipeline(t *testing.T) {
	// Test with common elements that should exist
	pipeline := "videotestsrc ! videoconvert ! appsink"
	err := ValidatePipeline(pipeline)
	
	// May pass or fail depending on GStreamer installation
	// Just ensure it doesn't panic
	if err != nil {
		t.Logf("Validation error (expected if GStreamer not installed): %v", err)
	}
}

func TestChainedOperations(t *testing.T) {
	// Test chaining multiple operations
	pb := NewPipelineBuilder()
	pipeline := pb.
		AddElement("videotestsrc").
		VideoConvert().
		VideoScale().
		AddVideoCaps(FormatRGB, 1920, 1080, 30).
		Queue("", 100, 0, 0).
		AppSink("sink", 30, true).
		Build()
	
	assert.Contains(t, pipeline, "videotestsrc")
	assert.Contains(t, pipeline, "videoconvert")
	assert.Contains(t, pipeline, "videoscale")
	assert.Contains(t, pipeline, "queue")
	assert.Contains(t, pipeline, "appsink")
	assert.True(t, strings.Count(pipeline, " ! ") >= 4)
}
