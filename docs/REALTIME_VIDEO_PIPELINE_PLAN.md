# Enterprise Real-Time Video Processing Pipeline
## Universal Cross-Platform UI Understanding System

**Version:** 1.0  
**Status:** Implementation Plan  
**Cost Target:** $0 (100% Open Source)  
**Platforms:** Android TV, Android Mobile, Desktop (Tauri), Web (React), API (Go)

---

## Executive Summary

This document outlines the complete implementation of an enterprise-grade, real-time video processing pipeline for HelixQA that:

- **Eliminates ALL Vision API costs** (OpenAI, Google, Anthropic) → $0
- **Works across ALL platforms** (Android, Desktop, Web, API)
- **Processes video in real-time** (<50ms latency)
- **Distributes across network hosts** (CPU/RAM/GPU sharing)
- **Uses only open-source components** (zero licensing fees)

### Core Technologies (All Open Source)

| Component | Technology | License | Cost |
|-----------|------------|---------|------|
| Video Capture | scrcpy (Android), native APIs (Desktop), WebRTC (Web) | Apache 2.0/MIT | $0 |
| Video Streaming | GStreamer + FFmpeg | LGPL/GPL | $0 |
| Frame Processing | OpenCV 4.x | Apache 2.0 | $0 |
| OCR | Tesseract 5.x + PaddleOCR | Apache 2.0 | $0 |
| Vision LLM | LLaVA via Ollama | MIT | $0 |
| Container Orchestration | Docker/Podman | Apache 2.0 | $0 |
| Stream Server | MediaMTX | MIT | $0 |
| WebRTC | Pion (Go) | MIT | $0 |

---

## Phase 0: Foundation & Architecture (Week 1)

### 0.1 Infrastructure Setup

#### 0.1.1 Network Host Discovery & Resource Inventory
```go
// pkg/discovery/host_discovery.go
package discovery

type HostCapabilities struct {
    IP            string
    CPUCount      int
    TotalRAM      uint64
    GPUAvailable  bool
    GPUModel      string
    GPUVRAM       uint64
    LatencyMs     float64
}

// Discover hosts on local network that can run containers
type HostDiscovery struct {
    mu     sync.RWMutex
    hosts  map[string]*HostCapabilities
}

// Methods:
// - ScanNetwork(subnet string) ([]*HostCapabilities, error)
// - TestLatency(hostIP string) (float64, error)
// - GetOptimalHost(requirements ResourceRequirements) (*HostCapabilities, error)
```

**Tasks:**
- [ ] 0.1.1.1 Implement network scanning (nmap integration)
- [ ] 0.1.1.2 Create SSH-based host capability detection
- [ ] 0.1.1.3 Build latency testing system
- [ ] 0.1.1.4 Create host registry service
- [ ] 0.1.1.5 Implement automatic host failover

#### 0.1.2 Container Runtime Setup
```bash
# All hosts run Podman (rootless containers)
# No Docker daemon required

# Setup script for each host
./scripts/setup-video-host.sh

# Installs:
# - podman + podman-compose
# - nvidia-container-toolkit (if GPU)
# - gstreamer1.0-* packages
# - opencv-python
# - ollama
```

**Tasks:**
- [ ] 0.1.2.1 Create host setup automation script
- [ ] 0.1.2.2 Build base container image (opencv-gstreamer)
- [ ] 0.1.2.3 Configure GPU passthrough for containers
- [ ] 0.1.2.4 Set up rootless container networking
- [ ] 0.1.2.5 Create container registry mirror

#### 0.1.3 Distributed State Management
```go
// pkg/distributed/state.go
package distributed

type FrameProcessingState struct {
    FrameID       string
    Timestamp     time.Time
    HostID        string
    Platform      string
    Status        ProcessingStatus
    Elements      []UIElement
    TextBlocks    []TextBlock
}

// Uses Redis for distributed state (already in stack)
// Or lightweight alternative: NATS JetStream
```

**Tasks:**
- [ ] 0.1.3.1 Set up NATS JetStream for message passing
- [ ] 0.1.3.2 Implement state synchronization protocol
- [ ] 0.1.3.3 Create leader election for coordinator
- [ ] 0.1.3.4 Build state persistence layer
- [ ] 0.1.3.5 Implement state cleanup/reaper

---

## Phase 1: Universal Video Capture (Week 2)

### 1.1 Android Capture (scrcpy Integration)

#### 1.1.1 Scrcpy Stream Extraction
```go
// pkg/capture/android_capture.go
package capture

type AndroidCapture struct {
    deviceID    string
    resolution  Resolution
    fps         int
    cmd         *exec.Cmd
    stdout      io.ReadCloser
    frameChan   chan *Frame
}

// NewAndroidCapture creates capture for Android device
// Uses scrcpy --record-format=raw for raw H.264 stream
func NewAndroidCapture(deviceID string, res Resolution, fps int) *AndroidCapture

// Start() error - begins streaming
// ReadFrame() (*Frame, error) - reads single frame
// Stop() error - cleanup
```

**Tasks:**
- [ ] 1.1.1.1 Implement scrcpy raw stream capture
- [ ] 1.1.1.2 Create H.264 NAL unit parser
- [ ] 1.1.1.3 Build frame decoder (ffmpeg-go bindings)
- [ ] 1.1.1.4 Implement adaptive bitrate based on network
- [ ] 1.1.1.5 Add touch/keyboard input injection

#### 1.1.2 Scrcpy to RTSP Bridge
```go
// pkg/streaming/scrcpy_rtsp_bridge.go
package streaming

// ScrcpyRTSPBridge converts scrcpy output to RTSP stream
// Allows multiple consumers without re-capture
type ScrcpyRTSPBridge struct {
    capture    *capture.AndroidCapture
    rtspServer *mediamtx.Server
    streamPath string
}

// Start() launches:
// 1. scrcpy capture
// 2. FFmpeg encoder: raw -> H.264
// 3. RTSP server endpoint
// 4. Frame distributor
```

