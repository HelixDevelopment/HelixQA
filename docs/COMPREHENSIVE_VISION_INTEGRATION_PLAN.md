# HelixQA Comprehensive Vision Integration Plan
## Bleeding-Edge Computer Vision Architecture for Autonomous QA

**Version**: 2.0  
**Date**: 2026-04-09  
**Status**: Implementation Phase  

---

## Executive Summary

This document presents a comprehensive, production-ready computer vision architecture for HelixQA that integrates:

- **Core OpenCV**: Feature detection (ORB, SIFT, AKAZE), text detection (EAST), contour analysis
- **OCR Engines**: Tesseract OCR, PaddleOCR, Chandra OCR, RapidOCR
- **UI Element Detection**: OmniParser, UGround, UI-DETR-1, RF-DETR, YOLOv8
- **UI Automation Frameworks**: Midscene.js, Aguvis, Optics, Maestro
- **Visual Regression**: Lost Pixel, BackstopJS, custom SSIM-based diff
- **Flow Awareness**: PhantomFlow, NoSmoke, GUIrilla

**Key Benefits**:
- **20-100x faster** screenshot analysis vs LLM-only approach
- **10x reduction** in Vision API costs
- **Offline capability** - no dependency on external APIs
- **Cross-platform** - unified approach for Web, Android, Desktop

---

## 1. Technology Integration Matrix

### 1.1 OCR & Text Recognition Layer

| Technology | Use Case | Integration Priority | Performance |
|------------|----------|---------------------|-------------|
| **Tesseract OCR** | Fast text extraction, button labels | Primary | ~50ms |
| **PaddleOCR** | Multilingual text, complex layouts | Secondary | ~100ms |
| **Chandra OCR** | Handwriting, tables, forms | Tertiary | ~150ms |
| **RapidOCR** | On-device fast inference | Mobile/Edge | ~30ms |

### 1.2 UI Element Detection Layer

| Technology | Use Case | Integration Priority | Model Size |
|------------|----------|---------------------|------------|
| **OmniParser V2** | Comprehensive UI parsing, icon detection | Primary | 4.5GB |
| **UGround** | Visual grounding, click prediction | Primary | 7B/2B |
| **UI-DETR-1** | Real-time element detection | Secondary | 180MB |
| **RF-DETR** | Object detection, custom training | Secondary | 30-126MB |
| **YOLOv8** | Fast bounding box detection | Tertiary | 6-22MB |

### 1.3 UI Automation Frameworks

| Technology | Use Case | Integration Method |
|------------|----------|-------------------|
| **Midscene.js** | Natural language UI automation | Service bridge |
| **Aguvis** | Pure vision agent framework | Model integration |
| **Optics Framework** | Vision-powered test automation | Adapter pattern |
| **Maestro** | Mobile UI automation | Native integration |

### 1.4 Visual Regression Tools

