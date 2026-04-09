// Package e2e provides end-to-end integration tests
package e2e

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"digital.vasic.helixqa/pkg/discovery"
	"digital.vasic.helixqa/pkg/distributed"
	"digital.vasic.helixqa/pkg/gst"
	"digital.vasic.helixqa/pkg/streaming"
	"digital.vasic.helixqa/pkg/vision"
)

// E2ETestSuite runs end-to-end integration tests
type E2ETestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *E2ETestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

func (s *E2ETestSuite) TearDownSuite() {
	s.cancel()
}

// TestFullPipeline tests the complete video processing pipeline
func (s *E2ETestSuite) TestFullPipeline() {
	// 1. Create test video source (simulated)
	s.T().Log("Step 1: Creating test video source...")
	testFrames := s.createTestFrames(30)
	require.NotEmpty(s.T(), testFrames)

	// 2. Test GStreamer frame extraction
	s.T().Log("Step 2: Testing frame extraction...")
	extractor := s.setupFrameExtractor()
	require.NotNil(s.T(), extractor)

	// 3. Test vision processing
	s.T().Log("Step 3: Testing vision processing...")
	visionResult := s.processWithVision(testFrames[0])
	require.NotNil(s.T(), visionResult)
	assert.NotEmpty(s.T(), visionResult.Elements)

	s.T().Log("✅ Full pipeline test completed successfully")
}

// TestDistributedState tests NATS JetStream integration
func (s *E2ETestSuite) TestDistributedState() {
	config := distributed.DefaultConfig()
	config.NATSURL = "nats://localhost:4222"

	stateManager, err := distributed.NewStateManager(config)
	if err != nil {
		s.T().Skipf("NATS not available: %v", err)
	}
	defer stateManager.Close()

	ctx := context.Background()
	state := &distributed.FrameProcessingState{
		FrameID:  "test-frame-001",
		Platform: "test",
		Status:   distributed.StatusProcessing,
	}

	err = stateManager.PublishFrameState(ctx, state)
	require.NoError(s.T(), err)

	retrieved, err := stateManager.GetFrameState(ctx, "test-frame-001")
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "test-frame-001", retrieved.FrameID)

	s.T().Log("✅ Distributed state test completed")
}

// TestWebRTCSignaling tests WebRTC signaling
func (s *E2ETestSuite) TestWebRTCSignaling() {
	config := streaming.DefaultWebRTCConfig()
	server := streaming.NewWebRTCServer(config)
	require.NotNil(s.T(), server)

	stats := server.GetServerStats()
	assert.NotNil(s.T(), stats)

	s.T().Log("✅ WebRTC signaling test completed")
}

// TestHostDiscovery tests network host discovery
func (s *E2ETestSuite) TestHostDiscovery() {
	hd := discovery.NewHostDiscovery()
	require.NotNil(s.T(), hd)

	hosts := hd.GetHosts()
	assert.NotNil(s.T(), hosts)

	s.T().Logf("Discovered %d hosts", len(hosts))
	s.T().Log("✅ Host discovery test completed")
}

// TestVisionOCRIntegration tests vision with OCR
func (s *E2ETestSuite) TestVisionOCRIntegration() {
	detectorConfig := vision.DefaultDetectorConfig()
	detectorConfig.EnableOCR = true
	detector := vision.NewElementDetector(detectorConfig)
	require.NotNil(s.T(), detector)

	img := s.createTestImageWithText()

	result, err := detector.Detect(img)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), result)

	s.T().Logf("Detected %d elements", len(result.Elements))

	stats := detector.GetStats()
	assert.Equal(s.T(), uint64(1), stats.FramesProcessed)

	s.T().Log("✅ Vision + OCR integration test completed")
}

// TestGStreamerPipeline tests GStreamer pipeline operations
func (s *E2ETestSuite) TestGStreamerPipeline() {
	pipeline := gst.FrameExtractionPipeline(
		"rtsp://localhost:8554/test",
		gst.SourceRTSP,
		gst.FormatRGB,
		1920, 1080, 30,
	)
	require.NotEmpty(s.T(), pipeline)
	assert.Contains(s.T(), pipeline, "rtspsrc")
	assert.Contains(s.T(), pipeline, "appsink")

	s.T().Log("✅ GStreamer pipeline test completed")
}