**Tasks:**
- [ ] 1.1.2.1 Implement FFmpeg pipeline for encoding
- [ ] 1.1.2.2 Set up MediaMTX RTSP server
- [ ] 1.1.2.3 Create multi-consumer stream fanout
- [ ] 1.1.2.4 Add stream quality metrics
- [ ] 1.1.2.5 Build connection health monitoring

### 1.2 Desktop Capture (Tauri/Native)

#### 1.2.1 Native Desktop Capture
```rust
// Desktop capture via Tauri/native APIs
// src-tauri/src/capture.rs

#[tauri::command]
async fn start_desktop_capture(
    window: tauri::Window,
    source: CaptureSource,  // Window or Screen
    fps: u32,
) -> Result<String, String> {
    // Platform-specific implementation:
    // Linux: pipewire + gstreamer
    // Windows: DXGI Desktop Duplication
    // macOS: ScreenCaptureKit
    
    // Returns RTSP stream URL
}
```

**Tasks:**
- [ ] 1.2.1.1 Implement Linux PipeWire capture
- [ ] 1.2.1.2 Implement Windows DXGI capture
- [ ] 1.2.1.3 Implement macOS ScreenCaptureKit
- [ ] 1.2.1.4 Create GStreamer encoder pipeline
- [ ] 1.2.1.5 Build RTSP output module

#### 1.2.2 Desktop to WebRTC Bridge
```go
// pkg/streaming/desktop_webrtc.go
package streaming

// DesktopWebRTCBridge for browser-based capture
// Uses Pion WebRTC library (Go)
type DesktopWebRTCBridge struct {
    peerConnection *webrtc.PeerConnection
    videoTrack     *webrtc.TrackLocalStaticSample
    signaling      SignalingServer
}

// Handles browser getDisplayMedia() -> WebRTC -> Go processing
```

**Tasks:**
- [ ] 1.2.2.1 Implement Pion WebRTC peer connection
- [ ] 1.2.2.2 Create signaling server (WebSocket)
- [ ] 1.2.2.3 Build SDP offer/answer handling
- [ ] 1.2.2.4 Implement ICE candidate exchange
- [ ] 1.2.2.5 Add screen sharing from browser

### 1.3 Web Capture (Browser APIs)

#### 1.3.1 Browser Media Capture
```typescript
// src/capture/browserCapture.ts

export class BrowserCapture {
    private stream: MediaStream | null = null;
    private videoTrack: MediaStreamTrack | null = null;
    private webRTC: WebRTCConnection;
    
    async startCapture(source: 'screen' | 'window' | 'tab'): Promise<void> {
        // Use getDisplayMedia for screen capture
        this.stream = await navigator.mediaDevices.getDisplayMedia({
            video: {
                width: { ideal: 1920 },
                height: { ideal: 1080 },
                frameRate: { ideal: 30 }
            }
        });
        
        // Send via WebRTC to processing server
        await this.webRTC.publish(this.stream);
    }
    
    // Extract frames for OpenCV processing
    async extractFrame(): Promise<ImageData> {
        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        // ... draw video frame
        return ctx.getImageData(0, 0, width, height);
    }
}
```

**Tasks:**
- [ ] 1.3.1.1 Implement getDisplayMedia wrapper
- [ ] 1.3.1.2 Create WebRTC publisher client
- [ ] 1.3.1.3 Build frame extraction to ArrayBuffer
- [ ] 1.3.1.4 Implement binary protobuf frame encoding
- [ ] 1.3.1.5 Add capture source selector UI

### 1.4 API/Backend Capture (N/A)

For API/backend testing, video capture is not applicable. Instead:
- HTTP request/response logging
- Database state snapshots
- Log file analysis

---

## Phase 2: Real-Time Video Streaming Infrastructure (Week 3)

### 2.1 Media Server (MediaMTX)

#### 2.1.1 RTSP Server Setup
```yaml
# mediamtx.yml
paths:
  # Android device streams
  android_tv:
    source: publisher
    runOnPublish: ffmpeg -i rtsp://localhost:$RTSP_PORT/$RTSP_PATH -c copy -f rtsp rtsp://backup:8554/backup
    
  # Desktop streams  
  desktop_app:
    source: publisher
    
  # Web streams
  web_app:
    source: publisher
    
  # Processed output (UI annotations)
  processed:
    source: publisher
    
  # Read paths for consumers
  all:
    source: publisher
```

**Tasks:**
- [ ] 2.1.1.1 Create MediaMTX configuration
- [ ] 2.1.1.2 Set up stream authentication
- [ ] 2.1.1.3 Configure stream recording
- [ ] 2.1.1.4 Implement stream health checks
- [ ] 2.1.1.5 Add stream metrics collection

### 2.2 GStreamer Processing Pipeline

#### 2.2.1 Frame Extraction Pipeline
```bash
# GStreamer pipeline for frame extraction from RTSP
# Runs in container on distributed hosts

gst-launch-1.0 rtspsrc location=rtsp://mediamtx:8554/android_tv \
    ! decodebin \
    ! videoconvert \
    ! videoscale \
    ! video/x-raw,format=RGB,width=1920,height=1080 \
    ! appsink name=sink

# Go bindings via go-gst
```

```go
// pkg/gst/frame_extractor.go
package gst

type FrameExtractor struct {
    pipeline *gst.Pipeline
    appsink  *gst.AppSink
    
    // Output channels
    RGBFrames   chan *Frame  // For OpenCV
    JPEGFrames  chan []byte  // For LLM processing
    Thumbnails  chan []byte  // For UI display
}

// Start extracting frames at specified FPS
func (fe *FrameExtractor) Start(targetFPS int) error

// GetFrame() returns next frame
func (fe *FrameExtractor) GetFrame() (*Frame, error)
```

**Tasks:**
- [ ] 2.2.1.1 Implement GStreamer Go bindings
- [ ] 2.2.1.2 Create frame extraction pipeline
- [ ] 2.2.1.3 Add frame rate control
- [ ] 2.2.1.4 Implement format conversion (YUV->RGB)
- [ ] 2.2.1.5 Build multi-resolution output