| Technology | Use Case | Integration |
|------------|----------|-------------|
| **Lost Pixel** | Baseline screenshot comparison | Service wrapper |
| **Custom SSIM** | Perceptual diff in GoCV | Native |
| **BackstopJS** | Web visual regression | Optional bridge |

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    HelixQA Vision Pipeline                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Screenshot │  │   Video      │  │   Scrcpy     │  │   Desktop    │     │
│  │   Capture    │  │   Recorder   │  │   Stream     │  │   Capture    │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
│         │                 │                 │                 │             │
│         └─────────────────┴─────────────────┴─────────────────┘             │
│                                    │                                        │
│                                    ▼                                        │
│  ┌──────────────────────────────────────────────────────────────┐          │
│  │              Unified Frame Processor (GoCV)                   │          │
│  │  • Format conversion (RGBA → BGR)                             │          │
│  │  • Resolution normalization                                   │          │
│  │  • Frame buffering                                            │          │
│  └──────────────────────────────┬───────────────────────────────┘          │
│                                 │                                           │
│         ┌───────────────────────┼───────────────────────┐                   │
│         ▼                       ▼                       ▼                   │
│  ┌──────────────┐     ┌──────────────────┐    ┌──────────────┐             │
│  │   Text       │     │   Element        │    │   Layout     │             │
│  │   Extraction │     │   Detection      │    │   Analysis   │             │
│  │   Pipeline   │     │   Pipeline       │    │   Pipeline   │             │
│  │              │     │                  │    │              │             │
│  │ • Tesseract  │     │ • OmniParser     │    │ • Contours   │             │
│  │ • PaddleOCR  │     │ • UGround        │    │ • Edges      │             │
│  │ • Chandra    │     │ • UI-DETR        │    │ • MSER       │             │
│  └──────┬───────┘     └────────┬─────────┘    └──────┬───────┘             │
│         │                      │                      │                     │
│         └──────────────────────┼──────────────────────┘                     │
│                                ▼                                            │
│  ┌──────────────────────────────────────────────────────────────┐          │
│  │              Vision Results Aggregator                        │          │
│  │  • Merge overlapping detections                               │          │
│  │  • Confidence scoring                                         │          │
│  │  • Coordinate normalization                                   │          │
│  └──────────────────────────────┬───────────────────────────────┘          │
│                                 │                                           │
│                                 ▼                                           │
│  ┌──────────────────────────────────────────────────────────────┐          │
│  │              Smart Navigator (CV-Augmented)                   │          │
│  │  • Find element by text/description                           │          │
│  │  • Navigate using visual cues                                 │          │
│  │  • Verify actions visually                                    │          │
│  └──────────────────────────────────────────────────────────────┘          │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────┐          │
│  │              External Service Bridges (Optional)              │          │
│  │  • Midscene.js integration                                    │          │
│  │  • Aguvis model inference                                     │          │
│  │  • Lost Pixel comparison                                      │          │
│  └──────────────────────────────────────────────────────────────┘          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Implementation Phases

### Phase 1: Core OpenCV Foundation (Week 1-2)

**Deliverables**:
1. GoCV setup and integration
2. Element detection with ORB/SIFT
3. Text detection with EAST
4. Tesseract OCR integration

**Key Files**:
- `pkg/vision/detector.go` - Core detection interface
- `pkg/vision/orb.go` - ORB feature detection
- `pkg/vision/text.go` - EAST + Tesseract pipeline
- `pkg/vision/layout.go` - Contour-based layout analysis

**Tests**: 100% coverage for all new modules

### Phase 2: Advanced OCR Integration (Week 3)

**Deliverables**:
1. PaddleOCR integration (multilingual support)
2. Chandra OCR integration (complex documents)
3. OCR result merging and confidence scoring
4. Language auto-detection

**Key Files**:
- `pkg/vision/ocr/paddle.go`
- `pkg/vision/ocr/chandra.go`
- `pkg/vision/ocr/ensemble.go` - Multi-OCR fusion

**Challenges**: 5 OCR-specific challenges for validation

### Phase 3: UI Element Detection Models (Week 4)

**Deliverables**:
1. OmniParser V2 integration (REST/gRPC)
2. UGround integration (local inference)
3. UI-DETR-1 integration (real-time)
4. Detection ensemble for best results

**Key Files**:
- `pkg/vision/models/omniparser.go`
- `pkg/vision/models/uground.go`
- `pkg/vision/models/uidetr.go`
- `pkg/vision/models/ensemble.go`

**Challenges**: 10 element detection challenges

### Phase 4: Navigation & Action (Week 5)

**Deliverables**:
1. CVNavigator implementation
2. Smart action verification
3. Stability detection
4. Visual assertion framework

**Key Files**:
- `pkg/navigator/cv_navigator.go`
- `pkg/vision/verifier.go`
- `pkg/vision/stability.go`

**Challenges**: 15 navigation challenges

### Phase 5: External Framework Integration (Week 6)

**Deliverables**:
1. Midscene.js bridge
2. Aguvis integration
3. Optics adapter
4. Maestro runner

**Key Files**:
- `pkg/bridges/midscene/bridge.go`
- `pkg/bridges/aguvis/client.go`
- `pkg/bridges/optics/adapter.go`
- `pkg/maestro/enhanced_runner.go`

**Challenges**: 10 framework integration challenges

### Phase 6: Visual Regression & Testing (Week 7)

**Deliverables**:
1. Lost Pixel integration
2. Custom SSIM-based diff
3. Baseline management
4. Regression report generation

**Key Files**:
- `pkg/regression/lostpixel.go`
- `pkg/regression/ssim.go`
- `pkg/regression/baseline.go`

**Challenges**: 10 visual regression challenges

### Phase 7: Optimization & Production (Week 8)