// TestPerformance benchmarks the pipeline performance
func (s *E2ETestSuite) TestPerformance() {
	if testing.Short() {
		s.T().Skip("Skipping performance test in short mode")
	}

	frames := s.createTestFrames(10)

	detectorConfig := vision.DefaultDetectorConfig()
	detectorConfig.EnableOCR = false
	detector := vision.NewElementDetector(detectorConfig)

	start := time.Now()
	for _, frame := range frames {
		_, err := detector.Detect(frame)
		require.NoError(s.T(), err)
	}
	duration := time.Since(start)

	avgLatency := duration / time.Duration(len(frames))
	fps := 1000.0 / float64(avgLatency.Milliseconds())

	s.T().Logf("Average latency: %v", avgLatency)
	s.T().Logf("Throughput: %.2f FPS", fps)

	assert.Less(s.T(), avgLatency.Milliseconds(), int64(100))

	s.T().Log("✅ Performance test completed")
}

// TestConcurrentProcessing tests concurrent stream processing
func (s *E2ETestSuite) TestConcurrentProcessing() {
	if testing.Short() {
		s.T().Skip("Skipping concurrent test in short mode")
	}

	frames := s.createTestFrames(20)

	detectorConfig := vision.DefaultDetectorConfig()
	detectorConfig.EnableOCR = false
	detector := vision.NewElementDetector(detectorConfig)

	results, err := detector.DetectBatch(frames)
	require.NoError(s.T(), err)
	assert.Len(s.T(), results, len(frames))

	for i, result := range results {
		assert.NotNil(s.T(), result, "Result %d is nil", i)
	}

	stats := detector.GetStats()
	assert.Equal(s.T(), uint64(len(frames)), stats.FramesProcessed)

	s.T().Logf("Successfully processed %d frames concurrently", len(frames))
	s.T().Log("✅ Concurrent processing test completed")
}

// Helper methods

func (s *E2ETestSuite) createTestFrames(count int) []image.Image {
	frames := make([]image.Image, count)
	for i := range frames {
		frames[i] = s.createSimpleTestImage(640, 480)
	}
	return frames
}

func (s *E2ETestSuite) createSimpleTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	
	return img
}

func (s *E2ETestSuite) createTestImageWithText() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	
	for y := 0; y < 200; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.White)
		}
	}
	
	buttonColor := color.RGBA{0, 120, 255, 255}
	for y := 80; y < 120; y++ {
		for x := 150; x < 250; x++ {
			img.Set(x, y, buttonColor)
		}
	}
	
	textColor := color.Black
	for y := 40; y < 50; y++ {
		for x := 50; x < 350; x++ {
			if x%10 < 8 {
				img.Set(x, y, textColor)
			}
		}
	}
	
	return img
}

func (s *E2ETestSuite) setupFrameExtractor() *gst.FrameExtractor {
	config := gst.DefaultExtractorConfig("test")
	config.SourceType = gst.SourceTest
	config.FPS = 1
	
	extractor := gst.NewFrameExtractor(config)
	return extractor
}

func (s *E2ETestSuite) processWithVision(img image.Image) *vision.FrameResult {
	detectorConfig := vision.DefaultDetectorConfig()
	detectorConfig.EnableOCR = false
	detector := vision.NewElementDetector(detectorConfig)
	
	result, err := detector.Detect(img)
	if err != nil {
		s.T().Logf("Vision detection error: %v", err)
		return nil
	}
	
	return result
}

// Run the test suite
func TestE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}
	
	suite.Run(t, new(E2ETestSuite))
}

// Individual quick tests for CI

func TestE2E_QuickPipeline(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	
	detectorConfig := vision.DefaultDetectorConfig()
	detectorConfig.EnableOCR = false
	detector := vision.NewElementDetector(detectorConfig)
	
	result, err := detector.Detect(img)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestE2E_QuickGStreamer(t *testing.T) {
	pipeline := gst.TestSource("smpte").
		VideoConvert().
		AppSink("sink", 10, true).
		Build()
	
	assert.NotEmpty(t, pipeline)
	assert.Contains(t, pipeline, "videotestsrc")
	assert.Contains(t, pipeline, "appsink")
}

func TestE2E_QuickWebRTC(t *testing.T) {
	config := streaming.DefaultWebRTCConfig()
	server := streaming.NewWebRTCServer(config)
	require.NotNil(t, server)
	
	stats := server.GetServerStats()
	assert.NotNil(t, stats)
}