### 2.3 WebRTC Signaling Server

#### 2.3.1 Pion WebRTC Implementation
```go
// pkg/webrtc/signaling.go
package webrtc

// SignalingServer coordinates WebRTC connections
type SignalingServer struct {
    upgrader websocket.Upgrader
    peers    map[string]*PeerConnection
    rooms    map[string]*Room
}

type SignalingMessage struct {
    Type      string          `json:"type"`      // offer, answer, candidate
    TargetID  string          `json:"targetId"`
    SDP       string          `json:"sdp,omitempty"`
    Candidate *ICECandidate   `json:"candidate,omitempty"`
}

// Endpoints:
// POST /webrtc/join/:room_id
// WS /webrtc/signal/:peer_id
```

**Tasks:**
- [ ] 2.3.1.1 Implement WebSocket signaling
- [ ] 2.3.1.2 Create room management
- [ ] 2.3.1.3 Add STUN/TURN server integration
- [ ] 2.3.1.4 Implement peer discovery
- [ ] 2.3.1.5 Build connection quality monitoring

---

## Phase 3: OpenCV Frame Processing (Week 4)

### 3.1 Core OpenCV Integration

#### 3.1.1 Go-OpenCV Bridge
```go
// pkg/opencv/bridge.go
package opencv

// #cgo pkg-config: opencv4
// #include <opencv2/opencv.hpp>
import "C"

type Mat struct {
    p C.Mat
}

// Core operations needed:
// - Resize, Crop, Rotate
// - Color space conversion
// - Threshold, Blur, Edge detection
// - Contour detection
// - Template matching
// - Feature detection (ORB, SIFT)
```

**Tasks:**
- [ ] 3.1.1.1 Set up CGO bindings for OpenCV
- [ ] 3.1.1.2 Implement basic image operations
- [ ] 3.1.1.3 Create memory management (Mat lifecycle)
- [ ] 3.1.1.4 Add GPU acceleration (CUDA) support
- [ ] 3.1.1.5 Build parallel processing pipelines

#### 3.1.2 UI Element Detection
```go
// pkg/opencv/element_detector.go
package opencv

type UIElement struct {
    ID        string
    Type      ElementType  // Button, TextField, Image, etc.
    Bounds    Rect
    Confidence float64
    Features  []FeaturePoint
}

// ElementDetector uses multiple CV techniques
type ElementDetector struct {
    orbDetector     *ORBDetector
    contourAnalyzer *ContourAnalyzer
    textDetector    *TextDetector
    templateMatcher *TemplateMatcher
}

// Detect finds all UI elements in frame
func (ed *ElementDetector) Detect(frame *Mat) ([]UIElement, error) {
    // 1. Contour detection for shape-based elements
    // 2. ORB features for texture-based elements
    // 3. Template matching for known elements
    // 4. Merge and deduplicate results
}
```

**Tasks:**
- [ ] 3.1.2.1 Implement contour-based element detection
- [ ] 3.1.2.2 Create ORB feature detector
- [ ] 3.1.2.3 Build template matching system
- [ ] 3.1.2.4 Implement element classification
- [ ] 3.1.2.5 Add temporal tracking across frames

### 3.2 OCR Integration (Tesseract + PaddleOCR)

#### 3.2.1 Tesseract Integration
```go
// pkg/ocr/tesseract.go
package ocr

// #cgo LDFLAGS: -lleptonica -ltesseract
// #include <tesseract/capi.h>
import "C"

type TesseractEngine struct {
    api C.TessBaseAPI
}

// NewTesseractEngine creates OCR engine with specified language
func NewTesseractEngine(lang string) (*TesseractEngine, error)

// Recognize extracts text from image
func (te *TesseractEngine) Recognize(img *opencv.Mat) (*OCRResult, error)

type OCRResult struct {
    Text      string
    Words     []Word
    Confidence float64
}

type Word struct {
    Text       string
    Bounds     Rect
    Confidence float64
}
```

**Tasks:**
- [ ] 3.2.1.1 Set up Tesseract C++ bindings
- [ ] 3.2.1.2 Implement text recognition
- [ ] 3.2.1.3 Add bounding box extraction
- [ ] 3.2.1.4 Create confidence scoring
- [ ] 3.2.1.5 Support multiple languages

#### 3.2.2 PaddleOCR Integration
```python
# pkg/ocr/paddleocr_server.py
# Runs as Python service in container

from paddleocr import PaddleOCR
import grpc

class PaddleOCRServicer(ocr_pb2_grpc.OCRServicer):
    def __init__(self):
        self.ocr = PaddleOCR(
            use_angle_cls=True,
            lang='en',
            use_gpu=True  # If available
        )
    
    def Recognize(self, request, context):
        # Process image bytes
        result = self.ocr.ocr(request.image_data, cls=True)
        return OCRResult(
            text_blocks=[
                TextBlock(
                    text=line[1][0],
                    confidence=line[1][1],
                    bbox=line[0]
                )
                for line in result[0]
            ]
        )
```

**Tasks:**
- [ ] 3.2.2.1 Create PaddleOCR gRPC service
- [ ] 3.2.2.2 Implement text recognition endpoint
- [ ] 3.2.2.3 Add table/structure detection
- [ ] 3.2.2.4 Build GPU acceleration support
- [ ] 3.2.2.5 Integrate with Go client

### 3.3 Feature Matching & Tracking

#### 3.3.1 ORB Feature System
```go
// pkg/opencv/feature_tracker.go
package opencv

type FeatureTracker struct {
    detector   *ORB
    matcher    *BFMatcher
    
    // Tracked elements
    elements   map[string]*TrackedElement
    
    // Frame counter for aging
    frameCount uint64
}

type TrackedElement struct {
    ID           string
    Keypoints    []KeyPoint
    Descriptors  Mat
    LastSeen     uint64
    Bounds       Rect
}

// Track updates element positions across frames
func (ft *FeatureTracker) Track(current *Mat) ([]TrackedElement, error) {
    // 1. Detect features in current frame
    // 2. Match with stored element features
    // 3. Update positions using homography
    // 4. Add new elements, age old ones
}
```