**Deliverables**:
1. GPU acceleration (CUDA)
2. Caching layer
3. Performance monitoring
4. Configuration management

**Key Files**:
- `pkg/vision/gpu.go`
- `pkg/vision/cache.go`
- `pkg/vision/metrics.go`

---

## 4. Directory Structure

```
HelixQA/pkg/vision/
├── core/                          # Core vision interfaces
│   ├── interfaces.go
│   ├── frame.go
│   └── config.go
│
├── detection/                     # Element detection
│   ├── orb.go                     # ORB feature detection
│   ├── sift.go                    # SIFT feature detection
│   ├── template.go                # Template matching
│   └── detector_test.go
│
├── text/                          # Text extraction
│   ├── tesseract.go               # Tesseract OCR
│   ├── east.go                    # EAST text detection
│   ├── paddle.go                  # PaddleOCR integration
│   ├── chandra.go                 # Chandra OCR integration
│   ├── ensemble.go                # Multi-OCR fusion
│   └── text_test.go
│
├── layout/                        # Layout analysis
│   ├── contours.go                # Contour detection
│   ├── edges.go                   # Edge detection
│   ├── regions.go                 # Region classification
│   ├── hierarchy.go               # Layout tree
│   └── layout_test.go
│
├── models/                        # ML model integrations
│   ├── omniparser/                # OmniParser V2
│   │   ├── client.go
│   │   ├── parser.go
│   │   └── omniparser_test.go
│   ├── uground/                   # UGround visual grounding
│   │   ├── client.go
│   │   ├── inference.go
│   │   └── uground_test.go
│   ├── uidetr/                    # UI-DETR-1
│   │   ├── client.go
│   │   ├── detector.go
│   │   └── uidetr_test.go
│   └── ensemble.go                # Model ensemble
│
├── navigation/                    # CV-based navigation
│   ├── navigator.go               # CVNavigator
│   ├── finder.go                  # Element finder
│   ├── verifier.go                # Action verifier
│   └── navigation_test.go
│
├── regression/                    # Visual regression
│   ├── ssim.go                    # SSIM comparison
│   ├── diff.go                    # Pixel diff
│   ├── baseline.go                # Baseline management
│   └── regression_test.go
│
├── cache/                         # Caching layer
│   ├── lru.go
│   ├── features.go
│   └── cache_test.go
│
├── gpu/                           # GPU acceleration
│   ├── cuda.go
│   └── opencl.go
│
└── utils/                         # Utilities
    ├── image.go
    ├── colors.go
    └── math.go

HelixQA/pkg/bridges/
├── midscene/                      # Midscene.js bridge
│   ├── bridge.go
│   ├── client.go
│   └── midscene_test.go
│
├── aguvis/                        # Aguvis integration
│   ├── client.go
│   ├── inference.go
│   └── aguvis_test.go
│
└── optics/                        # Optics framework adapter
    ├── adapter.go
    └── optics_test.go

HelixQA/pkg/maestro/
├── enhanced_runner.go             # Enhanced Maestro runner
├── vision_steps.go                # Vision-based steps
└── maestro_test.go
```

---

## 5. Key Technologies Deep Dive

### 5.1 OmniParser V2 Integration

**Capabilities**:
- Icon detection using fine-tuned YOLOv8
- Icon description using Florence-2
- Interactive element identification
- Bounding box generation with labels

**Integration**:
```go
// Local deployment via Docker
// REST API endpoint: http://localhost:8000/parse

type OmniParserClient struct {
    endpoint string
    timeout  time.Duration
}

func (c *OmniParserClient) ParseScreenshot(
    ctx context.Context,
    screenshot []byte,
) (*ParsedUI, error) {
    // Send screenshot to OmniParser service
    // Receive structured element list
}
```

**Performance**: ~0.6s per frame on A100, ~0.8s on RTX 4090

### 5.2 UGround Integration

**Capabilities**:
- Visual grounding for GUI elements
- Natural language to coordinate mapping
- Cross-platform support (web, mobile, desktop)
- Training on 10M GUI elements

**Integration**:
```go
// vLLM server for local inference
// Model: osunlp/UGround-V1-7B

type UGroundClient struct {
    vllmEndpoint string
}

func (c *UGroundClient) GroundElement(
    ctx context.Context,
    screenshot []byte,
    description string,
) (*GroundingResult, error) {
    // Returns: x, y coordinates normalized to [0,1000)
}
```