**Tasks:**
- [ ] 3.3.1.1 Implement ORB feature detection
- [ ] 3.3.1.2 Create feature matching algorithm
- [ ] 3.3.1.3 Build element tracking system
- [ ] 3.3.1.4 Add homography-based position update
- [ ] 3.3.1.5 Implement element lifecycle management

---

## Phase 4: Local Vision LLM Integration (Week 5)

### 4.1 Ollama Setup & Management

#### 4.1.1 Ollama Container Deployment
```yaml
# docker-compose.ollama.yml
services:
  ollama:
    image: ollama/ollama:latest
    container_name: ollama
    volumes:
      - ollama-models:/root/.ollama
    environment:
      - OLLAMA_KEEP_ALIVE=24h
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
    # Distributed: run on GPU-capable hosts
    
  # LLaVA model specifically for UI understanding
  # ollama pull llava:13b-v1.6
  # ollama pull bakllava:latest
  # ollama pull qwen2-vl:latest
```

**Tasks:**
- [ ] 4.1.1.1 Create Ollama service configuration
- [ ] 4.1.1.2 Implement model auto-download
- [ ] 4.1.1.3 Build model management API
- [ ] 4.1.1.4 Add GPU scheduling across hosts
- [ ] 4.1.1.5 Create model warming system

#### 4.1.2 Go Ollama Client
```go
// pkg/llm/ollama_client.go
package llm

type OllamaClient struct {
    baseURL string
    model   string
    client  *http.Client
}

type VisionPrompt struct {
    Image      []byte   // JPEG/PNG
    Prompt     string
    Context    []Message
}

type VisionResponse struct {
    Text       string
    Done       bool
    TokensUsed int
}

// AnalyzeUI sends frame to LLaVA for understanding
func (oc *OllamaClient) AnalyzeUI(prompt VisionPrompt) (*VisionResponse, error) {
    req := OllamaRequest{
        Model:  oc.model,
        Prompt: prompt.Prompt,
        Images: []string{base64Encode(prompt.Image)},
        Stream: false,
    }
    // POST /api/generate
}
```

**Tasks:**
- [ ] 4.1.2.1 Implement Ollama HTTP client
- [ ] 4.1.2.2 Create image encoding utilities
- [ ] 4.1.2.3 Add prompt templating system
- [ ] 4.1.2.4 Implement response parsing
- [ ] 4.1.2.5 Build retry/circuit breaker logic

### 4.2 UI Understanding Pipeline

#### 4.2.1 Structured UI Parsing
```go
// pkg/vision/ui_parser.go
package vision

// UIParser combines CV + OCR + LLM for complete understanding
type UIParser struct {
    elementDetector *opencv.ElementDetector
    ocrEngine       ocr.Engine
    llmClient       llm.VisionClient
    
    // Caching
    cache           *UICache
}

type ParsedUI struct {
    Timestamp    time.Time
    Elements     []UIElement
    TextContent  []TextBlock
    Layout       LayoutInfo
    LLMAnalysis  string
    Actions      []PossibleAction
}

// Parse performs full UI analysis
func (up *UIParser) Parse(frame *Frame) (*ParsedUI, error) {
    // 1. Fast CV detection (always run)
    elements, _ := up.elementDetector.Detect(frame.Mat)
    
    // 2. OCR for all text
    textBlocks, _ := up.ocrEngine.Recognize(frame.Mat)
    
    // 3. LLM for semantic understanding (rate limited)
    llmResult, _ := up.llmClient.AnalyzeUI(llm.VisionPrompt{
        Image:  frame.JPEG,
        Prompt: up.buildPrompt(elements, textBlocks),
    })
    
    // 4. Merge and structure results
    return up.mergeResults(elements, textBlocks, llmResult)
}
```

**Tasks:**
- [ ] 4.2.1.1 Implement CV-based element detection
- [ ] 4.2.1.2 Integrate OCR for text extraction
- [ ] 4.2.1.3 Create LLM prompt builder
- [ ] 4.2.1.4 Build result merging logic
- [ ] 4.2.1.5 Add caching for similar frames

#### 4.2.2 Prompt Engineering for UI
```go
// pkg/vision/prompts.go
package vision

const UIParsingPrompt = `You are analyzing a UI screenshot for automated testing.

Describe the UI elements you see with these details:
1. Element type (button, text field, list, image, etc.)
2. Visible text content
3. Approximate position (top-left, top-right, center, etc.)
4. Current state (enabled, disabled, selected, etc.)

Format your response as JSON:
{
  "elements": [
    {
      "type": "button",
      "text": "Submit",
      "position": "bottom-center",
      "state": "enabled"
    }
  ],
  "screen_type": "login_form",
  "possible_actions": ["click Submit", "enter username"]
}`

const NavigationPrompt = `Given this UI screenshot, what navigation options are available?
List all clickable elements that would take the user to a different screen.`
```

**Tasks:**
- [ ] 4.2.2.1 Create comprehensive prompt templates
- [ ] 4.2.2.2 Implement prompt selection logic
- [ ] 4.2.2.3 Add JSON response parsing
- [ ] 4.2.2.4 Build prompt A/B testing framework
- [ ] 4.2.2.5 Optimize prompts for latency

---

## Phase 5: Distributed Processing Architecture (Week 6)

### 5.1 Orchestrator Service

#### 5.1.1 Central Coordinator
```go
// pkg/orchestrator/coordinator.go
package orchestrator

// Coordinator manages distributed processing across hosts
type Coordinator struct {
    hosts       *HostRegistry
    pipelines   map[string]*Pipeline
    scheduler   *Scheduler
    
    // Frame routing
    routers     map[string]*FrameRouter
}

type Pipeline struct {
    ID          string
    Platform    string
    Capture     *CaptureConfig
    Processors  []ProcessorConfig
    Consumers   []ConsumerConfig
}

// StartPipeline creates distributed processing chain
func (c *Coordinator) StartPipeline(config PipelineConfig) (*Pipeline, error) {
    // 1. Select optimal hosts based on requirements
    // 2. Deploy capture service on source host
    // 3. Deploy processors on available hosts
    // 4. Set up stream routing
    // 5. Start monitoring
}
```

**Tasks:**
- [ ] 5.1.1.1 Implement host selection algorithm
- [ ] 5.1.1.2 Create pipeline lifecycle management
- [ ] 5.1.1.3 Build dynamic scaling logic
- [ ] 5.1.1.4 Add failure recovery mechanisms
- [ ] 5.1.1.5 Implement resource quotas

#### 5.1.2 Frame Routing System
```go
// pkg/orchestrator/router.go
package orchestrator

// FrameRouter distributes frames to processors
type FrameRouter struct {
    source      chan *Frame
    subscribers []chan *Frame
    
    // Routing strategies
    strategy    RoutingStrategy
}

type RoutingStrategy int

const (
    StrategyRoundRobin RoutingStrategy = iota
    StrategyLoadBalanced
    StrategyAffinity
)

// Route distributes frames according to strategy
func (fr *FrameRouter) Route(frames <-chan *Frame) {
    // Round-robin: cycle through subscribers
    // Load-balanced: send to least loaded
    // Affinity: consistent hashing by frame hash
}
```

**Tasks:**
- [ ] 5.1.2.1 Implement routing strategies
- [ ] 5.1.2.2 Create backpressure handling
- [ ] 5.1.2.3 Add frame deduplication
- [ ] 5.1.2.4 Build priority queuing
- [ ] 5.1.2.5 Implement flow control

### 5.2 Worker Pool

#### 5.2.1 Processing Workers
```go
// pkg/worker/frame_processor.go
package worker

// FrameProcessorWorker runs on each host
type FrameProcessorWorker struct {
    id          string
    hostID      string
    
    // Processing components
    cvEngine    *opencv.Engine
    ocrEngine   ocr.Engine
    llmClient   llm.Client
    
    // Task queue
    tasks       chan *ProcessingTask
    results     chan *ProcessingResult
}

// Start begins processing loop
func (w *FrameProcessorWorker) Start(ctx context.Context) error {
    for {
        select {
        case task := <-w.tasks:
            result := w.process(task)
            w.results <- result
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (w *FrameProcessorWorker) process(task *ProcessingTask) *ProcessingResult {
    // Run OpenCV detection
    // Run OCR if needed
    // Run LLM if needed (rate limited)
    // Return structured result
}
```

**Tasks:**
- [ ] 5.2.1.1 Implement worker registration
- [ ] 5.2.1.2 Create task distribution system
- [ ] 5.2.1.3 Build result aggregation
- [ ] 5.2.1.4 Add worker health checks
- [ ] 5.2.1.5 Implement graceful shutdown

#### 5.2.2 GPU Scheduling
```go
// pkg/worker/gpu_scheduler.go
package worker

// GPUScheduler manages GPU resources across hosts
type GPUScheduler struct {
    gpus        map[string]*GPUResource
    queue       []*GPUMemoryRequest
}

type GPUResource struct {
    ID          string
    HostID      string
    Model       string
    TotalVRAM   uint64
    UsedVRAM    uint64
    QueueDepth  int
}

// Allocate finds best GPU for workload
func (gs *GPUScheduler) Allocate(requirements GPUMemoryRequest) (*GPUResource, error) {
    // 1. Filter GPUs by memory requirement
    // 2. Sort by queue depth (least loaded)
    // 3. Reserve VRAM
    // 4. Return allocation
}
```

**Tasks:**
- [ ] 5.2.2.1 Implement GPU discovery
- [ ] 5.2.2.2 Create VRAM accounting
- [ ] 5.2.2.3 Build queue management
- [ ] 5.2.2.4 Add preemption support
- [ ] 5.2.2.5 Implement multi-GPU load balancing

---

## Phase 6: Universal Platform Integration (Week 7)

### 6.1 Android TV Integration

#### 6.1.1 scrcpy Capture Service
```go
// pkg/platform/android/capture_service.go
package android

type AndroidCaptureService struct {
    deviceID    string
    scrcpyPath  string
    
    capture     *capture.AndroidCapture
    bridge      *streaming.ScrcpyRTSPBridge
    
    // Device state
    lastFrame   *Frame
    isConnected bool
}

// Start begins capture for Android device
func (acs *AndroidCaptureService) Start() error {
    // 1. Verify adb connection
    // 2. Check app foreground status
    // 3. Start scrcpy with raw output
    // 4. Bridge to RTSP
    // 5. Start frame processing pipeline
}
```

**Tasks:**
- [ ] 6.1.1.1 Implement adb wrapper
- [ ] 6.1.1.2 Create scrcpy launcher
- [ ] 6.1.1.3 Add device connection monitoring
- [ ] 6.1.1.4 Build app lifecycle detection
- [ ] 6.1.1.5 Integrate with HelixQA runner

#### 6.1.2 Android Input Injection
```go
// pkg/platform/android/input.go
package android

// AndroidInput simulates user interactions
type AndroidInput struct {
    deviceID string
}

// Tap performs touch at coordinates
func (ai *AndroidInput) Tap(x, y int) error {
    return ai.runAdb("shell", "input", "tap", 
        strconv.Itoa(x), strconv.Itoa(y))
}

// Swipe performs swipe gesture
func (ai *AndroidInput) Swipe(x1, y1, x2, y2 int, durationMs int) error

// KeyEvent sends key press
func (ai *AndroidInput) KeyEvent(keyCode int) error

// Text types text string
func (ai *AndroidInput) Text(text string) error
```