**Accuracy**: 74.1% average on ScreenSpot benchmark

### 5.3 PaddleOCR Integration

**Capabilities**:
- 100+ language support
- PP-OCRv5 for mobile
- PP-StructureV3 for layout analysis
- PaddleOCR-VL for document understanding

**Integration**:
```go
// Python service via gRPC/REST
// FastAPI wrapper around PaddleOCR

type PaddleOCRClient struct {
    endpoint string
}

func (c *PaddleOCRClient) Recognize(
    ctx context.Context,
    image []byte,
    lang string,
) (*OCRResult, error) {
    // Returns text with bounding boxes
}
```

### 5.4 Chandra OCR Integration

**Capabilities**:
- State-of-the-art on olmOCR benchmark (85.9%)
- Complex tables, forms, handwriting
- 90+ language support
- Layout preservation (Markdown/HTML/JSON)

**Integration**:
```go
// vLLM or HuggingFace inference
// Model: datalab-to/chandra-ocr-2

type ChandraClient struct {
    endpoint string
}

func (c *ChandraClient) ExtractDocument(
    ctx context.Context,
    image []byte,
) (*DocumentResult, error) {
    // Returns structured document with layout
}
```

### 5.5 Midscene.js Bridge

**Capabilities**:
- Natural language UI automation
- Playwright/Puppeteer integration
- Android automation via ADB
- MCP (Model Context Protocol) support

**Integration**:
```go
// Node.js service with REST API
// JavaScript SDK wrapper

type MidsceneBridge struct {
    client *http.Client
    baseURL string
}

func (b *MidsceneBridge) ExecuteAction(
    ctx context.Context,
    action string, // "Click the Login button"
) error {
    // Send to Midscene service
    // Execute via connected browser/device
}
```

---

## 6. Testing Strategy

### 6.1 Unit Tests (100% Coverage)

Every module must have:
- Interface mocking
- Edge case coverage
- Error path testing
- Performance benchmarks

```go
// Example test structure
func TestElementDetector_FindElement(t *testing.T) {
    tests := []struct {
        name        string
        template    string
        confidence  float64
        wantFound   bool
        wantErr     bool
    }{
        // Happy path
        {
            name:       "find existing button",
            template:   "login_button",
            confidence: 0.8,
            wantFound:  true,
        },
        // Edge cases
        {
            name:       "low confidence threshold",
            template:   "login_button",
            confidence: 0.99,
            wantFound:  false,
        },
        {
            name:       "non-existent template",
            template:   "nonexistent",
            confidence: 0.8,
            wantErr:    true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 6.2 Integration Tests

Test complete pipelines:
- Screenshot → Element detection → Action
- Video stream → Text extraction → Verification
- Navigation flows across platforms

### 6.3 Challenge-Based Validation

Create 50+ challenges for comprehensive validation:

**OCR Challenges** (10):
- Extract text from complex layouts
- Handwriting recognition
- Multi-language documents
- Table data extraction

**Element Detection Challenges** (15):
- Find buttons by text
- Identify icons by description
- Navigate nested menus
- Handle dynamic content

**Navigation Challenges** (15):
- Complete user flows
- Error recovery
- State verification
- Cross-page navigation

**Visual Regression Challenges** (10):
- Detect visual changes
- Baseline management
- Threshold tuning
- Report generation

---

## 7. Configuration

```yaml
# vision-config.yaml

# Core OpenCV settings
opencv:
  enabled: true
  gpu_enabled: true
  cache_size: 1000
  
  # Feature detection
  feature_detection:
    algorithm: "ORB"  # ORB, SIFT, AKAZE
    max_features: 500
    scale_factor: 1.2
    n_levels: 8
  
  # Text detection
  text_detection:
    east_model: "models/east/frozen_east_text_detection.pb"
    confidence_threshold: 0.5
    nms_threshold: 0.4

# OCR engines
ocr:
  primary: "tesseract"
  fallback: "paddle"
  
  tesseract:
    data_path: "/usr/share/tesseract-ocr/4.00/tessdata"
    languages: ["eng"]
    
  paddle:
    endpoint: "http://localhost:8080"
    use_gpu: false
    
  chandra:
    endpoint: "http://localhost:8000/v1"
    model: "datalab-to/chandra-ocr-2"