**Tasks:**
- [ ] 6.1.2.1 Implement tap/swipe gestures
- [ ] 6.1.2.2 Add key event injection
- [ ] 6.1.2.3 Create text input method
- [ ] 6.1.2.4 Build coordinate transformation
- [ ] 6.1.2.5 Add gesture recording/playback

### 6.2 Desktop (Tauri) Integration

#### 6.2.1 Desktop Capture Adapter
```rust
// Desktop-specific implementation
// src-tauri/src/video/capture.rs

pub struct DesktopCaptureAdapter {
    source: CaptureSource,
    rtsp_endpoint: String,
}

impl DesktopCaptureAdapter {
    pub fn new(source: CaptureSource) -> Self {
        // Platform detection and setup
    }
    
    pub fn start(&self) -> Result<String, String> {
        // Returns RTSP URL for this capture
        // Platform-specific capture initialization
    }
}

// Platform implementations:
// - Linux: PipeWire via GStreamer
// - Windows: DXGI via Windows.Graphics.Capture
// - macOS: ScreenCaptureKit via CoreMediaIO
```

**Tasks:**
- [ ] 6.2.1.1 Implement Linux PipeWire capture
- [ ] 6.2.1.2 Implement Windows DXGI capture
- [ ] 6.2.1.3 Implement macOS ScreenCaptureKit
- [ ] 6.2.1.4 Create RTSP encoding
- [ ] 6.2.1.5 Build window enumeration

#### 6.2.2 Desktop Input Simulation
```rust
// src-tauri/src/input/simulator.rs

pub trait InputSimulator {
    fn click(&self, x: i32, y: i32) -> Result<(), String>;
    fn move_mouse(&self, x: i32, y: i32) -> Result<(), String>;
    fn scroll(&self, delta: i32) -> Result<(), String>;
    fn key_down(&self, key: Key) -> Result<(), String>;
    fn key_up(&self, key: Key) -> Result<(), String>;
}

// Platform implementations using:
// - Linux: X11/XTest or libei
// - Windows: SendInput API
// - macOS: CGEventCreateMouseEvent
```

**Tasks:**
- [ ] 6.2.2.1 Implement mouse simulation
- [ ] 6.2.2.2 Add keyboard simulation
- [ ] 6.2.2.3 Create scroll gesture support
- [ ] 6.2.2.4 Build coordinate mapping
- [ ] 6.2.2.5 Add multi-monitor support

### 6.3 Web (Browser) Integration

#### 6.3.1 Browser Capture Adapter
```typescript
// src/capture/WebRTCAdapter.ts

export class WebRTCAdapter {
    private pc: RTCPeerConnection;
    private signaling: SignalingClient;
    
    async startCapture(source: 'screen' | 'window'): Promise<MediaStream> {
        const stream = await navigator.mediaDevices.getDisplayMedia({
            video: { 
                cursor: 'always',
                displaySurface: source
            }
        });
        
        // Connect to Go WebRTC server
        await this.connect(stream);
        
        return stream;
    }
    
    private async connect(stream: MediaStream): Promise<void> {
        // Create peer connection to Go server
        // Exchange SDP via signaling
        // Start sending video track
    }
}
```

**Tasks:**
- [ ] 6.3.1.1 Implement getDisplayMedia wrapper
- [ ] 6.3.1.2 Create WebRTC peer connection
- [ ] 6.3.1.3 Build signaling client
- [ ] 6.3.1.4 Add connection state management
- [ ] 6.3.1.5 Implement reconnection logic

#### 6.3.2 Browser Action Execution
```typescript
// src/actions/BrowserActionExecutor.ts

export class BrowserActionExecutor {
    async execute(action: UIAction): Promise<void> {
        switch (action.type) {
            case 'click':
                await this.simulateClick(action.target);
                break;
            case 'type':
                await this.simulateTyping(action.target, action.text);
                break;
            case 'scroll':
                await this.simulateScroll(action.delta);
                break;
        }
    }
    
    private async simulateClick(target: ElementTarget): Promise<void> {
        const element = await this.findElement(target);
        
        // Dispatch proper mouse events
        element.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
        element.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
        element.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    }
}
```

**Tasks:**
- [ ] 6.3.2.1 Implement element finding strategies
- [ ] 6.3.2.2 Create mouse event simulation
- [ ] 6.3.2.3 Add keyboard input simulation
- [ ] 6.3.2.4 Build scroll handling
- [ ] 6.3.2.5 Implement action recording

### 6.4 API Integration

For API/backend testing, video capture is not applicable. Instead:

#### 6.4.1 HTTP Request/Response Capture
```go
// pkg/platform/api/request_capture.go
package api

type APICaptureService struct {
    proxy       *httputil.ReverseProxy
    targetURL   string
    
    requests    []CapturedRequest
    mu          sync.RWMutex
}

type CapturedRequest struct {
    Timestamp   time.Time
    Method      string
    Path        string
    Headers     http.Header
    Body        []byte
    Response    *CapturedResponse
}

// Capture records API interaction
func (acs *APICaptureService) Capture(req *http.Request, resp *http.Response) {
    // Store request/response for analysis
}
```

**Tasks:**
- [ ] 6.4.1.1 Implement HTTP proxy capture
- [ ] 6.4.1.2 Create request/response storage
- [ ] 6.4.1.3 Add WebSocket capture support
- [ ] 6.4.1.4 Build diff analysis tools
- [ ] 6.4.1.5 Integrate with HelixQA assertions

---

## Phase 7: HelixQA Integration (Week 8)

### 7.1 Vision Engine Replacement

#### 7.1.1 Local Vision Engine
```go
// pkg/vision/engine.go
package vision

// LocalVisionEngine replaces cloud API calls
type LocalVisionEngine struct {
    parser      *UIParser
    cache       *VisionCache
    
    // Performance metrics
    latencyHist *Histogram
}

// AnalyzeFrame processes single frame locally
func (lve *LocalVisionEngine) AnalyzeFrame(frame *Frame) (*UIAnalysis, error) {
    // Check cache for similar frame
    if cached := lve.cache.Get(frame.Hash); cached != nil {
        return cached, nil
    }
    
    // Run local analysis pipeline
    result, err := lve.parser.Parse(frame)
    if err != nil {
        return nil, err
    }
    
    // Cache result
    lve.cache.Set(frame.Hash, result)
    
    return result, nil
}

type UIAnalysis struct {
    Elements     []UIElement
    TextContent  []TextBlock
    LLMOutput    string
    Timestamp    time.Time
    ProcessingMs int64
}
```

**Tasks:**
- [ ] 7.1.1.1 Implement frame hash caching
- [ ] 7.1.1.2 Create similarity detection
- [ ] 7.1.1.3 Build LRU cache eviction
- [ ] 7.1.1.4 Add performance metrics
- [ ] 7.1.1.5 Integrate with existing Vision interface

#### 7.1.2 Cloud API Fallback (Optional)
```go
// pkg/vision/hybrid_engine.go
package vision

// HybridEngine uses local first, cloud as fallback
type HybridEngine struct {
    local       *LocalVisionEngine
    cloud       *CloudVisionEngine  // Existing
    
    // Circuit breaker for cloud
    breaker     *CircuitBreaker
    
    // Cost tracking
    cloudCalls  int64
}

// AnalyzeFrame tries local first
func (he *HybridEngine) AnalyzeFrame(frame *Frame) (*UIAnalysis, error) {
    // Always try local first
    result, err := he.local.AnalyzeFrame(frame)
    if err == nil && result.Confidence > 0.8 {
        return result, nil
    }
    
    // Fall back to cloud if local confidence low
    if he.breaker.Allow() {
        return he.cloud.AnalyzeFrame(frame)
    }
    
    return result, err  // Return local result even if lower confidence
}
```

**Tasks:**
- [ ] 7.1.2.1 Implement hybrid fallback logic
- [ ] 7.1.2.2 Create circuit breaker pattern
- [ ] 7.1.2.3 Add cost monitoring
- [ ] 7.1.2.4 Build confidence threshold tuning
- [ ] 7.1.2.5 Implement A/B testing framework

### 7.2 Action Decision Pipeline

#### 7.2.1 LLM-Driven Action Selection
```go
// pkg/orchestrator/action_selector.go
package orchestrator

// ActionSelector uses local LLM for decision making
type ActionSelector struct {
    llmClient   llm.VisionClient
    history     *ActionHistory
}

type ActionDecision struct {
    Action      UIAction
    Confidence  float64
    Reasoning   string
}

// DecideNextAction analyzes UI and determines next step
func (as *ActionSelector) DecideNextAction(
    analysis *UIAnalysis, 
    goal string,
) (*ActionDecision, error) {
    prompt := as.buildDecisionPrompt(analysis, goal)
    
    response, err := as.llmClient.Complete(prompt)
    if err != nil {
        return nil, err
    }
    
    // Parse structured decision from LLM output
    return as.parseDecision(response)
}
```

**Tasks:**
- [ ] 7.2.1.1 Implement prompt building
- [ ] 7.2.1.2 Create decision parsing
- [ ] 7.2.1.3 Add history context
- [ ] 7.2.1.4 Build goal decomposition
- [ ] 7.2.1.5 Implement retry logic

---

## Phase 8: Testing & Validation (Week 9)

### 8.1 Unit Tests

#### 8.1.1 OpenCV Tests
```go
// pkg/opencv/bridge_test.go
func TestElementDetection(t *testing.T) {
    // Load test images
    testCases := []struct {
        name     string
        image    string
        expected int  // expected element count
    }{
        {"login_screen", "testdata/login.png", 5},
        {"dashboard", "testdata/dashboard.png", 12},
        {"player", "testdata/player.png", 8},
    }
    
    detector := NewElementDetector()
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            img := LoadTestImage(tc.image)
            elements, err := detector.Detect(img)
            require.NoError(t, err)
            assert.GreaterOrEqual(t, len(elements), tc.expected)
        })
    }
}
```

**Tasks:**
- [ ] 8.1.1.1 Create test image dataset
- [ ] 8.1.1.2 Implement detector tests
- [ ] 8.1.1.3 Add OCR accuracy tests
- [ ] 8.1.1.4 Build feature matching tests
- [ ] 8.1.1.5 Achieve 80%+ coverage

#### 8.1.2 Integration Tests
```go
// tests/integration/pipeline_test.go
func TestFullPipeline(t *testing.T) {
    // Start test RTSP stream
    stream := StartTestStream("testdata/demo_video.mp4")
    defer stream.Stop()
    
    // Create pipeline
    pipeline := NewPipeline(PipelineConfig{
        Source: stream.URL,
        Processors: []ProcessorConfig{
            {Type: "cv", Host: "localhost"},
            {Type: "ocr", Host: "localhost"},
        },
    })
    
    // Process frames
    results := pipeline.Process(100)  // 100 frames
    
    // Verify results
    assert.NotEmpty(t, results)
    assert.True(t, allFramesHaveElements(results))
}
```

**Tasks:**
- [ ] 8.1.2.1 Create test video fixtures
- [ ] 8.1.2.2 Implement pipeline integration tests
- [ ] 8.1.2.3 Add distributed processing tests
- [ ] 8.1.2.4 Build end-to-end scenario tests
- [ ] 8.1.2.5 Create performance benchmarks

### 8.2 Challenge Tests

```yaml
# challenges/video_pipeline.yaml
challenges:
  - id: "video_pipeline_basic"
    name: "Video Pipeline Basic Functionality"
    description: "Verify basic video capture and processing"
    
  - id: "opencv_element_detection"
    name: "OpenCV Element Detection"
    description: "Test UI element detection accuracy"
    
  - id: "ocr_text_extraction"
    name: "OCR Text Extraction"
    description: "Verify text extraction from UI"
    
  - id: "llm_ui_understanding"
    name: "LLM UI Understanding"
    description: "Test LLM-based UI parsing"
    
  - id: "distributed_processing"
    name: "Distributed Processing"
    description: "Verify multi-host processing"
```