# UI element detection models
models:
  omniparser:
    enabled: true
    endpoint: "http://localhost:8000"
    box_threshold: 0.5
    iou_threshold: 0.3
    
  uground:
    enabled: true
    endpoint: "http://localhost:8000/v1"
    model: "osunlp/UGround-V1-7B"
    temperature: 0
    
  uidetr:
    enabled: true
    model_path: "models/uidetr/ui-detr-1.onnx"
    confidence_threshold: 0.7

# External frameworks
frameworks:
  midscene:
    enabled: true
    endpoint: "http://localhost:3000"
    
  aguvis:
    enabled: false
    endpoint: "http://localhost:8000"

# Visual regression
regression:
  enabled: true
  tool: "ssim"  # ssim, lost-pixel
  threshold: 0.95
  baseline_dir: "baselines/"
```

---

## 8. Deployment

### 8.1 Container Setup

```dockerfile
# Dockerfile.vision
FROM gocv/opencv:4.8.0

# Install Tesseract
RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    tesseract-ocr-eng \
    tesseract-ocr-chi-sim \
    libtesseract-dev

# Install Python for PaddleOCR/Chandra
RUN apt-get install -y python3 python3-pip
RUN pip3 install paddleocr chandra-ocr

# Copy HelixQA vision modules
COPY pkg/vision /app/pkg/vision

# Build Go binary
RUN go build -o helixqa-vision ./cmd/vision

EXPOSE 8080
CMD ["./helixqa-vision"]
```

### 8.2 Docker Compose

```yaml
# docker-compose.vision.yml
version: '3.8'

services:
  helixqa-vision:
    build:
      context: .
      dockerfile: Dockerfile.vision
    ports:
      - "8080:8080"
    volumes:
      - ./models:/app/models:ro
      - ./baselines:/app/baselines
    environment:
      - OPENCV_GPU_ENABLED=true
      - TESSDATA_PREFIX=/usr/share/tesseract-ocr/4.00/tessdata
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]

  omniparser:
    image: omniparser-v2:latest
    ports:
      - "8000:8000"
    volumes:
      - ./models/omniparser:/app/models:ro

  paddleocr:
    image: paddleocr-service:latest
    ports:
      - "8081:8080"
```

---

## 9. Performance Targets

| Operation | Target | Current (LLM-only) | Improvement |
|-----------|--------|-------------------|-------------|
| Text extraction | <100ms | 2-5s | 20-50x |
| Element detection | <200ms | 2-5s | 10-25x |
| Layout analysis | <150ms | N/A | New capability |
| Screenshot compare | <50ms | 2-5s | 40-100x |
| Navigation action | <500ms | 3-8s | 6-16x |

---

## 10. Cost Analysis

| Approach | Per 1K Screenshots | Monthly (10K tests) |
|----------|-------------------|---------------------|
| **LLM-only (GPT-4V)** | $20-40 | $2,000-4,000 |
| **OpenCV + Tesseract** | $0 (local) | $0 |
| **OpenCV + PaddleOCR** | $0 (local) | $0 |
| **OmniParser (self-hosted)** | ~$5 (GPU cost) | ~$500 |
| **Hybrid (this plan)** | ~$2-5 | ~$200-500 |

**Savings**: 90-95% reduction in Vision API costs

---

## 11. Next Steps

1. **Immediate**: Begin Phase 1 implementation (Core OpenCV)
2. **Week 2**: Complete OCR pipeline with Tesseract
3. **Week 3**: Integrate OmniParser V2
4. **Week 4**: Build CVNavigator
5. **Week 5**: Add PaddleOCR and Chandra OCR
6. **Week 6**: Integrate UGround and UI-DETR
7. **Week 7**: Build visual regression framework
8. **Week 8**: Performance optimization and deployment

---

## References

1. [GoCV Documentation](https://gocv.io/)
2. [OmniParser V2 Paper](https://arxiv.org/abs/2408.11432)
3. [UGround Project](https://osu-nlp-group.github.io/UGround/)
4. [PaddleOCR GitHub](https://github.com/PaddlePaddle/PaddleOCR)
5. [Chandra OCR GitHub](https://github.com/datalab-to/chandra)
6. [Midscene.js Documentation](https://midscenejs.com/)
7. [Aguvis Paper](https://aguvis-project.github.io/)
8. [RF-DETR GitHub](https://github.com/roboflow/rf-detr)