**Tasks:**
- [ ] 8.2.1 Create Challenge definitions
- [ ] 8.2.2 Implement Challenge test cases
- [ ] 8.2.3 Add regression tests
- [ ] 8.2.4 Build performance Challenges
- [ ] 8.2.5 Create cost comparison Challenges

---

## Phase 9: Deployment & Operations (Week 10)

### 9.1 Container Deployment

#### 9.1.1 Docker Compose Stack
```yaml
# docker-compose.video-pipeline.yml
version: '3.8'

services:
  mediamtx:
    image: bluenviron/mediamtx:latest
    ports:
      - "8554:8554"  # RTSP
      - "8888:8888"  # WebRTC
    volumes:
      - ./mediamtx.yml:/mediamtx.yml
      
  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama-models:/root/.ollama
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
              
  paddleocr:
    build: ./docker/paddleocr
    environment:
      - CUDA_VISIBLE_DEVICES=0
    volumes:
      - ./models:/models
      
  coordinator:
    build: ./docker/coordinator
    environment:
      - REDIS_URL=redis:6379
      - OLLAMA_URL=http://ollama:11434
    depends_on:
      - redis
      - ollama
      
  worker:
    build: ./docker/worker
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '2'
          memory: 4G
    environment:
      - COORDINATOR_URL=http://coordinator:8080
```

**Tasks:**
- [ ] 9.1.1.1 Create base container images
- [ ] 9.1.1.2 Implement multi-stage builds
- [ ] 9.1.1.3 Add health checks
- [ ] 9.1.1.4 Configure resource limits
- [ ] 9.1.1.5 Build auto-scaling logic

#### 9.1.2 Kubernetes Deployment (Optional)
```yaml
# k8s/video-pipeline-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: video-worker
spec:
  replicas: 3
  selector:
    matchLabels:
      app: video-worker
  template:
    spec:
      containers:
        - name: worker
          image: helixqa/video-worker:latest
          resources:
            limits:
              nvidia.com/gpu: 1
              memory: "8Gi"
              cpu: "4"
```

**Tasks:**
- [ ] 9.1.2.1 Create Kubernetes manifests
- [ ] 9.1.2.2 Implement GPU operator config
- [ ] 9.1.2.3 Add HPA for auto-scaling
- [ ] 9.1.2.4 Configure ingress
- [ ] 9.1.2.5 Build Helm chart

### 9.2 Monitoring & Observability

```go
// pkg/metrics/pipeline_metrics.go
package metrics

var (
    FramesProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "helixqa_frames_processed_total",
            Help: "Total frames processed",
        },
        []string{"platform", "processor"},
    )
    
    ProcessingLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "helixqa_processing_latency_seconds",
            Help:    "Frame processing latency",
            Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
        },
        []string{"platform", "stage"},
    )
    
    OllamaTokensUsed = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "helixqa_ollama_tokens_total",
            Help: "Total tokens used by Ollama",
        },
    )
)
```

**Tasks:**
- [ ] 9.2.1 Implement Prometheus metrics
- [ ] 9.2.2 Create Grafana dashboards
- [ ] 9.2.3 Add structured logging
- [ ] 9.2.4 Build alerting rules
- [ ] 9.2.5 Implement distributed tracing

---

## Cost Analysis

### Before (Cloud APIs)
| Service | Monthly Usage | Cost |
|---------|--------------|------|
| OpenAI GPT-4V | 100K images | $1,000+ |
| Google Vision | 500K requests | $750+ |
| OCR API | 200K requests | $200+ |
| **Total** | | **$1,950+/month** |

### After (Local Pipeline)
| Component | Setup Cost | Monthly Cost |
|-----------|-----------|--------------|
| GPU Server (local) | $2,000 (one-time) | ~$50 (power) |
| CPU Workers (existing) | $0 | $0 |
| Storage | $0 | $0 |
| **Total** | **$2,000** | **$50/month** |

**ROI Break-even: ~1.1 months**

---

## Appendix A: All Open Source Components

| Component | License | URL |
|-----------|---------|-----|
| scrcpy | Apache 2.0 | https://github.com/Genymobile/scrcpy |
| GStreamer | LGPL | https://gstreamer.freedesktop.org |
| FFmpeg | LGPL/GPL | https://ffmpeg.org |
| OpenCV | Apache 2.0 | https://opencv.org |
| Tesseract | Apache 2.0 | https://github.com/tesseract-ocr |
| PaddleOCR | Apache 2.0 | https://github.com/PaddlePaddle/PaddleOCR |
| Ollama | MIT | https://github.com/ollama/ollama |
| LLaVA | Apache 2.0 | https://github.com/haotian-liu/LLaVA |
| MediaMTX | MIT | https://github.com/bluenviron/mediamtx |
| Pion WebRTC | MIT | https://github.com/pion/webrtc |
| Podman | Apache 2.0 | https://podman.io |

---

## Appendix B: Platform-Specific Notes

### Android TV
- scrcpy supports Android TV natively
- Use `--encoder OMX.google.h264.encoder` for compatibility
- D-pad navigation via `adb shell input keyevent`

### Desktop (Tauri)
- Use Tauri's `window` API for capture
- Alternative: native screen capture via platform APIs
- Input simulation varies by OS

### Web (React)
- `getDisplayMedia` requires secure context (HTTPS)
- WebRTC works in all modern browsers
- Input simulation via DOM events

### API (Go)
- HTTP capture via reverse proxy
- WebSocket capture via connection hijacking
- No video processing needed

---

## Summary

This implementation plan provides a **complete, enterprise-grade video processing pipeline** that:

1. **Eliminates all Vision API costs** → $0/month operational
2. **Works across all platforms** → Universal compatibility
3. **Processes in real-time** → <50ms latency target
4. **Scales across hosts** → Distributed architecture
5. **Uses only open source** → No licensing fees

**Total Implementation Time: 10 weeks**  
**Team Size: 2-3 engineers**  
**Cost Savings: $20,000+/year**
