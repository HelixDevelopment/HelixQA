# OpenClaw Ultimate Capabilities Extension: Comprehensive Integration Plan

## Forensic-Level Integration of Game-Changer Technologies for Real-Time Autonomous UI/UX Control

**Report Date:** 2026/04/18
**Classification:** Strategic Engineering Architecture Document
**Scope:** Extension of OpenClaw architecture with OpenCV, Vulkan, OpenGL, CUDA, RTX, Low-Level OS APIs, GPU-Accelerated Capture, Recording Pipelines, and Cross-Platform Automation

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Game-Changer Technology Overview](#2-game-changer-technology-overview)
3. [OpenCV Heavy Use - Real-Time Computer Vision Pipeline](#3-opencv-heavy-use)
4. [Vulkan & OpenGL GPU-Accelerated Processing](#4-vulkan--opengl)
5. [CUDA & RTX GPU Compute for Real-Time Inference](#5-cuda--rtx)
6. [Low-Level OS-Specific Technologies](#6-low-level-os-technologies)
7. [Real-Time Screen Capture Architectures](#7-real-time-screen-capture)
8. [High-Performance Recording & Streaming Pipelines](#8-recording--streaming)
9. [Hook & Interception Systems](#9-hook--interception)
10. [Cross-Platform Input Simulation](#10-input-simulation)
11. [TUI (Terminal User Interface) Automation](#11-tui-automation)
12. [Mobile Device Automation Integration](#12-mobile-automation)
13. [Complete Integration Architecture](#13-integration-architecture)
14. [Step-by-Step Implementation Guide](#14-implementation-guide)
15. [Source Code Reference Map](#15-source-code-references)

---

## 1. Executive Summary

### 1.1 The Strategic Vision: Beyond LLM/Vision Models

The existing OpenClaw architecture, as analyzed in OpenClawing2.md, provides a solid foundation for conversational AI with basic tool integration. However, the comparative analysis reveals a critical gap: **all evaluated frameworks (Anthropic computer-use-demo, browser-use, Skyvern, Stagehand, UI-TARS) rely on high-level abstractions that introduce latency, reduce precision, and lack true real-time processing capabilities.**

This document presents a **revolutionary extension** that brings industrial-grade, low-level technologies into the OpenClaw ecosystem. The goal is to create a system capable of:

- **Full autonomous real-time interaction** with Web, Mobile, Desktop, API, and TUI applications
- **Sub-16ms screen capture and analysis** using GPU-zero-copy pipelines
- **Real-time high-resolution recording** with hardware-accelerated encoding
- **In-depth visual analysis** through OpenCV pipelines running on CUDA/Vulkan compute
- **Universal UI element detection** combining DOM parsing, computer vision, and accessibility APIs
- **Kernel-level input injection** for undetectable, precise interaction
- **Hook-based observation** of application behavior without modification

### 1.2 Core Game-Changer Technologies

| Technology | Role | Performance Impact | Integration Complexity |
|------------|------|-------------------|----------------------|
| **OpenCV (GPU)** | Real-time image processing, template matching, object detection | 10-100x speedup with CUDA | Medium |
| **Vulkan Compute** | Cross-platform GPU compute shaders for image analysis | Eliminates CPU-GPU copy overhead | High |
| **CUDA/TensorRT** | Real-time inference acceleration on RTX GPUs | 100-1000x vs CPU inference | Medium-High |
| **DXGI Desktop Duplication** | Hardware-accelerated Windows screen capture | <5ms capture latency | Medium |
| **DMA-BUF/VAAPI** | Zero-copy Linux GPU capture and encode | <5ms, no CPU involvement | High |
| **NVFBC/NVENC** | NVIDIA framebuffer capture + hardware encoding | <2ms capture + encode | High |
| **evdev/uinput** | Linux kernel-level input injection | Microsecond precision | Medium |
| **SendInput/LLHooks** | Windows low-level input simulation | Millisecond precision | Low |
| **LD_PRELOAD/plthook** | API interception and monitoring | Zero-overhead when idle | High |
| **FFmpeg/GStreamer** | Universal media pipeline framework | Hardware-accelerated | Medium |
| **WebRTC/WHIP** | Sub-second streaming protocol | <100ms end-to-end latency | Medium |
| **scrcpy/ADB** | Android device control and mirroring | 1-3ms latency | Low |
| **pilotty/PTY** | TUI automation via pseudoterminals | Native terminal speed | Medium |

### 1.3 Expected Outcomes

After full integration, OpenClaw will achieve:
- **<16ms reaction time** from screen change to action (human-competitive)
- **4K@60fps recording capability** with <5% CPU overhead
- **Real-time multi-screen analysis** across Web, Desktop, Mobile, and Terminal
- **Precise coordinate-based interaction** backed by GPU-accelerated vision
- **Comprehensive audit trail** through hook-based observation

---

## 2. Game-Changer Technology Overview

### 2.1 Technology Classification Matrix

```
LAYER 0: HARDWARE ABSTRACTION
├── NVIDIA: CUDA, TensorRT, NVFBC, NVENC, NVIFR, Maxine SDK
├── AMD: ROCm, AMF, GPUOpen
├── Intel: oneAPI, OpenVINO, QuickSync, VAAPI
└── Cross-vendor: Vulkan, OpenCL, DMA-BUF

LAYER 1: CAPTURE ENGINE
├── Windows: DXGI Desktop Duplication, GDI BitBlt, Magnification API, WGC
├── Linux: KMS/DRM, DMA-BUF, PipeWire, X11 SHM, Wayland screencopy
├── macOS: ScreenCaptureKit, CGDisplayStream, AX API
└── Mobile: ADB screenrecord, scrcpy, iOS QuickTime

LAYER 2: PROCESSING PIPELINE
├── OpenCV (CPU/GPU/CUDA/OpenCL)
├── Vulkan Compute Shaders
├── GLSL/OpenGL Processing
├── FFmpeg Filters (CPU/VAAPI/NVENC)
└── GStreamer Elements

LAYER 3: ANALYSIS ENGINE
├── Template Matching (OpenCV cv::matchTemplate)
├── Feature Detection (SIFT, SURF, ORB)
├── Object Detection (YOLO, EfficientDet via TensorRT)
├── OCR (Tesseract, PaddleOCR, NVidia TAO)
└── Accessibility Tree Parsing

LAYER 4: INTERACTION ENGINE
├── Linux: evdev/uinput, libevdev, X11 XTest
├── Windows: SendInput, mouse_event/keybd_event, LLHooks
├── macOS: CGEventPost, AXUIElementPerformAction
└── Mobile: ADB input, UIAutomator2, XCUITest

LAYER 5: OBSERVATION ENGINE
├── LD_PRELOAD (Linux dynamic linking interception)
├── plthook (PLT/GOT hooking)
├── Accessibility API monitoring
├── Windows SetWindowsHookEx (WH_KEYBOARD_LL, WH_MOUSE_LL)
└── D-Bus signal monitoring (Linux)

LAYER 6: RECORDING & STREAMING
├── FFmpeg (x264, NVENC, VAAPI, AMF)
├── GStreamer (pipeline-based)
├── OBS libobs (plugin architecture)
├── WebRTC/WHIP/WHEP (sub-second streaming)
└── MKV/MP4 segmented recording

LAYER 7: PLATFORM AUTOMATION
├── Web: Playwright, CDP, Puppeteer
├── Desktop: pyautogui, RobotGo, nut.js, libnut
├── TUI: pilotty, node-pty, pty.js, tmux control mode
├── Android: scrcpy, ADB, UIAutomator2, Appium
└── iOS: XCUITest, WebDriverAgent, go-ios
```

---

## 3. OpenCV Heavy Use - Real-Time Computer Vision Pipeline

### 3.1 Architecture Overview

OpenCV provides the foundational image processing capabilities that enable real-time screen analysis. When GPU-accelerated via CUDA or OpenCL, OpenCV operations achieve the throughput necessary for 60fps analysis.

**Source Reference:** OpenCV 4.x modules:
- `modules/core/` - Core data structures (cv::Mat, cv::UMat, cv::cuda::GpuMat)
- `modules/imgproc/` - Image processing (resize, threshold, edge detection)
- `modules/imgcodecs/` - Image encoding/decoding
- `modules/video/` - Video analysis (optical flow, background subtraction)
- `modules/objdetect/` - Object detection (cascades, HOG)
- `modules/features2d/` - Feature detection and matching
- `modules/cuda*` - GPU-accelerated modules

### 3.2 GPU-Accelerated OpenCV Pipeline

```cpp
// File: src/vision/gpu_pipeline.cpp
// GPU-accelerated screen analysis pipeline for OpenClaw

#include <opencv2/opencv.hpp>
#include <opencv2/cudaarithm.hpp>
#include <opencv2/cudaimgproc.hpp>
#include <opencv2/cudafilters.hpp>
#include <opencv2/cudaobjdetect.hpp>
#include <opencv2/cudafeatures2d.hpp>

namespace openclaw {
namespace vision {

class GPUAnalysisPipeline {
private:
    cv::cuda::Stream stream_;
    cv::Ptr<cv::cuda::TemplateMatching> tmpl_matcher_;
    cv::Ptr<cv::cuda::Filter> gaussian_filter_;
    cv::Ptr<cv::cuda::CannyEdgeDetector> canny_detector_;
    cv::cuda::GpuMat gpu_frame_;
    cv::cuda::GpuMat gpu_gray_;
    cv::cuda::GpuMat gpu_processed_;
    
public:
    // Initialize with CUDA stream for async processing
    void Initialize(int width, int height) {
        // Pre-allocate GPU memory to avoid allocation during capture
        gpu_frame_.create(height, width, CV_8UC4);
        gpu_gray_.create(height, width, CV_8UC1);
        gpu_processed_.create(height, width, CV_8UC1);
        
        // Create reusable GPU operators
        tmpl_matcher_ = cv::cuda::createTemplateMatching(
            CV_8UC1, cv::TM_CCOEFF_NORMED);
        gaussian_filter_ = cv::cuda::createGaussianFilter(
            CV_8UC1, CV_8UC1, cv::Size(5, 5), 1.5);
        canny_detector_ = cv::cuda::createCannyEdgeDetector(
            50, 150, 3, true); // L2 gradient
    }
    
    // Process frame entirely on GPU - zero CPU-GPU transfer during processing
    UIElementList AnalyzeFrame(const cv::cuda::GpuMat& input_bgra) {
        // Convert to grayscale on GPU
        cv::cuda::cvtColor(input_bgra, gpu_gray_, cv::COLOR_BGRA2GRAY, 
                          0, stream_);
        
        // Gaussian blur for noise reduction
        gaussian_filter_->apply(gpu_gray_, gpu_processed_, stream_);
        
        // Edge detection for UI element boundaries
        cv::cuda::GpuMat edges;
        canny_detector_->detect(gpu_processed_, edges, stream_);
        
        // Find contours (using CPU fallback - cv::cuda::findContours doesn't exist)
        cv::Mat cpu_edges;
        edges.download(cpu_edges, stream_);
        stream_.waitForCompletion();
        
        std::vector<std::vector<cv::Point>> contours;
        cv::findContours(cpu_edges, contours, cv::RETR_TREE, 
                        cv::CHAIN_APPROX_SIMPLE);
        
        return ExtractUIElements(contours);
    }
    
    // GPU-accelerated template matching for element finding
    cv::Point2f FindTemplate(const cv::cuda::GpuMat& screen, 
                             const cv::cuda::GpuMat& template_img) {
        cv::cuda::GpuMat result;
        tmpl_matcher_->match(screen, template_img, result, stream_);
        
        double min_val, max_val;
        cv::Point min_loc, max_loc;
        cv::cuda::minMaxLoc(result, &min_val, &max_val, 
                           &min_loc, &max_loc, cv::noArray(), stream_);
        
        stream_.waitForCompletion();
        return cv::Point2f(max_loc.x, max_loc.y);
    }
};

} // namespace vision
} // namespace openclaw
```

### 3.3 OpenCL Backend for Cross-Vendor GPU Support

```cpp
// File: src/vision/opencl_pipeline.cpp
// Cross-vendor GPU support via OpenCL

#include <opencv2/core/ocl.hpp>

namespace openclaw {
namespace vision {

class OpenCLPipeline {
public:
    bool InitializeOpenCL() {
        if (!cv::ocl::haveOpenCL()) {
            return false;
        }
        
        cv::ocl::setUseOpenCL(true);
        
        // Select best GPU device
        cv::ocl::Context context;
        if (!context.create(cv::ocl::Device::TYPE_GPU)) {
            return false;
        }
        
        // Print available devices
        for (int i = 0; i < context.ndevices(); i++) {
            cv::ocl::Device device = context.device(i);
            LOG_INFO("OpenCL Device {}: {}", i, device.name());
        }
        
        cv::ocl::Device(context.device(0)); // Select first GPU
        return true;
    }
    
    // UMat operations automatically use OpenCL
    cv::UMat ProcessWithOpenCL(const cv::Mat& input) {
        cv::UMat u_input = input.getUMat(cv::ACCESS_READ);
        cv::UMat u_gray, u_blurred, u_result;
        
        cv::cvtColor(u_input, u_gray, cv::COLOR_BGRA2GRAY);
        cv::GaussianBlur(u_gray, u_blurred, cv::Size(5, 5), 1.5);
        cv::Canny(u_blurred, u_result, 50, 150);
        
        return u_result;
    }
};

} // namespace vision
} // namespace openclaw
```

### 3.4 Integration with OpenClaw's DOM Processing

From OpenClawing2.md Section 4.2, `browser-use` implements a custom DOM processing service in `browser_use/browser/custom_browser.py`. OpenCV extends this with visual verification:

```typescript
// File: src/agents/vision-dom-hybrid.ts
// Hybrid DOM + Vision analysis for maximum reliability

import * as cv from '@u4/opencv4nodejs';

interface DOMElement {
  selector: string;
  boundingBox?: { x: number; y: number; width: number; height: number };
  text?: string;
  tagName: string;
}

interface VisionElement {
  boundingBox: { x: number; y: number; width: number; height: number };
  confidence: number;
  type: 'button' | 'input' | 'text' | 'image' | 'icon';
  ocrText?: string;
}

class HybridDOMVisionAnalyzer {
  private gpuPipeline: GPUAnalysisPipeline;
  
  // Correlate DOM elements with vision-detected elements
  async correlateElements(
    domElements: DOMElement[],
    screenshot: cv.Mat,
    highlightedScreenshot: cv.Mat
  ): Promise<Map<DOMElement, VisionElement>> {
    const correlations = new Map<DOMElement, VisionElement>();
    
    for (const domEl of domElements) {
      if (!domEl.boundingBox) continue;
      
      // Extract region from screenshot
      const roi = screenshot.getRegion(new cv.Rect(
        domEl.boundingBox.x,
        domEl.boundingBox.y,
        domEl.boundingBox.width,
        domEl.boundingBox.height
      ));
      
      // Run visual classification
      const visionEl = await this.classifyElementVisual(roi);
      
      if (visionEl.confidence > 0.85) {
        correlations.set(domEl, visionEl);
      }
    }
    
    return correlations;
  }
  
  // Detect elements missed by DOM parsing (shadow DOM, canvas, etc.)
  async detectMissedElements(
    screenshot: cv.Mat,
    domElements: DOMElement[]
  ): Promise<VisionElement[]> {
    // Convert to grayscale
    const gray = screenshot.cvtColor(cv.COLOR_BGRA2GRAY);
    
    // Morphological operations to find component boundaries
    const kernel = cv.getStructuringElement(
      cv.MORPH_RECT, 
      new cv.Size(5, 5)
    );
    const dilated = gray.dilate(kernel);
    
    // Find MSER regions (stable text/element regions)
    const mser = new cv.MSERDetector();
    const regions = mser.detectRegions(dilated);
    
    const missedElements: VisionElement[] = [];
    
    for (const region of regions) {
      const bbox = region.bbox;
      
      // Check if this region is NOT covered by any DOM element
      const covered = domElements.some(de => 
        de.boundingBox && this.iou(de.boundingBox, bbox) > 0.5
      );
      
      if (!covered) {
        missedElements.push({
          boundingBox: bbox,
          confidence: region.confidence,
          type: await this.classifyRegionType(screenshot.getRegion(region.rect))
        });
      }
    }
    
    return missedElements;
  }
  
  private iou(a: any, b: any): number {
    const x1 = Math.max(a.x, b.x);
    const y1 = Math.max(a.y, b.y);
    const x2 = Math.min(a.x + a.width, b.x + b.width);
    const y2 = Math.min(a.y + a.height, b.y + b.height);
    
    const intersection = Math.max(0, x2 - x1) * Math.max(0, y2 - y1);
    const areaA = a.width * a.height;
    const areaB = b.width * b.height;
    
    return intersection / (areaA + areaB - intersection);
  }
}
```

---

## 4. Vulkan & OpenGL GPU-Accelerated Processing

### 4.1 Vulkan Compute for Image Processing

Vulkan Compute provides a cross-platform, vendor-neutral GPU compute API that eliminates the need for CUDA-specific code on non-NVIDIA hardware.

**Source Reference:** vkCompViz project (`github.com/ichlubna/vkCompViz`)

```glsl
// File: src/shaders/screen_analysis.comp
// Vulkan compute shader for real-time screen analysis
#version 450

layout(local_size_x = 16, local_size_y = 16) in;

// Input screen texture (RGBA8)
layout(set = 0, binding = 0, rgba8) readonly uniform image2D inputScreen;

// Output edge detection result (R8)
layout(set = 0, binding = 1, r8) writeonly uniform image2D outputEdges;

// Output UI element mask (R8)
layout(set = 0, binding = 2, r8) writeonly uniform image2D elementMask;

// Uniform parameters
layout(set = 0, binding = 3) uniform Params {
    float edgeThreshold;
    float minElementSize;
    float maxElementSize;
    uint screenWidth;
    uint screenHeight;
} params;

// Sobel edge detection
vec2 sobel(ivec2 coord) {
    float gx = 0.0;
    float gy = 0.0;
    
    // 3x3 Sobel kernel
    float[9] kernel_x = float[9](-1, 0, 1, -2, 0, 2, -1, 0, 1);
    float[9] kernel_y = float[9](-1, -2, -1, 0, 0, 0, 1, 2, 1);
    
    for (int i = -1; i <= 1; i++) {
        for (int j = -1; j <= 1; j++) {
            vec4 color = imageLoad(inputScreen, coord + ivec2(i, j));
            float gray = dot(color.rgb, vec3(0.299, 0.587, 0.114));
            int idx = (i + 1) * 3 + (j + 1);
            gx += gray * kernel_x[idx];
            gy += gray * kernel_y[idx];
        }
    }
    
    return vec2(gx, gy);
}

void main() {
    ivec2 coord = ivec2(gl_GlobalInvocationID.xy);
    
    if (coord.x >= int(params.screenWidth) || 
        coord.y >= int(params.screenHeight)) {
        return;
    }
    
    // Edge detection
    vec2 grad = sobel(coord);
    float magnitude = length(grad);
    float edgeVal = magnitude > params.edgeThreshold ? 1.0 : 0.0;
    
    imageStore(outputEdges, coord, vec4(edgeVal, 0, 0, 1));
    
    // UI element detection based on color uniformity and size
    vec4 centerColor = imageLoad(inputScreen, coord);
    float variance = 0.0;
    
    for (int i = -2; i <= 2; i++) {
        for (int j = -2; j <= 2; j++) {
            vec4 neighbor = imageLoad(inputScreen, coord + ivec2(i, j));
            variance += length(neighbor.rgb - centerColor.rgb);
        }
    }
    
    float isElement = variance < 0.5 && edgeVal > 0.0 ? 1.0 : 0.0;
    imageStore(elementMask, coord, vec4(isElement, 0, 0, 1));
}
```

```cpp
// File: src/vision/vulkan_compute.cpp
// Vulkan compute pipeline setup and execution

#include <vulkan/vulkan.hpp>

namespace openclaw {
namespace vision {

class VulkanComputePipeline {
private:
    vk::Device device_;
    vk::Queue computeQueue_;
    vk::CommandPool commandPool_;
    vk::DescriptorPool descriptorPool_;
    
    vk::Pipeline computePipeline_;
    vk::PipelineLayout pipelineLayout_;
    vk::DescriptorSetLayout descriptorSetLayout_;
    
    // Shader module
    vk::ShaderModule computeShader_;
    
public:
    void Initialize(vk::PhysicalDevice physicalDevice) {
        // Create device with compute queue
        float queuePriority = 1.0f;
        vk::DeviceQueueCreateInfo queueCreateInfo(
            {}, 0, 1, &queuePriority
        );
        
        vk::DeviceCreateInfo deviceCreateInfo({}, queueCreateInfo);
        device_ = physicalDevice.createDevice(deviceCreateInfo);
        computeQueue_ = device_.getQueue(0, 0);
        
        // Create command pool
        commandPool_ = device_.createCommandPool(
            {vk::CommandPoolCreateFlagBits::eResetCommandBuffer}
        );
        
        // Load and compile compute shader
        computeShader_ = LoadShaderModule("screen_analysis.comp.spv");
        
        // Create descriptor set layout
        std::array<vk::DescriptorSetLayoutBinding, 4> bindings = {
            vk::DescriptorSetLayoutBinding(
                0, vk::DescriptorType::eStorageImage, 
                1, vk::ShaderStageFlagBits::eCompute
            ),
            vk::DescriptorSetLayoutBinding(
                1, vk::DescriptorType::eStorageImage,
                1, vk::ShaderStageFlagBits::eCompute
            ),
            vk::DescriptorSetLayoutBinding(
                2, vk::DescriptorType::eStorageImage,
                1, vk::ShaderStageFlagBits::eCompute
            ),
            vk::DescriptorSetLayoutBinding(
                3, vk::DescriptorType::eUniformBuffer,
                1, vk::ShaderStageFlagBits::eCompute
            )
        };
        
        descriptorSetLayout_ = device_.createDescriptorSetLayout(
            {{}, (uint32_t)bindings.size(), bindings.data()}
        );
        
        // Create pipeline layout and pipeline
        pipelineLayout_ = device_.createPipelineLayout(
            {{}, descriptorSetLayout_}
        );
        
        vk::ComputePipelineCreateInfo pipelineCreateInfo(
            {},
            vk::PipelineShaderStageCreateInfo(
                {}, vk::ShaderStageFlagBits::eCompute, 
                computeShader_, "main"
            ),
            pipelineLayout_
        );
        
        auto result = device_.createComputePipeline(
            nullptr, pipelineCreateInfo
        );
        computePipeline_ = result.value;
    }
    
    void ExecuteAnalysis(const GPUBuffer& inputScreen,
                        GPUBuffer& outputEdges,
                        GPUBuffer& elementMask,
                        uint32_t width, uint32_t height) {
        // Allocate command buffer
        vk::CommandBufferAllocateInfo allocInfo(
            commandPool_, vk::CommandBufferLevel::ePrimary, 1
        );
        auto cmdBuffers = device_.allocateCommandBuffers(allocInfo);
        vk::CommandBuffer cmd = cmdBuffers[0];
        
        // Record commands
        cmd.begin({vk::CommandBufferUsageFlagBits::eOneTimeSubmit});
        
        cmd.bindPipeline(vk::PipelineBindPoint::eCompute, computePipeline_);
        
        // Update descriptor sets and bind
        // ... (descriptor set update code)
        
        // Dispatch compute shader
        cmd.dispatch(
            (width + 15) / 16,  // Work groups X
            (height + 15) / 16, // Work groups Y
            1
        );
        
        cmd.end();
        
        // Submit to queue
        vk::SubmitInfo submitInfo({}, {}, cmd);
        computeQueue_.submit(submitInfo);
        computeQueue_.waitIdle();
    }
};

} // namespace vision
} // namespace openclaw
```

### 4.2 MoltenVK for macOS GPU Compute

On macOS, MoltenVK translates Vulkan to Metal, enabling the same compute shaders to run on Apple Silicon:

```bash
# Build OpenClaw vision module with MoltenVK on macOS
export VULKAN_SDK=/path/to/vulkansdk/macOS
cmake -DCMAKE_BUILD_TYPE=Release \
      -DVULKAN_BACKEND=MoltenVK \
      -DENABLE_METAL_COMPUTE=ON \
      ../src/vision
make -j$(sysctl -n hw.ncpu)
```

---

## 5. CUDA & RTX GPU Compute for Real-Time Inference

### 5.1 TensorRT Optimization Pipeline

**Source Reference:** NVIDIA TensorRT documentation and sample code

```cpp
// File: src/inference/tensorrt_engine.cpp
// TensorRT inference engine for real-time screen understanding

#include <NvInfer.h>
#include <NvOnnxParser.h>
#include <cuda_runtime_api.h>

namespace openclaw {
namespace inference {

using namespace nvinfer1;

class TensorRTEngine {
private:
    IRuntime* runtime_ = nullptr;
    ICudaEngine* engine_ = nullptr;
    IExecutionContext* context_ = nullptr;
    
    // CUDA streams for async inference
    cudaStream_t inferenceStream_;
    
    // GPU buffers
    void* inputBuffer_ = nullptr;
    void* outputBuffer_ = nullptr;
    std::vector<void*> bindings_;
    
    // TensorRT logger
    class Logger : public ILogger {
    public:
        void log(Severity severity, const char* msg) noexcept override {
            if (severity <= Severity::kWARNING) {
                LOG_INFO("[TensorRT] {}", msg);
            }
        }
    };
    static Logger logger_;
    
public:
    bool BuildEngine(const std::string& onnxModelPath,
                    const BuildConfig& config) {
        // Create builder
        IBuilder* builder = createInferBuilder(logger_);
        
        // Parse ONNX model
        const auto explicitBatch = 1U << static_cast<uint32_t>(
            NetworkDefinitionCreationFlag::kEXPLICIT_BATCH
        );
        INetworkDefinition* network = builder->createNetworkV2(explicitBatch);
        nvonnxparser::IParser* parser = nvonnxparser::createParser(
            *network, logger_
        );
        
        if (!parser->parseFromFile(onnxModelPath.c_str(), 
                                   static_cast<int>(ILogger::Severity::kWARNING))) {
            LOG_ERROR("Failed to parse ONNX model");
            return false;
        }
        
        // Configure builder for RTX optimization
        IBuilderConfig* buildConfig = builder->createBuilderConfig();
        
        // Enable FP16 for 2x throughput (all RTX GPUs support this)
        buildConfig->setFlag(BuilderFlag::kFP16);
        
        // Enable INT8 for 4x throughput (RTX 20-series+)
        if (config.enableInt8) {
            buildConfig->setFlag(BuilderFlag::kINT8);
            // Set INT8 calibration
            // ...
        }
        
        // Enable DLA on supported hardware (Jetson/RTX)
        if (builder->getNbDLACores() > 0 && config.useDLA) {
            buildConfig->setDefaultDeviceType(DeviceType::kDLA);
            buildConfig->setDLACore(config.dlaCore);
        }
        
        // Set max workspace size (use most of VRAM)
        buildConfig->setMemoryPoolLimit(
            MemoryPoolType::kWORKSPACE, 
            config.maxWorkspaceSize
        );
        
        // Enable CUDA Graphs for reduced launch overhead
        buildConfig->setFlag(BuilderFlag::kCUDA_GRAPH);
        
        // Build serialized engine
        IHostMemory* serializedEngine = builder->buildSerializedNetwork(
            *network, *buildConfig
        );
        
        if (!serializedEngine) {
            LOG_ERROR("Failed to build engine");
            return false;
        }
        
        // Save engine to disk for fast loading
        SaveEngine(serializedEngine->data(), serializedEngine->size(), 
                  config.engineCachePath);
        
        // Cleanup
        parser->destroy();
        network->destroy();
        buildConfig->destroy();
        builder->destroy();
        
        return LoadEngine(serializedEngine->data(), serializedEngine->size());
    }
    
    bool LoadEngine(const void* data, size_t size) {
        runtime_ = createInferRuntime(logger_);
        engine_ = runtime_->deserializeCudaEngine(data, size);
        context_ = engine_->createExecutionContext();
        
        // Allocate GPU buffers
        AllocateBuffers();
        
        // Create CUDA stream
        cudaStreamCreate(&inferenceStream_);
        
        return true;
    }
    
    // Run inference with zero-copy if input is already on GPU
    bool InferAsync(const void* gpuInput, void* gpuOutput) {
        bindings_[0] = const_cast<void*>(gpuInput);
        bindings_[1] = gpuOutput;
        
        return context_->enqueueV3(inferenceStream_);
    }
    
    // Synchronize and get results
    void Synchronize() {
        cudaStreamSynchronize(inferenceStream_);
    }
    
private:
    void AllocateBuffers() {
        for (int i = 0; i < engine_->getNbIOTensors(); i++) {
            const char* name = engine_->getIOTensorName(i);
            auto dims = engine_->getTensorShape(name);
            size_t size = GetTensorSize(dims) * sizeof(float);
            
            void* buffer;
            cudaMallocAsync(&buffer, size, inferenceStream_);
            bindings_.push_back(buffer);
        }
    }
    
    size_t GetTensorSize(const Dims& dims) {
        size_t size = 1;
        for (int i = 0; i < dims.nbDims; i++) {
            size *= dims.d[i];
        }
        return size;
    }
};

} // namespace inference
} // namespace openclaw
```

### 5.2 NVIDIA Maxine SDK Integration for Video Enhancement

**Source Reference:** `github.com/NVIDIA-Maxine/VFX-SDK-Samples`

```cpp
// File: src/recording/maxine_enhancer.cpp
// NVIDIA Maxine video effects for enhanced screen recording quality

#include "NvVFX.h"
#include "NvCVImage.h"

namespace openclaw {
namespace recording {

class MaxineVideoEnhancer {
private:
    NvVFX_Handle effect_ = nullptr;
    NvCVImage srcImage_;
    NvCVImage dstImage_;
    CUstream cudaStream_;
    
public:
    enum class EffectType {
        SUPER_RESOLUTION,
        BACKGROUND_REMOVAL,
        DENOISING,
        UPSCALE
    };
    
    bool Initialize(EffectType type, int width, int height) {
        // Create effect
        NvVFX_CreateEffect(GetEffectSelector(type), &effect_);
        
        // Set CUDA stream
        cudaStreamCreate(&cudaStream_);
        NvVFX_CudaStream effect;
        effect.stream = cudaStream_;
        NvVFX_SetObject(effect_, NVVFX_CUDA_STREAM, &effect);
        
        // Set model directory
        NvVFX_SetString(effect_, NVVFX_MODEL_DIRECTORY, 
                       "./models/maxine");
        
        // Allocate input/output images
        NvCVImage_Alloc(&srcImage_, width, height, NVCV_BGR, 
                       NVCV_F32, NVCV_PLANAR, NVCV_GPU, 1);
        NvCVImage_Alloc(&dstImage_, width * 2, height * 2, NVCV_BGR,
                       NVCV_F32, NVCV_PLANAR, NVCV_GPU, 1);
        
        // Set source image
        NvVFX_SetImage(effect_, NVVFX_INPUT_IMAGE, &srcImage_);
        NvVFX_SetImage(effect_, NVVFX_OUTPUT_IMAGE, &dstImage_);
        
        // Load model and allocate resources
        NvVFX_Load(effect_);
        
        return true;
    }
    
    // Enhance captured frame in real-time
    bool EnhanceFrame(const void* gpuInput, void* gpuOutput, 
                     int srcWidth, int srcHeight) {
        // Upload to srcImage (assuming gpuInput is CUDA device pointer)
        srcImage_.pixels = const_cast<void*>(gpuInput);
        
        // Run effect
        NvVFX_Run(effect_, 0);
        
        // Copy result to output
        cudaMemcpyAsync(gpuOutput, dstImage_.pixels,
                       dstImage_.pixelBytes * dstImage_.width * dstImage_.height,
                       cudaMemcpyDeviceToDevice, cudaStream_);
        
        return true;
    }
    
private:
    const char* GetEffectSelector(EffectType type) {
        switch (type) {
            case EffectType::SUPER_RESOLUTION: 
                return NVVFX_FX_SUPER_RES;
            case EffectType::BACKGROUND_REMOVAL: 
                return NVVFX_FX_GREEN_SCREEN;
            case EffectType::DENOISING: 
                return NVVFX_FX_DENOISING;
            case EffectType::UPSCALE: 
                return NVVFX_FX_UPSCALE;
            default: return "";
        }
    }
};

} // namespace recording
} // namespace openclaw
```

---

## 6. Low-Level OS-Specific Technologies

### 6.1 Windows: DXGI Desktop Duplication API

**Source Reference:** Microsoft DXGI documentation, OBS Studio `libobs-winrt` and `dc-capture`

```cpp
// File: src/capture/windows/dxgi_duplicator.cpp
// Hardware-accelerated Windows screen capture via DXGI

#include <d3d11.h>
#include <dxgi1_2.h>

namespace openclaw {
namespace capture {

class DXGIDesktopDuplicator {
private:
    ID3D11Device* d3dDevice_ = nullptr;
    ID3D11DeviceContext* d3dContext_ = nullptr;
    IDXGIOutputDuplication* duplication_ = nullptr;
    
    // Textures for GPU readback
    ID3D11Texture2D* stagingTexture_ = nullptr;
    ID3D11Texture2D* sharedTexture_ = nullptr;
    
    HANDLE sharedHandle_ = nullptr;
    
    DXGI_OUTPUT_DESC outputDesc_;
    D3D11_TEXTURE2D_DESC textureDesc_;
    
public:
    struct CaptureResult {
        bool success;
        ID3D11Texture2D* texture; // GPU texture - zero copy
        DXGI_OUTDUPL_FRAME_INFO frameInfo;
        uint64_t timestamp;
    };
    
    bool Initialize(int adapterIndex, int outputIndex) {
        // Create D3D11 device
        D3D_FEATURE_LEVEL featureLevel;
        HRESULT hr = D3D11CreateDevice(
            nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr,
            D3D11_CREATE_DEVICE_BGRA_SUPPORT | D3D11_CREATE_DEVICE_VIDEO_SUPPORT,
            nullptr, 0, D3D11_SDK_VERSION,
            &d3dDevice_, &featureLevel, &d3dContext_
        );
        
        if (FAILED(hr)) return false;
        
        // Get DXGI output
        IDXGIDevice* dxgiDevice;
        d3dDevice_->QueryInterface(__uuidof(IDXGIDevice), 
                                   (void**)&dxgiDevice);
        
        IDXGIAdapter* adapter;
        dxgiDevice->GetParent(__uuidof(IDXGIAdapter), (void**)&adapter);
        
        IDXGIOutput* output;
        adapter->EnumOutputs(outputIndex, &output);
        
        IDXGIOutput1* output1;
        output->QueryInterface(__uuidof(IDXGIOutput1), (void**)&output1);
        
        output->GetDesc(&outputDesc_);
        
        // Create desktop duplication
        hr = output1->DuplicateOutput(d3dDevice_, &duplication_);
        if (FAILED(hr)) return false;
        
        // Get texture description for the desktop
        DXGI_OUTDUPL_DESC duplDesc;
        duplication_->GetDesc(&duplDesc);
        
        textureDesc_.Width = duplDesc.ModeDesc.Width;
        textureDesc_.Height = duplDesc.ModeDesc.Height;
        textureDesc_.Format = duplDesc.ModeDesc.Format;
        textureDesc_.ArraySize = 1;
        textureDesc_.BindFlags = D3D11_BIND_SHADER_RESOURCE;
        textureDesc_.MiscFlags = D3D11_RESOURCE_MISC_SHARED;
        textureDesc_.SampleDesc.Count = 1;
        textureDesc_.MipLevels = 1;
        
        // Create shared texture for zero-copy sharing with CUDA/Vulkan
        d3dDevice_->CreateTexture2D(&textureDesc_, nullptr, &sharedTexture_);
        
        // Get shared handle for cross-API sharing
        IDXGIResource* dxgiResource;
        sharedTexture_->QueryInterface(__uuidof(IDXGIResource), 
                                       (void**)&dxgiResource);
        dxgiResource->GetSharedHandle(&sharedHandle_);
        dxgiResource->Release();
        
        // Create staging texture for CPU readback (if needed)
        D3D11_TEXTURE2D_DESC stagingDesc = textureDesc_;
        stagingDesc.BindFlags = 0;
        stagingDesc.Usage = D3D11_USAGE_STAGING;
        stagingDesc.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
        stagingDesc.MiscFlags = 0;
        d3dDevice_->CreateTexture2D(&stagingDesc, nullptr, &stagingTexture_);
        
        return true;
    }
    
    CaptureResult CaptureFrame() {
        CaptureResult result = {};
        
        IDXGIResource* desktopResource = nullptr;
        DXGI_OUTDUPL_FRAME_INFO frameInfo;
        
        // Acquire next frame (blocks until new frame available)
        HRESULT hr = duplication_->AcquireNextFrame(
            100, // 100ms timeout
            &frameInfo, 
            &desktopResource
        );
        
        if (hr == DXGI_ERROR_WAIT_TIMEOUT) {
            result.success = false;
            return result;
        }
        
        if (FAILED(hr)) {
            result.success = false;
            return result;
        }
        
        // Get texture from resource
        ID3D11Texture2D* desktopTexture;
        desktopResource->QueryInterface(__uuidof(ID3D11Texture2D), 
                                        (void**)&desktopTexture);
        
        // Copy to shared texture (still on GPU - zero CPU copy)
        d3dContext_->CopyResource(sharedTexture_, desktopTexture);
        
        desktopTexture->Release();
        desktopResource->Release();
        duplication_->ReleaseFrame();
        
        result.success = true;
        result.texture = sharedTexture_;
        result.frameInfo = frameInfo;
        result.timestamp = frameInfo.LastPresentTime.QuadPart;
        
        return result;
    }
    
    // Share with CUDA via shared handle
    cudaGraphicsResource* MapToCUDA() {
        cudaGraphicsResource* cudaResource;
        cudaGraphicsD3D11RegisterResource(&cudaResource, sharedTexture_,
                                         cudaGraphicsRegisterFlagsNone);
        return cudaResource;
    }
    
    // Share with Vulkan via shared handle
    HANDLE GetSharedHandle() const { return sharedHandle_; }
};

} // namespace capture
} // namespace openclaw
```

### 6.2 Linux: KMS/DRM + DMA-BUF Zero-Copy Capture

**Source Reference:** `github.com/w23/obs-kmsgrab`, Weston compositor code

```c
// File: src/capture/linux/kms_capture.c
// Zero-copy Linux screen capture via KMS/DRM and DMA-BUF

#include <xf86drm.h>
#include <xf86drmMode.h>
#include <gbm.h>
#include <EGL/egl.h>
#include <EGL/eglext.h>
#include <GLES2/gl2.h>
#include <GLES2/gl2ext.h>
#include <libdrm/drm_fourcc.h>

// DMA-BUF EGL extension
#ifndef EGL_EGLEXT_PROTOTYPES
#define EGL_EGLEXT_PROTOTYPES
#endif

// PFNEGLCREATEIMAGEKHRPROC, etc.

namespace openclaw {
namespace capture {

class KMSCapture {
private:
    int drmFd_ = -1;
    uint32_t crtcId_ = 0;
    uint32_t connectorId_ = 0;
    
    struct gbm_device* gbmDevice_ = nullptr;
    EGLDisplay eglDisplay_ = EGL_NO_DISPLAY;
    EGLContext eglContext_ = EGL_NO_CONTEXT;
    
    // DMA-BUF import extension
    PFNEGLCREATEIMAGEKHRPROC eglCreateImageKHR_ = nullptr;
    PFNEGLDESTROYIMAGEKHRPROC eglDestroyImageKHR_ = nullptr;
    PFNGLEGLIMAGETARGETTEXTURE2DOESPROC glEGLImageTargetTexture2DOES_ = nullptr;
    PFNEGLCREATEDMABUFIMAGEEXTPROC eglCreateDmaBufImageEXT_ = nullptr;
    
public:
    bool Initialize(const char* drmDevice = "/dev/dri/card0") {
        // Open DRM device
        drmFd_ = open(drmDevice, O_RDWR | O_CLOEXEC);
        if (drmFd_ < 0) return false;
        
        // Create GBM device
        gbmDevice_ = gbm_create_device(drmFd_);
        if (!gbmDevice_) return false;
        
        // Create EGL display from GBM
        eglDisplay_ = eglGetPlatformDisplayEXT(
            EGL_PLATFORM_GBM_MESA, gbmDevice_, nullptr
        );
        
        eglInitialize(eglDisplay_, nullptr, nullptr);
        
        // Choose EGL config
        static const EGLint configAttribs[] = {
            EGL_SURFACE_TYPE, EGL_PBUFFER_BIT,
            EGL_RENDERABLE_TYPE, EGL_OPENGL_ES2_BIT,
            EGL_NONE
        };
        
        EGLConfig config;
        EGLint numConfigs;
        eglChooseConfig(eglDisplay_, configAttribs, &config, 1, &numConfigs);
        
        // Create context
        static const EGLint contextAttribs[] = {
            EGL_CONTEXT_CLIENT_VERSION, 2,
            EGL_NONE
        };
        eglContext_ = eglCreateContext(eglDisplay_, config, 
                                       EGL_NO_CONTEXT, contextAttribs);
        eglMakeCurrent(eglDisplay_, EGL_NO_SURFACE, EGL_NO_SURFACE, 
                      eglContext_);
        
        // Load DMA-BUF extensions
        eglCreateImageKHR_ = (PFNEGLCREATEIMAGEKHRPROC)
            eglGetProcAddress("eglCreateImageKHR");
        eglDestroyImageKHR_ = (PFNEGLDESTROYIMAGEKHRPROC)
            eglGetProcAddress("eglDestroyImageKHR");
        glEGLImageTargetTexture2DOES_ = 
            (PFNGLEGLIMAGETARGETTEXTURE2DOESPROC)
            eglGetProcAddress("glEGLImageTargetTexture2DOES");
        eglCreateDmaBufImageEXT_ = (PFNEGLCREATEDMABUFIMAGEEXTPROC)
            eglGetProcAddress("eglCreateDmaBufImageEXT");
        
        // Find active CRTC and connector
        drmModeResPtr resources = drmModeGetResources(drmFd_);
        for (int i = 0; i < resources->count_connectors; i++) {
            drmModeConnectorPtr connector = drmModeGetConnector(
                drmFd_, resources->connectors[i]
            );
            if (connector->connection == DRM_MODE_CONNECTED) {
                connectorId_ = connector->connector_id;
                crtcId_ = connector->encoder_id; // simplified
                drmModeFreeConnector(connector);
                break;
            }
            drmModeFreeConnector(connector);
        }
        drmModeFreeResources(resources);
        
        return true;
    }
    
    // Capture framebuffer as DMA-BUF (zero-copy)
    DmaBufCapture CaptureDmaBuf() {
        DmaBufCapture result = {};
        
        // Get current framebuffer info from KMS
        drmModeCrtcPtr crtc = drmModeGetCrtc(drmFd_, crtcId_);
        if (!crtc) return result;
        
        uint32_t fbId = crtc->buffer_id;
        drmModeFreeCrtc(crtc);
        
        // Get framebuffer info
        drmModeFB2Ptr fb = drmModeGetFB2(drmFd_, fbId);
        if (!fb) return result;
        
        // Export DMA-BUF fds from framebuffer
        for (int i = 0; i < 4; i++) {
            if (fb->handles[i]) {
                // Prime handle to dma-buf fd
                drmPrimeHandleToFD(drmFd_, fb->handles[i], 
                                  DRM_CLOEXEC | DRM_RDWR, &result.fds[i]);
                result.pitches[i] = fb->pitches[i];
                result.offsets[i] = fb->offsets[i];
                result.modifier = fb->modifier;
            }
        }
        
        result.width = fb->width;
        result.height = fb->height;
        result.fourcc = fb->pixel_format;
        result.numPlanes = fb->num_planes;
        
        drmModeFreeFB2(fb);
        
        // Import as EGLImage
        EGLint attribs[] = {
            EGL_WIDTH, (EGLint)result.width,
            EGL_HEIGHT, (EGLint)result.height,
            EGL_LINUX_DRM_FOURCC_EXT, (EGLint)result.fourcc,
            EGL_DMA_BUF_PLANE0_FD_EXT, result.fds[0],
            EGL_DMA_BUF_PLANE0_OFFSET_EXT, (EGLint)result.offsets[0],
            EGL_DMA_BUF_PLANE0_PITCH_EXT, (EGLint)result.pitches[0],
            EGL_NONE
        };
        
        EGLImageKHR image = eglCreateImageKHR_(
            eglDisplay_, EGL_NO_CONTEXT, EGL_LINUX_DMA_BUF_EXT,
            nullptr, attribs
        );
        
        // Create texture from EGLImage (still on GPU)
        GLuint texture;
        glGenTextures(1, &texture);
        glBindTexture(GL_TEXTURE_2D, texture);
        glEGLImageTargetTexture2DOES_(GL_TEXTURE_2D, image);
        
        result.textureId = texture;
        result.eglImage = image;
        result.success = true;
        
        return result;
    }
    
    // Share DMA-BUF with Vulkan via FD
    int GetDmaBufFD() const {
        // Return the primary plane fd for Vulkan import
        return currentCapture_.fds[0];
    }
};

} // namespace capture
} // namespace openclaw
```

### 6.3 macOS: ScreenCaptureKit + AX Accessibility

**Source Reference:** Fazm macOS AI (`fazm.ai`), Apple ScreenCaptureKit docs

```objc
// File: src/capture/macos/ScreenCaptureKitBridge.m
// macOS screen capture using ScreenCaptureKit (macOS 12.3+)

#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <CoreVideo/CoreVideo.h>

API_AVAILABLE(macos(12.3))
@interface OpenClawCaptureDelegate : NSObject <SCStreamOutput>
@property (nonatomic, copy) void (^frameHandler)(CVPixelBufferRef, uint64_t);
@end

@implementation OpenClawCaptureDelegate

- (void)stream:(SCStream *)stream 
    didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer 
    ofType:(SCStreamOutputType)type {
    
    if (type != SCStreamOutputTypeScreen) return;
    
    CVPixelBufferRef pixelBuffer = CMSampleBufferGetImageBuffer(sampleBuffer);
    if (!pixelBuffer) return;
    
    // Lock pixel buffer - on Apple Silicon this is often IOSurface-backed
    CVPixelBufferLockBaseAddress(pixelBuffer, kCVPixelBufferLock_ReadOnly);
    
    // Get IOSurface for zero-copy sharing with Metal
    IOSurfaceRef ioSurface = CVPixelBufferGetIOSurface(pixelBuffer);
    uint64_t timestamp = CMSampleBufferGetPresentationTimeStamp(sampleBuffer).value;
    
    if (self.frameHandler) {
        self.frameHandler(pixelBuffer, timestamp);
    }
    
    CVPixelBufferUnlockBaseAddress(pixelBuffer, kCVPixelBufferLock_ReadOnly);
}

@end

// Accessibility API integration
@interface OpenClawAccessibility : NSObject
- (NSArray *)getUIElementsForApp:(NSString *)bundleIdentifier;
- (BOOL)clickElement:(AXUIElementRef)element;
- (BOOL)setValue:(NSString *)value forElement:(AXUIElementRef)element;
@end

@implementation OpenClawAccessibility

- (NSArray *)getUIElementsForApp:(NSString *)bundleIdentifier {
    // Use AX API to get full UI tree
    AXUIElementRef systemWide = AXUIElementCreateSystemWide();
    AXUIElementRef frontApp = NULL;
    AXUIElementCopyAttributeValue(systemWide, kAXFrontmostApplicationAttribute,
                                  (CFTypeRef*)&frontApp);
    
    // Get focused window
    AXUIElementRef focusedWindow = NULL;
    AXUIElementCopyAttributeValue(frontApp, kAXFocusedWindowAttribute,
                                  (CFTypeRef*)&focusedWindow);
    
    // Get all children recursively
    NSMutableArray *elements = [NSMutableArray array];
    [self enumerateElements:focusedWindow array:elements depth:0];
    
    CFRelease(focusedWindow);
    CFRelease(frontApp);
    CFRelease(systemWide);
    
    return elements;
}

- (void)enumerateElements:(AXUIElementRef)element 
                   array:(NSMutableArray *)array 
                   depth:(int)depth {
    if (depth > 20) return; // Prevent infinite recursion
    
    // Get element attributes
    CFStringRef title = NULL;
    AXUIElementCopyAttributeValue(element, kAXTitleAttribute, (CFTypeRef*)&title);
    
    CFStringRef role = NULL;
    AXUIElementCopyAttributeValue(element, kAXRoleAttribute, (CFTypeRef*)&role);
    
    CFTypeRef position = NULL;
    AXUIElementCopyAttributeValue(element, kAXPositionAttribute, &position);
    
    CFTypeRef size = NULL;
    AXUIElementCopyAttributeValue(element, kAXSizeAttribute, &size);
    
    if (title || role) {
        [array addObject:@{
            @"title": (__bridge_transfer NSString*)title ?: @"",
            @"role": (__bridge_transfer NSString*)role ?: @"",
            @"position": [NSValue valueWithPoint:AXValueGetPoint(position)],
            @"size": [NSValue valueWithSize:AXValueGetSize(size)]
        }];
    }
    
    // Get children
    CFArrayRef children = NULL;
    AXError error = AXUIElementCopyAttributeValue(
        element, kAXChildrenAttribute, (CFTypeRef*)&children
    );
    
    if (error == kAXErrorSuccess && children) {
        for (NSInteger i = 0; i < CFArrayGetCount(children); i++) {
            AXUIElementRef child = CFArrayGetValueAtIndex(children, i);
            [self enumerateElements:child array:array depth:depth + 1];
        }
        CFRelease(children);
    }
}

- (BOOL)clickElement:(AXUIElementRef)element {
    AXUIElementPerformAction(element, kAXPressAction);
    return YES;
}

@end
```

---

## 7. Real-Time Screen Capture Architectures

### 7.1 Unified Capture Abstraction

```cpp
// File: src/capture/capture_engine.hpp
// Unified capture engine supporting all platforms and methods

namespace openclaw {
namespace capture {

enum class CaptureMethod {
    // Windows
    DXGI_DUPLICATION,      // DXGI Desktop Duplication (Win8+)
    WINDOWS_GRAPHICS_CAPTURE, // WGC (Win10 1803+)
    GDI_BITBLT,            // Legacy GDI
    MAGNIFICATION_API,     // For occluded windows
    NVFBC,                 // NVIDIA Framebuffer Capture
    
    // Linux
    KMS_DMABUF,            // KMS/DRM zero-copy
    PIPEWIRE,              // Wayland/X11 portal
    X11_SHM,               // X11 shared memory
    NVFBC_LINUX,           // NVIDIA Linux capture
    
    // macOS
    SCREENCAPTUREKIT,      // macOS 12.3+
    CGDISPLAYSTREAM,       // Legacy CoreGraphics
    
    // Mobile
    ADB_SCREENRECORD,      // Android screenrecord
    SCRCPY,                // Android scrcpy
    IOS_QUICKTIME,         // iOS via QuickTime
    
    // Generic
    V4L2_LOOPBACK,         // Virtual camera
    RTSP_STREAM            // Network stream
};

enum class GPUShareMode {
    NONE,                  // CPU readback required
    CUDA_SHARE,           // Share with CUDA via CUarray
    VULKAN_SHARE,         // Share with Vulkan via external memory
    METAL_SHARE,          // Share with Metal via IOSurface
    EGL_DMABUF            // Share via EGL DMA-BUF
};

struct CaptureConfig {
    CaptureMethod method;
    int displayIndex = 0;
    int width = 0;         // 0 = native
    int height = 0;
    int fps = 60;
    bool captureCursor = true;
    bool hdr = false;
    GPUShareMode shareMode = GPUShareMode::NONE;
};

class ICaptureEngine {
public:
    virtual ~ICaptureEngine() = default;
    
    virtual bool Initialize(const CaptureConfig& config) = 0;
    virtual bool Start() = 0;
    virtual void Stop() = 0;
    
    // Get frame as GPU texture (zero-copy)
    virtual GPUFrame AcquireGPUFrame() = 0;
    
    // Get frame as CPU buffer (with copy)
    virtual CPUFrame AcquireCPUFrame() = 0;
    
    // Get frame info without acquiring
    virtual FrameInfo PeekFrame() = 0;
    
    // Register callback for new frames
    virtual void SetFrameCallback(FrameCallback callback) = 0;
    
    // Cross-API sharing
    virtual void* GetSharedHandle() = 0;
    virtual cudaGraphicsResource* MapToCUDA() = 0;
};

// Factory
std::unique_ptr<ICaptureEngine> CreateCaptureEngine(
    const CaptureConfig& config
);

} // namespace capture
} // namespace openclaw
```

### 7.2 Platform-Specific Implementations

| Platform | Primary Method | Fallback | GPU Sharing |
|----------|---------------|----------|-------------|
| Windows (NVIDIA) | NVFBC | DXGI | CUDA interop |
| Windows (AMD/Intel) | DXGI Desktop Duplication | WGC | Vulkan external memory |
| Linux (NVIDIA) | NVFBC + EGL | KMS DMA-BUF | CUDA-EGL interop |
| Linux (AMD/Intel) | KMS DMA-BUF | PipeWire | Vulkan DMA-BUF |
| macOS | ScreenCaptureKit | CGDisplayStream | IOSurface |
| Android | scrcpy (ADB) | screenrecord | N/A |
| iOS | QuickTime USB | WebDriverAgent | N/A |

---

## 8. High-Performance Recording & Streaming Pipelines

### 8.1 OBS-Style Recording Architecture

**Source Reference:** OBS Studio Backend Design (`docs.obsproject.com/backend-design`)

```cpp
// File: src/recording/libobs_pipeline.cpp
// Recording pipeline inspired by OBS libobs architecture

namespace openclaw {
namespace recording {

// OBS-style source object
class RecordingSource {
public:
    virtual ~RecordingSource() = default;
    
    // Called by graphics thread
    virtual void VideoRender(gs_texture_t* target) = 0;
    virtual void AudioRender(float** outputData, uint32_t numSamples) = 0;
    
    // OBS-style tick/render cycle
    virtual void Tick(float seconds) = 0;
};

// Recording output (muxed file or stream)
class RecordingOutput {
public:
    virtual ~RecordingOutput() = default;
    
    virtual bool Start(const OutputConfig& config) = 0;
    virtual void Stop() = 0;
    
    // Receive encoded packets
    virtual void ReceiveVideoPacket(EncoderPacket* packet) = 0;
    virtual void ReceiveAudioPacket(EncoderPacket* packet) = 0;
};

// Hardware encoder wrapper
class HWVideoEncoder {
public:
    // NVENC (NVIDIA)
    static std::unique_ptr<HWVideoEncoder> CreateNVENC(
        const VideoConfig& config,
        void* d3dDevice = nullptr // For D3D11 sharing
    );
    
    // VAAPI (Intel/AMD Linux)
    static std::unique_ptr<HWVideoEncoder> CreateVAAPI(
        const VideoConfig& config,
        int drmFd = -1
    );
    
    // AMF (AMD Windows)
    static std::unique_ptr<HWVideoEncoder> CreateAMF(
        const VideoConfig& config
    );
    
    // VideoToolbox (macOS)
    static std::unique_ptr<HWVideoEncoder> CreateVideoToolbox(
        const VideoConfig& config
    );
    
    virtual bool Initialize() = 0;
    virtual bool EncodeFrame(void* gpuTexture, uint64_t timestamp) = 0;
    virtual bool EncodeFrameCPU(const void* rgbaData, uint64_t timestamp) = 0;
    virtual bool GetNextPacket(EncoderPacket* packet) = 0;
};

// The recording pipeline
class RecordingPipeline {
private:
    // Three OBS-style threads:
    std::thread graphicsThread_;  // obs_graphics_thread
    std::thread videoThread_;     // video_thread (encoding)
    std::thread audioThread_;     // audio_thread
    
    std::unique_ptr<ICaptureEngine> captureEngine_;
    std::unique_ptr<HWVideoEncoder> videoEncoder_;
    std::unique_ptr<RecordingOutput> output_;
    
    // Frame queue (graphics thread -> video thread)
    ThreadQueue<VideoFrame> rawFrameQueue_;
    
    // Packet queue (video thread -> output thread)
    ThreadQueue<EncoderPacket> encodedPacketQueue_;
    
public:
    bool Initialize(const RecordingConfig& config) {
        // 1. Create capture engine
        CaptureConfig captureCfg;
        captureCfg.method = config.captureMethod;
        captureCfg.width = config.width;
        captureCfg.height = config.height;
        captureCfg.fps = config.fps;
        captureCfg.shareMode = GPUShareMode::CUDA_SHARE;
        
        captureEngine_ = CreateCaptureEngine(captureCfg);
        
        // 2. Create hardware encoder
        VideoConfig vidConfig;
        vidConfig.width = config.width;
        vidConfig.height = config.height;
        vidConfig.fps = config.fps;
        vidConfig.bitrate = config.bitrate;
        vidConfig.codec = config.codec; // H264, H265, AV1
        
        videoEncoder_ = SelectBestEncoder(vidConfig);
        
        // 3. Create output
        if (config.outputType == OutputType::FILE) {
            output_ = std::make_unique<FFmpegFileOutput>();
        } else if (config.outputType == OutputType::STREAM) {
            output_ = std::make_unique<RTMPOutput>();
        } else if (config.outputType == OutputType::WEBRTC) {
            output_ = std::make_unique<WebRTCOutput>();
        }
        
        return true;
    }
    
    void Start() {
        // Start OBS-style threads
        graphicsThread_ = std::thread(&RecordingPipeline::GraphicsLoop, this);
        videoThread_ = std::thread(&RecordingPipeline::VideoEncodingLoop, this);
        audioThread_ = std::thread(&RecordingPipeline::AudioLoop, this);
    }
    
private:
    // Graphics thread: captures and composites
    void GraphicsLoop() {
        while (running_) {
            // Acquire frame from capture engine (GPU texture)
            GPUFrame frame = captureEngine_->AcquireGPUFrame();
            
            // Add timestamp
            frame.timestamp = GetNanosecondTimestamp();
            
            // Queue for video thread
            rawFrameQueue_.Push(frame);
            
            // Throttle to target FPS
            std::this_thread::sleep_until(nextFrameTime);
        }
    }
    
    // Video encoding thread
    void VideoEncodingLoop() {
        while (running_) {
            VideoFrame frame = rawFrameQueue_.Pop();
            
            // Encode using hardware encoder
            videoEncoder_->EncodeFrame(frame.gpuTexture, frame.timestamp);
            
            // Retrieve encoded packets
            EncoderPacket packet;
            while (videoEncoder_->GetNextPacket(&packet)) {
                encodedPacketQueue_.Push(packet);
            }
        }
    }
};

// FFmpeg-based file output for maximum compatibility
class FFmpegFileOutput : public RecordingOutput {
private:
    AVFormatContext* formatContext_ = nullptr;
    AVStream* videoStream_ = nullptr;
    AVStream* audioStream_ = nullptr;
    
public:
    bool Start(const OutputConfig& config) override {
        avformat_alloc_output_context2(&formatContext_, nullptr, nullptr,
                                       config.filename.c_str());
        
        // Add video stream
        videoStream_ = avformat_new_stream(formatContext_, nullptr);
        videoStream_->codecpar->codec_id = AV_CODEC_ID_H264;
        videoStream_->codecpar->codec_type = AVMEDIA_TYPE_VIDEO;
        videoStream_->codecpar->width = config.width;
        videoStream_->codecpar->height = config.height;
        videoStream_->codecpar->format = AV_PIX_FMT_YUV420P;
        videoStream_->time_base = {1, (int)config.fps};
        
        // Open output file
        avio_open(&formatContext_->pb, config.filename.c_str(), 
                 AVIO_FLAG_WRITE);
        avformat_write_header(formatContext_, nullptr);
        
        return true;
    }
    
    void ReceiveVideoPacket(EncoderPacket* packet) override {
        AVPacket pkt;
        av_new_packet(&pkt, packet->size);
        memcpy(pkt.data, packet->data, packet->size);
        
        pkt.pts = packet->pts;
        pkt.dts = packet->dts;
        pkt.stream_index = videoStream_->index;
        
        if (packet->keyframe) {
            pkt.flags |= AV_PKT_FLAG_KEY;
        }
        
        av_interleaved_write_frame(formatContext_, &pkt);
        av_packet_unref(&pkt);
    }
};

} // namespace recording
} // namespace openclaw
```

### 8.2 WebRTC Streaming for Remote Monitoring

```cpp
// File: src/recording/webrtc_output.cpp
// WebRTC output for sub-second remote viewing

#include <gst/gst.h>
#include <gst/webrtc/webrtc.h>

namespace openclaw {
namespace recording {

class WebRTCOutput : public RecordingOutput {
private:
    GstElement* pipeline_ = nullptr;
    GstElement* webrtcbin_ = nullptr;
    
    // WHIP endpoint for streaming
    std::string whipEndpoint_;
    
public:
    bool Start(const OutputConfig& config) override {
        whipEndpoint_ = config.whipUrl;
        
        // Create GStreamer pipeline with hardware encode + WebRTC
        // Pipeline: appsrc → h264parse → rtph264pay → webrtcbin
        pipeline_ = gst_parse_launch(
            "webrtcbin name=sendonly bundle-policy=max-bundle "
            "appsrc name=videosrc ! h264parse ! "
            "rtph264pay config-interval=-1 pt=96 ! "
            "application/x-rtp,media=video,encoding-name=H264,payload=96 ! "
            "sendonly.",
            nullptr
        );
        
        webrtcbin_ = gst_bin_get_by_name(GST_BIN(pipeline_), "sendonly");
        
        // Connect WHIP endpoint
        ConnectWHIP();
        
        gst_element_set_state(pipeline_, GST_STATE_PLAYING);
        
        return true;
    }
    
    void ReceiveVideoPacket(EncoderPacket* packet) override {
        // Push encoded H264 into GStreamer
        GstElement* appsrc = gst_bin_get_by_name(GST_BIN(pipeline_), "videosrc");
        
        GstBuffer* buffer = gst_buffer_new_allocate(nullptr, packet->size, nullptr);
        GstMapInfo map;
        gst_buffer_map(buffer, &map, GST_MAP_WRITE);
        memcpy(map.data, packet->data, packet->size);
        gst_buffer_unmap(buffer, &map);
        
        GST_BUFFER_PTS(buffer) = packet->pts * GST_SECOND / 90000;
        GST_BUFFER_DTS(buffer) = packet->dts * GST_SECOND / 90000;
        
        gst_app_src_push_buffer(GST_APP_SRC(appsrc), buffer);
    }
    
private:
    void ConnectWHIP() {
        // Implement WHIP protocol for ingest
        // POST SDP offer to WHIP endpoint
        // Set remote SDP answer
        // ICE connection automatically handled by webrtcbin
    }
};

} // namespace recording
} // namespace openclaw
```

---

## 9. Hook & Interception Systems

### 9.1 Linux: LD_PRELOAD for API Interception

**Source Reference:** Multiple sources on `LD_PRELOAD` hooking techniques

```c
// File: src/hooks/linux/ld_preload_interceptor.c
// LD_PRELOAD-based API interception for observing application behavior

#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <string.h>
#include <link.h>

// Original function pointers
static int (*real_connect)(int sockfd, const struct sockaddr *addr, 
                           socklen_t addrlen) = NULL;
static ssize_t (*real_send)(int sockfd, const void *buf, size_t len, 
                            int flags) = NULL;
static ssize_t (*real_recv)(int sockfd, void *buf, size_t len, 
                            int flags) = NULL;
static int (*real_open)(const char *pathname, int flags, ...) = NULL;
static int (*real_open64)(const char *pathname, int flags, ...) = NULL;
static FILE* (*real_fopen)(const char *pathname, const char *mode) = NULL;
static void (*real_XMapWindow)(Display*, Window) = NULL;
static int (*real_XRaiseWindow)(Display*, Window) = NULL;

// Hook server communication
static int hook_socket = -1;

void __attribute__((constructor)) init_hooks(void) {
    // Resolve original functions
    real_connect = dlsym(RTLD_NEXT, "connect");
    real_send = dlsym(RTLD_NEXT, "send");
    real_recv = dlsym(RTLD_NEXT, "recv");
    real_open = dlsym(RTLD_NEXT, "open");
    real_open64 = dlsym(RTLD_NEXT, "open64");
    real_fopen = dlsym(RTLD_NEXT, "fopen");
    real_XMapWindow = dlsym(RTLD_NEXT, "XMapWindow");
    real_XRaiseWindow = dlsym(RTLD_NEXT, "XRaiseWindow");
    
    // Connect to OpenClaw hook server
    const char* hook_addr = getenv("OPENCLAW_HOOK_SOCKET");
    if (hook_addr) {
        hook_socket = atoi(hook_addr);
    }
    
    fprintf(stderr, "[OpenClaw Hook] LD_PRELOAD interceptor loaded\n");
}

// Intercept network connections
int connect(int sockfd, const struct sockaddr *addr, socklen_t addrlen) {
    // Log connection attempt
    if (addr->sa_family == AF_INET) {
        struct sockaddr_in* sin = (struct sockaddr_in*)addr;
        char ip[INET_ADDRSTRLEN];
        inet_ntop(AF_INET, &sin->sin_addr, ip, sizeof(ip));
        
        notify_hook_server("NETWORK_CONNECT", ip, ntohs(sin->sin_port));
    }
    
    return real_connect(sockfd, addr, addrlen);
}

// Intercept file opens
int open(const char *pathname, int flags, ...) {
    mode_t mode = 0;
    if (flags & O_CREAT) {
        va_list ap;
        va_start(ap, flags);
        mode = va_arg(ap, mode_t);
        va_end(ap);
    }
    
    notify_hook_server("FILE_OPEN", pathname, flags);
    
    if (real_open) {
        return real_open(pathname, flags, mode);
    }
    return syscall(SYS_open, pathname, flags, mode);
}

// Intercept X11 window operations (track UI changes)
int XRaiseWindow(Display *display, Window w) {
    XWindowAttributes attrs;
    XGetWindowAttributes(display, w, &attrs);
    
    notify_hook_server("WINDOW_RAISE", 
                      DisplayString(display), (int)w);
    
    return real_XRaiseWindow(display, w);
}

int XMapWindow(Display *display, Window w) {
    notify_hook_server("WINDOW_MAP", 
                      DisplayString(display), (int)w);
    
    return real_XMapWindow(display, w);
}

// Notify OpenClaw hook server via Unix domain socket
static void notify_hook_server(const char* event_type, 
                               const char* detail, int value) {
    if (hook_socket < 0) return;
    
    HookEvent event = {
        .timestamp = get_nanoseconds(),
        .pid = getpid(),
        .tid = gettid(),
        .event_type = event_type,
        .detail = detail,
        .value = value
    };
    
    send(hook_socket, &event, sizeof(event), MSG_DONTWAIT);
}
```

### 9.2 Runtime PLT Hooking with plthook

**Source Reference:** `github.com/kubo/plthook`

```c
// File: src/hooks/linux/plt_hook_runtime.c
// Runtime PLT/GOT hooking without LD_PRELOAD

#include "plthook.h"

bool InstallRuntimeHooks(const char* targetLibrary) {
    plthook_t* hook;
    
    // Open target library for hooking
    if (plthook_open(&hook, targetLibrary) != 0) {
        fprintf(stderr, "plthook_open error: %s\n", plthook_error());
        return false;
    }
    
    // Hook specific functions
    void* original_connect;
    plthook_replace(hook, "connect", (void*)hooked_connect, &original_connect);
    real_connect = original_connect;
    
    void* original_send;
    plthook_replace(hook, "send", (void*)hooked_send, &original_send);
    real_send = original_send;
    
    // Also hook dynamically loaded libraries
    void* original_dlopen;
    plthook_replace(hook, "dlopen", (void*)hooked_dlopen, &original_dlopen);
    real_dlopen = original_dlopen;
    
    plthook_close(hook);
    return true;
}

// Track newly loaded libraries and hook them too
void* hooked_dlopen(const char* filename, int flags) {
    void* handle = real_dlopen(filename, flags);
    
    if (handle) {
        // New library loaded - install hooks
        char libPath[PATH_MAX];
        dlinfo(handle, RTLD_DI_ORIGIN, libPath);
        
        notify_hook_server("LIBRARY_LOAD", filename, 0);
        
        // Install hooks in newly loaded library
        InstallRuntimeHooks(filename);
    }
    
    return handle;
}
```

### 9.3 Windows: Low-Level Hooks

```cpp
// File: src/hooks/windows/ll_hooks.cpp
// Windows low-level keyboard/mouse hooks for input monitoring

#include <windows.h>

namespace openclaw {
namespace hooks {

class WindowsInputHook {
private:
    HHOOK keyboardHook_ = NULL;
    HHOOK mouseHook_ = NULL;
    
    static WindowsInputHook* instance_;
    
public:
    bool Install() {
        instance_ = this;
        
        // Install low-level keyboard hook
        keyboardHook_ = SetWindowsHookEx(
            WH_KEYBOARD_LL,
            KeyboardProc,
            GetModuleHandle(NULL),
            0
        );
        
        // Install low-level mouse hook
        mouseHook_ = SetWindowsHookEx(
            WH_MOUSE_LL,
            MouseProc,
            GetModuleHandle(NULL),
            0
        );
        
        // Message loop for hook processing
        hookThread_ = std::thread([this]() {
            MSG msg;
            while (GetMessage(&msg, NULL, 0, 0)) {
                TranslateMessage(&msg);
                DispatchMessage(&msg);
            }
        });
        
        return keyboardHook_ && mouseHook_;
    }
    
    void Uninstall() {
        if (keyboardHook_) UnhookWindowsHookEx(keyboardHook_);
        if (mouseHook_) UnhookWindowsHookEx(mouseHook_);
    }
    
private:
    static LRESULT CALLBACK KeyboardProc(int nCode, WPARAM wParam, 
                                         LPARAM lParam) {
        if (nCode >= 0) {
            KBDLLHOOKSTRUCT* pKb = (KBDLLHOOKSTRUCT*)lParam;
            
            InputEvent event;
            event.type = InputEventType::KEYBOARD;
            event.timestamp = pKb->time;
            event.vkCode = pKb->vkCode;
            event.scanCode = pKb->scanCode;
            event.flags = pKb->flags;
            event.injected = (pKb->flags & LLKHF_INJECTED) != 0;
            event.keyUp = (wParam == WM_KEYUP || wParam == WM_SYSKEYUP);
            
            instance_->OnInputEvent(event);
        }
        
        return CallNextHookEx(NULL, nCode, wParam, lParam);
    }
    
    static LRESULT CALLBACK MouseProc(int nCode, WPARAM wParam, 
                                      LPARAM lParam) {
        if (nCode >= 0) {
            MSLLHOOKSTRUCT* pMouse = (MSLLHOOKSTRUCT*)lParam;
            
            InputEvent event;
            event.type = InputEventType::MOUSE;
            event.timestamp = pMouse->time;
            event.x = pMouse->pt.x;
            event.y = pMouse->pt.y;
            event.injected = (pMouse->flags & LLMHF_INJECTED) != 0;
            
            switch (wParam) {
                case WM_LBUTTONDOWN: event.mouseButton = 0; event.mouseDown = true; break;
                case WM_LBUTTONUP: event.mouseButton = 0; event.mouseDown = false; break;
                case WM_RBUTTONDOWN: event.mouseButton = 1; event.mouseDown = true; break;
                case WM_RBUTTONUP: event.mouseButton = 1; event.mouseDown = false; break;
                case WM_MOUSEWHEEL: event.wheelDelta = GET_WHEEL_DELTA_WPARAM(pMouse->mouseData); break;
            }
            
            instance_->OnInputEvent(event);
        }
        
        return CallNextHookEx(NULL, nCode, wParam, lParam);
    }
    
    void OnInputEvent(const InputEvent& event) {
        // Forward to OpenClaw for analysis
        // Distinguish human vs. injected input
        // Track user interaction patterns
    }
};

} // namespace hooks
} // namespace openclaw
```

---

## 10. Cross-Platform Input Simulation

### 10.1 Linux: evdev/uinput

**Source Reference:** `kernel.org/doc/html/v4.12/input/uinput.html`, `python-evdev.readthedocs.io`

```cpp
// File: src/input/linux/uinput_controller.cpp
// Kernel-level input injection via Linux uinput

#include <linux/uinput.h>
#include <linux/input.h>
#include <fcntl.h>
#include <unistd.h>

namespace openclaw {
namespace input {

class UInputController {
private:
    int fd_ = -1;
    struct uinput_user_dev uidev_;
    
public:
    bool Initialize() {
        fd_ = open("/dev/uinput", O_WRONLY | O_NONBLOCK);
        if (fd_ < 0) {
            fd_ = open("/dev/input/uinput", O_WRONLY | O_NONBLOCK);
            if (fd_ < 0) return false;
        }
        
        // Enable device capabilities
        ioctl(fd_, UI_SET_EVBIT, EV_KEY);   // Keyboard keys
        ioctl(fd_, UI_SET_EVBIT, EV_REL);   // Relative movement (mouse)
        ioctl(fd_, UI_SET_EVBIT, EV_ABS);   // Absolute movement (touch)
        ioctl(fd_, UI_SET_EVBIT, EV_SYN);   // Synchronization
        
        // Enable all keyboard keys
        for (int key = KEY_ESC; key <= KEY_MICMUTE; key++) {
            ioctl(fd_, UI_SET_KEYBIT, key);
        }
        
        // Enable mouse buttons
        ioctl(fd_, UI_SET_KEYBIT, BTN_LEFT);
        ioctl(fd_, UI_SET_KEYBIT, BTN_RIGHT);
        ioctl(fd_, UI_SET_KEYBIT, BTN_MIDDLE);
        ioctl(fd_, UI_SET_KEYBIT, BTN_SIDE);
        ioctl(fd_, UI_SET_KEYBIT, BTN_EXTRA);
        
        // Enable relative axes
        ioctl(fd_, UI_SET_RELBIT, REL_X);
        ioctl(fd_, UI_SET_RELBIT, REL_Y);
        ioctl(fd_, UI_SET_RELBIT, REL_WHEEL);
        ioctl(fd_, UI_SET_RELBIT, REL_HWHEEL);
        
        // Enable absolute axes (for touch/pad)
        ioctl(fd_, UI_SET_ABSBIT, ABS_X);
        ioctl(fd_, UI_SET_ABSBIT, ABS_Y);
        ioctl(fd_, UI_SET_ABSBIT, ABS_MT_SLOT);
        ioctl(fd_, UI_SET_ABSBIT, ABS_MT_POSITION_X);
        ioctl(fd_, UI_SET_ABSBIT, ABS_MT_POSITION_Y);
        ioctl(fd_, UI_SET_ABSBIT, ABS_MT_TRACKING_ID);
        
        // Configure absolute axes
        struct uinput_abs_setup absSetup;
        memset(&absSetup, 0, sizeof(absSetup));
        absSetup.code = ABS_X;
        absSetup.absinfo.minimum = 0;
        absSetup.absinfo.maximum = 32767;
        ioctl(fd_, UI_ABS_SETUP, &absSetup);
        
        absSetup.code = ABS_Y;
        ioctl(fd_, UI_ABS_SETUP, &absSetup);
        
        // Setup device info
        memset(&uidev_, 0, sizeof(uidev_));
        snprintf(uidev_.name, UINPUT_MAX_NAME_SIZE, "OpenClaw Virtual Input");
        uidev_.id.bustype = BUS_USB;
        uidev_.id.vendor = 0xDEAD;
        uidev_.id.product = 0xBEEF;
        uidev_.id.version = 1;
        
        write(fd_, &uidev_, sizeof(uidev_));
        ioctl(fd_, UI_DEV_CREATE);
        
        // Wait for device node creation
        usleep(500000);
        
        return true;
    }
    
    // High-precision mouse movement
    void MoveMouse(int dx, int dy) {
        emit(EV_REL, REL_X, dx);
        emit(EV_REL, REL_Y, dy);
        emit(EV_SYN, SYN_REPORT, 0);
    }
    
    // Absolute positioning (requires absolute coordinate mapping)
    void MoveMouseAbsolute(int x, int y, int screenWidth, int screenHeight) {
        int absX = (x * 32767) / screenWidth;
        int absY = (y * 32767) / screenHeight;
        
        emit(EV_ABS, ABS_X, absX);
        emit(EV_ABS, ABS_Y, absY);
        emit(EV_SYN, SYN_REPORT, 0);
    }
    
    void MouseClick(int button, bool down) {
        int btnCode;
        switch (button) {
            case 0: btnCode = BTN_LEFT; break;
            case 1: btnCode = BTN_RIGHT; break;
            case 2: btnCode = BTN_MIDDLE; break;
            default: btnCode = BTN_LEFT;
        }
        emit(EV_KEY, btnCode, down ? 1 : 0);
        emit(EV_SYN, SYN_REPORT, 0);
    }
    
    void Scroll(int delta) {
        emit(EV_REL, REL_WHEEL, delta);
        emit(EV_SYN, SYN_REPORT, 0);
    }
    
    void KeyPress(int keyCode, bool down) {
        int linuxKeyCode = ConvertToLinuxKeyCode(keyCode);
        emit(EV_KEY, linuxKeyCode, down ? 1 : 0);
        emit(EV_SYN, SYN_REPORT, 0);
    }
    
    // Multi-touch support (for mobile-like interaction)
    void MultiTouch(int slot, int x, int y, bool touchDown, 
                    int screenWidth, int screenHeight) {
        emit(EV_ABS, ABS_MT_SLOT, slot);
        
        if (touchDown) {
            emit(EV_ABS, ABS_MT_TRACKING_ID, slot + 1);
            emit(EV_ABS, ABS_MT_POSITION_X, (x * 32767) / screenWidth);
            emit(EV_ABS, ABS_MT_POSITION_Y, (y * 32767) / screenHeight);
            emit(EV_KEY, BTN_TOUCH, 1);
        } else {
            emit(EV_ABS, ABS_MT_TRACKING_ID, -1);
            emit(EV_KEY, BTN_TOUCH, 0);
        }
        
        emit(EV_SYN, SYN_REPORT, 0);
    }
    
private:
    void emit(int type, int code, int value) {
        struct input_event ev;
        memset(&ev, 0, sizeof(ev));
        ev.type = type;
        ev.code = code;
        ev.value = value;
        ev.time.tv_sec = 0;
        ev.time.tv_usec = 0;
        write(fd_, &ev, sizeof(ev));
    }
    
    int ConvertToLinuxKeyCode(int keyCode) {
        // Convert from USB HID keycodes to Linux keycodes
        // Reference: linux/input-event-codes.h
        static const int mapping[] = {
            [0x04] = KEY_A, [0x05] = KEY_B, // ... etc
        };
        return mapping[keyCode] ?: keyCode;
    }
};

} // namespace input
} // namespace openclaw
```

### 10.2 Windows: SendInput + FakerInput

**Source Reference:** `autoptt.com/posts/simulating-a-real-keyboard-with-faker-input/`

```cpp
// File: src/input/windows/sendinput_controller.cpp
// Windows input simulation with hardware-level fallback

#include <windows.h>

namespace openclaw {
namespace input {

class WindowsInputController {
private:
    bool useFakerInput_ = false;
    HANDLE fakerDevice_ = INVALID_HANDLE_VALUE;
    
public:
    bool Initialize() {
        // Try FakerInput driver first (undetectable by anti-cheat)
        fakerDevice_ = CreateFile(
            L"\\\\.\\FakerInput",
            GENERIC_READ | GENERIC_WRITE,
            0,
            NULL,
            OPEN_EXISTING,
            FILE_ATTRIBUTE_NORMAL,
            NULL
        );
        
        useFakerInput_ = (fakerDevice_ != INVALID_HANDLE_VALUE);
        
        return true;
    }
    
    void MoveMouse(int x, int y, bool absolute = false) {
        if (useFakerInput_) {
            FAKER_INPUT_MOUSE_REPORT report = {};
            report.ReportID = FAKER_REPORT_ID_MOUSE;
            report.X = x;
            report.Y = y;
            report.Absolute = absolute ? 1 : 0;
            
            DWORD written;
            WriteFile(fakerDevice_, &report, sizeof(report), &written, NULL);
        } else {
            INPUT input = {};
            input.type = INPUT_MOUSE;
            input.mi.dx = absolute ? (x * 65535) / GetSystemMetrics(SM_CXSCREEN) : x;
            input.mi.dy = absolute ? (y * 65535) / GetSystemMetrics(SM_CYSCREEN) : y;
            input.mi.dwFlags = MOUSEEVENTF_MOVE | 
                              (absolute ? MOUSEEVENTF_ABSOLUTE : 0);
            
            SendInput(1, &input, sizeof(INPUT));
        }
    }
    
    void MouseClick(int button, bool down) {
        DWORD flags;
        switch (button) {
            case 0: flags = down ? MOUSEEVENTF_LEFTDOWN : MOUSEEVENTF_LEFTUP; break;
            case 1: flags = down ? MOUSEEVENTF_RIGHTDOWN : MOUSEEVENTF_RIGHTUP; break;
            case 2: flags = down ? MOUSEEVENTF_MIDDLEDOWN : MOUSEEVENTF_MIDDLEUP; break;
        }
        
        INPUT input = {};
        input.type = INPUT_MOUSE;
        input.mi.dwFlags = flags;
        input.mi.dwExtraInfo = GetMessageExtraInfo();
        
        SendInput(1, &input, sizeof(INPUT));
    }
    
    void KeyPress(WORD vkCode, bool down, bool extended = false) {
        INPUT input = {};
        input.type = INPUT_KEYBOARD;
        input.ki.wVk = vkCode;
        input.ki.dwFlags = (down ? 0 : KEYEVENTF_KEYUP) | 
                          (extended ? KEYEVENTF_EXTENDEDKEY : 0);
        input.ki.dwExtraInfo = GetMessageExtraInfo();
        
        // Use scan code for games that ignore virtual key codes
        input.ki.wScan = MapVirtualKey(vkCode, MAPVK_VK_TO_VSC_EX);
        if (input.ki.wScan & 0xE000) {
            input.ki.dwFlags |= KEYEVENTF_SCANCODE | KEYEVENTF_EXTENDEDKEY;
        } else {
            input.ki.dwFlags |= KEYEVENTF_SCANCODE;
        }
        input.ki.wVk = 0; // Use scan code only
        
        SendInput(1, &input, sizeof(INPUT));
    }
    
    // DirectInput-compatible raw input
    void SendRawInput(WORD usagePage, WORD usage, LONG value) {
        // For applications using Raw Input API
        // This requires a custom HID mini-driver
    }
};

} // namespace input
} // namespace openclaw
```

---

## 11. TUI (Terminal User Interface) Automation

### 11.1 PTY-Based Terminal Automation

**Source Reference:** `github.com/msmps/pilotty`, `mcpmarket.com/server/tui`

```typescript
// File: src/tui/pty_controller.ts
// TUI automation via pseudoterminal

import { spawn, IPty } from 'node-pty';
import { Terminal } from 'xterm-headless';
import { SerializeAddon } from 'xterm-addon-serialize';
import { ImageAddon } from 'xterm-addon-image';

interface TUISession {
    id: string;
    pty: IPty;
    terminal: Terminal;
    application: string;
    screenBuffer: string;
    cellMatrix: CellData[][];
}

interface CellData {
    char: string;
    fgColor: string;
    bgColor: string;
    bold: boolean;
    italic: boolean;
    underline: boolean;
    inverse: boolean;
}

class TUIAutomationController {
    private sessions: Map<string, TUISession> = new Map();
    private serializeAddon: SerializeAddon = new SerializeAddon();
    
    // Launch a TUI application
    async launchApplication(
        command: string, 
        args: string[] = [],
        options: {
            cols?: number;
            rows?: number;
            env?: Record<string, string>;
            cwd?: string;
        } = {}
    ): Promise<string> {
        const id = `tui_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
        
        // Spawn PTY
        const pty = spawn(command, args, {
            name: 'xterm-256color',
            cols: options.cols || 120,
            rows: options.rows || 40,
            cwd: options.cwd || process.cwd(),
            env: {
                ...process.env,
                ...options.env,
                TERM: 'xterm-256color',
                COLORTERM: 'truecolor'
            }
        });
        
        // Create headless terminal for rendering
        const terminal = new Terminal({
            cols: options.cols || 120,
            rows: options.rows || 40,
            allowProposedApi: true,
            scrollback: 1000
        });
        
        // Enable image support for Sixel/Kitty graphics
        terminal.loadAddon(new ImageAddon());
        terminal.loadAddon(this.serializeAddon);
        
        // Capture output
        pty.onData((data: string) => {
            terminal.write(data);
            this.updateScreenBuffer(id);
        });
        
        // Handle exit
        pty.onExit(({ exitCode, signal }) => {
            this.sessions.delete(id);
        });
        
        const session: TUISession = {
            id,
            pty,
            terminal,
            application: command,
            screenBuffer: '',
            cellMatrix: []
        };
        
        this.sessions.set(id, session);
        
        // Wait for initial render
        await this.waitForStable(id, 500);
        
        return id;
    }
    
    // Get current screen state as text
    getScreenText(sessionId: string): string {
        const session = this.sessions.get(sessionId);
        if (!session) return '';
        
        return session.terminal.buffer.active
            .getNullCell()
            .toString();
    }
    
    // Get cell-level data for precise analysis
    getCellMatrix(sessionId: string): CellData[][] {
        const session = this.sessions.get(sessionId);
        if (!session) return [];
        
        const matrix: CellData[][] = [];
        const buffer = session.terminal.buffer.active;
        
        for (let row = 0; row < session.terminal.rows; row++) {
            const rowData: CellData[] = [];
            for (let col = 0; col < session.terminal.cols; col++) {
                const cell = buffer.getLine(row)?.getCell(col);
                if (cell) {
                    rowData.push({
                        char: cell.getChars() || ' ',
                        fgColor: cell.getFgColorMode().toString(),
                        bgColor: cell.getBgColorMode().toString(),
                        bold: cell.isBold(),
                        italic: cell.isItalic(),
                        underline: cell.isUnderline(),
                        inverse: cell.isInverse()
                    });
                }
            }
            matrix.push(rowData);
        }
        
        return matrix;
    }
    
    // Send keyboard input
    async sendInput(sessionId: string, input: string | KeyInput): Promise<void> {
        const session = this.sessions.get(sessionId);
        if (!session) throw new Error('Session not found');
        
        if (typeof input === 'string') {
            session.pty.write(input);
        } else {
            // Handle special keys
            const keySequence = this.convertKeyToSequence(input);
            session.pty.write(keySequence);
        }
        
        // Wait for screen update
        await this.waitForStable(sessionId, 100);
    }
    
    // Find text on screen and interact with it
    async findAndInteract(
        sessionId: string, 
        pattern: string | RegExp,
        action: 'click' | 'hover' | { type: 'key', key: string }
    ): Promise<{ found: boolean; position?: { row: number; col: number } }> {
        const matrix = this.getCellMatrix(sessionId);
        
        // Search for pattern
        for (let row = 0; row < matrix.length; row++) {
            const rowText = matrix[row].map(c => c.char).join('');
            const match = rowText.match(pattern);
            
            if (match) {
                const col = match.index!;
                
                if (action === 'click' || action === 'hover') {
                    // Navigate to position using arrow keys
                    await this.navigateTo(sessionId, row, col);
                    
                    if (action === 'click') {
                        await this.sendInput(sessionId, { key: 'Enter' });
                    }
                } else if (typeof action === 'object') {
                    await this.navigateTo(sessionId, row, col);
                    await this.sendInput(sessionId, action.key);
                }
                
                return { found: true, position: { row, col } };
            }
        }
        
        return { found: false };
    }
    
    // Navigate cursor to specific cell using arrow keys
    private async navigateTo(
        sessionId: string, 
        targetRow: number, 
        targetCol: number
    ): Promise<void> {
        const session = this.sessions.get(sessionId)!;
        const cursor = session.terminal.buffer.active.cursorY;
        
        // Use arrow keys to navigate (works for most TUIs)
        const rowDelta = targetRow - cursor;
        const colDelta = targetCol; // Assuming start of line
        
        if (rowDelta > 0) {
            await this.sendInput(sessionId, 
                { key: 'Down' }.repeat(rowDelta));
        } else if (rowDelta < 0) {
            await this.sendInput(sessionId, 
                { key: 'Up' }.repeat(-rowDelta));
        }
        
        // Use Home then Right for column
        await this.sendInput(sessionId, { key: 'Home' });
        if (targetCol > 0) {
            await this.sendInput(sessionId, 
                { key: 'Right' }.repeat(targetCol));
        }
    }
    
    // Wait for screen to stabilize
    private async waitForStable(sessionId: string, timeout: number): Promise<void> {
        const session = this.sessions.get(sessionId);
        if (!session) return;
        
        let lastBuffer = '';
        const startTime = Date.now();
        
        while (Date.now() - startTime < timeout) {
            await new Promise(r => setTimeout(r, 50));
            const currentBuffer = this.getScreenText(sessionId);
            
            if (currentBuffer === lastBuffer) {
                return; // Stable
            }
            
            lastBuffer = currentBuffer;
        }
    }
    
    // Convert key name to VT sequence
    private convertKeyToSequence(key: KeyInput): string {
        const sequences: Record<string, string> = {
            'Enter': '\r',
            'Escape': '\u001b',
            'Tab': '\t',
            'Backspace': '\u007f',
            'Up': '\u001b[A',
            'Down': '\u001b[B',
            'Right': '\u001b[C',
            'Left': '\u001b[D',
            'Home': '\u001b[H',
            'End': '\u001b[F',
            'PageUp': '\u001b[5~',
            'PageDown': '\u001b[6~',
            'Delete': '\u001b[3~',
            'Insert': '\u001b[2~',
            'F1': '\u001bOP',
            'F2': '\u001bOQ',
            'F3': '\u001bOR',
            'F4': '\u001bOS',
            'F5': '\u001b[15~',
            'F6': '\u001b[17~',
            'F7': '\u001b[18~',
            'F8': '\u001b[19~',
            'F9': '\u001b[20~',
            'F10': '\u001b[21~',
            'F11': '\u001b[23~',
            'F12': '\u001b[24~',
        };
        
        if (key.ctrl) return String.fromCharCode(key.char!.charCodeAt(0) & 0x1f);
        if (key.alt) return '\u001b' + (key.char || sequences[key.key!] || '');
        
        return sequences[key.key!] || key.char || '';
    }
    
    // Cleanup
    destroy(sessionId: string): void {
        const session = this.sessions.get(sessionId);
        if (session) {
            session.pty.kill();
            session.terminal.dispose();
            this.sessions.delete(sessionId);
        }
    }
}

interface KeyInput {
    key?: string;
    char?: string;
    ctrl?: boolean;
    alt?: boolean;
    shift?: boolean;
}

declare global {
    interface String {
        repeat(count: number): string;
    }
}
```

---

## 12. Mobile Device Automation Integration

### 12.1 Android: scrcpy + ADB

**Source Reference:** `scrcpy.org`, `github.com/Genymobile/scrcpy`

```python
# File: src/mobile/android/scrcpy_controller.py
# Android automation via scrcpy with real-time screen mirror

import subprocess
import socket
import struct
import numpy as np
import cv2
from dataclasses import dataclass
from typing import Optional, Callable, Tuple
import threading
import queue

@dataclass
class TouchEvent:
    action: int  # 0=DOWN, 1=UP, 2=MOVE
    pointer_id: int
    x: int
    y: int
    width: int
    height: int
    pressure: int = 0xffff
    buttons: int = 1

class ScrcpyController:
    """
    Real-time Android device controller using scrcpy protocol.
    Provides sub-frame-latency screen capture and touch injection.
    """
    
    # scrcpy protocol constants
    DEVICE_NAME_LENGTH = 64
    VIDEO_BUFFER_SIZE = 0x10000
    
    # Touch injection actions
    ACTION_DOWN = 0
    ACTION_UP = 1
    ACTION_MOVE = 2
    ACTION_SCROLL = 3
    
    def __init__(self, max_size: int = 0, bit_rate: int = 8000000, 
                 max_fps: int = 60):
        self.max_size = max_size
        self.bit_rate = bit_rate
        self.max_fps = max_fps
        
        self.device_socket: Optional[socket.socket] = None
        self.video_socket: Optional[socket.socket] = None
        self.control_socket: Optional[socket.socket] = None
        
        self.device_name: str = ""
        self.frame_size: Tuple[int, int] = (0, 0)
        
        self.frame_queue: queue.Queue = queue.Queue(maxsize=3)
        self._capture_thread: Optional[threading.Thread] = None
        self._running = False
        
        # H264 decoder
        self._decoder = None
        
    def connect(self, serial: Optional[str] = None, 
                tcpip: Optional[str] = None) -> bool:
        """
        Connect to Android device via ADB and start scrcpy server.
        
        Args:
            serial: Device serial for USB connection
            tcpip: IP:port for wireless ADB connection
        """
        # Push scrcpy server to device
        self._adb("push", "scrcpy-server", "/data/local/tmp/scrcpy-server.jar",
                 serial=serial)
        
        # Forward local ports
        self._adb("forward", "tcp:27183", "localabstract:scrcpy",
                 serial=serial)
        self._adb("forward", "tcp:27184", "localabstract:scrcpy_control",
                 serial=serial)
        
        # Start server on device
        cmd = [
            "shell", "CLASSPATH=/data/local/tmp/scrcpy-server.jar",
            "app_process", "/", "com.genymobile.scrcpy.Server",
            str(self.max_size), str(self.bit_rate), str(self.max_fps),
            "true", "-", "false", "true", "0", "false", "false",
            "-", "-", "false", "-"
        ]
        self._adb(*cmd, serial=serial, background=True)
        
        # Connect video socket
        self.video_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.video_socket.connect(("localhost", 27183))
        
        # Read device info
        device_info = self.video_socket.recv(self.DEVICE_NAME_LENGTH + 4)
        self.device_name = device_info[:self.DEVICE_NAME_LENGTH].decode('utf-8').rstrip('\x00')
        width = struct.unpack(">H", device_info[self.DEVICE_NAME_LENGTH:self.DEVICE_NAME_LENGTH+2])[0]
        height = struct.unpack(">H", device_info[self.DEVICE_NAME_LENGTH+2:self.DEVICE_NAME_LENGTH+4])[0]
        self.frame_size = (width, height)
        
        # Connect control socket
        self.control_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.control_socket.connect(("localhost", 27184))
        
        # Initialize H264 decoder
        self._init_decoder()
        
        # Start capture thread
        self._running = True
        self._capture_thread = threading.Thread(target=self._capture_loop)
        self._capture_thread.start()
        
        return True
    
    def _capture_loop(self):
        """Background thread: decode and queue video frames."""
        while self._running:
            # Read H264 NAL unit from socket
            # Parse PTS, decode with FFmpeg/OpenH264
            # Convert to cv2.Mat and push to queue
            
            frame_data = self._read_video_frame()
            if frame_data:
                frame = self._decode_frame(frame_data)
                
                # Drop oldest frame if queue full (keep latest)
                if self.frame_queue.full():
                    try:
                        self.frame_queue.get_nowait()
                    except queue.Empty:
                        pass
                
                self.frame_queue.put(frame)
    
    def get_frame(self, timeout: float = 0.016) -> Optional[np.ndarray]:
        """
        Get latest decoded frame.
        
        Returns:
            BGRA numpy array of shape (height, width, 4)
        """
        try:
            return self.frame_queue.get(timeout=timeout)
        except queue.Empty:
            return None
    
    def inject_touch(self, event: TouchEvent):
        """Inject touch event into device."""
        if not self.control_socket:
            return
        
        # scrcpy touch injection packet format
        buffer = struct.pack(">BBqiiHHH",
            2,  # INJECT_TOUCH_EVENT
            event.action,
            event.pointer_id,
            event.x,
            event.y,
            self.frame_size[0],  # screen width
            self.frame_size[1],  # screen height
            event.pressure
        )
        buffer += struct.pack(">H", event.buttons)
        
        self.control_socket.sendall(buffer)
    
    def tap(self, x: int, y: int, duration_ms: int = 50):
        """Simple tap at coordinates."""
        # Convert to scrcpy coordinates
        screen_x = int(x * 65535 / self.frame_size[0])
        screen_y = int(y * 65535 / self.frame_size[1])
        
        # Inject DOWN
        self.inject_touch(TouchEvent(
            action=self.ACTION_DOWN,
            pointer_id=0,
            x=screen_x, y=screen_y,
            width=self.frame_size[0],
            height=self.frame_size[1]
        ))
        
        # Small delay
        if duration_ms > 0:
            import time
            time.sleep(duration_ms / 1000.0)
        
        # Inject UP
        self.inject_touch(TouchEvent(
            action=self.ACTION_UP,
            pointer_id=0,
            x=screen_x, y=screen_y,
            width=self.frame_size[0],
            height=self.frame_size[1]
        ))
    
    def swipe(self, x1: int, y1: int, x2: int, y2: int, 
              duration_ms: int = 300):
        """Swipe from (x1,y1) to (x2,y2)."""
        steps = max(int(duration_ms / 16), 10)
        
        for i in range(steps + 1):
            t = i / steps
            x = int(x1 + (x2 - x1) * t)
            y = int(y1 + (y2 - y1) * t)
            
            action = self.ACTION_DOWN if i == 0 else \
                    (self.ACTION_UP if i == steps else self.ACTION_MOVE)
            
            self.inject_touch(TouchEvent(
                action=action,
                pointer_id=0,
                x=int(x * 65535 / self.frame_size[0]),
                y=int(y * 65535 / self.frame_size[1]),
                width=self.frame_size[0],
                height=self.frame_size[1]
            ))
            
            if i < steps:
                import time
                time.sleep(duration_ms / steps / 1000.0)
    
    def inject_key(self, keycode: int, metastate: int = 0, 
                   action: int = 0):  # 0=down, 1=up, 2=multiple
        """Inject key event."""
        buffer = struct.pack(">BBiii",
            0,  # INJECT_KEYCODE
            action,
            keycode,
            metastate,
            0  # repeat
        )
        self.control_socket.sendall(buffer)
    
    def _adb(self, *args, serial: Optional[str] = None, 
             background: bool = False) -> subprocess.Popen:
        """Execute ADB command."""
        cmd = ["adb"]
        if serial:
            cmd.extend(["-s", serial])
        cmd.extend(args)
        
        if background:
            return subprocess.Popen(cmd, stdout=subprocess.PIPE, 
                                  stderr=subprocess.PIPE)
        else:
            return subprocess.run(cmd, capture_output=True, text=True)
    
    def disconnect(self):
        """Clean disconnect."""
        self._running = False
        if self._capture_thread:
            self._capture_thread.join(timeout=1.0)
        if self.video_socket:
            self.video_socket.close()
        if self.control_socket:
            self.control_socket.close()


class UIAutomator2Integration:
    """
    Hybrid approach: scrcpy for video + UIAutomator2 for element hierarchy.
    """
    
    def __init__(self, scrcpy: ScrcpyController):
        self.scrcpy = scrcpy
        self.uiautomator_url = "http://localhost:8200"
    
    def get_element_tree(self) -> dict:
        """Get accessibility tree from UIAutomator2."""
        import requests
        
        # Dump window hierarchy
        resp = requests.get(f"{self.uiautomator_url}/dump/hierarchy")
        return resp.json()
    
    def find_element_by_vision(self, template_path: str, 
                                threshold: float = 0.8) -> Optional[Tuple[int, int]]:
        """Find element on screen using OpenCV template matching."""
        frame = self.scrcpy.get_frame(timeout=0.1)
        if frame is None:
            return None
        
        template = cv2.imread(template_path, cv2.IMREAD_UNCHANGED)
        if template is None:
            return None
        
        # Template matching with OpenCV
        result = cv2.matchTemplate(frame, template, cv2.TM_CCOEFF_NORMED)
        min_val, max_val, min_loc, max_loc = cv2.minMaxLoc(result)
        
        if max_val >= threshold:
            center_x = max_loc[0] + template.shape[1] // 2
            center_y = max_loc[1] + template.shape[0] // 2
            return (center_x, center_y)
        
        return None
    
    def tap_element(self, selector: str, by: str = "xpath", 
                    use_vision_fallback: bool = True):
        """Tap element by selector, with vision fallback."""
        import requests
        
        # Try UIAutomator2 first
        try:
            resp = requests.post(
                f"{self.uiautomator_url}/elements/{by}/click",
                json={"selector": selector}
            )
            if resp.status_code == 200:
                return True
        except:
            pass
        
        # Vision fallback
        if use_vision_fallback:
            # Take screenshot and use OCR/template matching
            pos = self.find_element_by_vision(f"templates/{selector}.png")
            if pos:
                self.scrcpy.tap(pos[0], pos[1])
                return True
        
        return False
```

### 12.2 iOS: WebDriverAgent + go-ios

```python
# File: src/mobile/ios/ios_controller.py
# iOS automation via WebDriverAgent

import requests
import subprocess
import cv2
import numpy as np
from PIL import Image
import io
import wda  # facebook-wda or appium-wda

class iOSController:
    """
    iOS device automation using WebDriverAgent.
    Provides screen capture, element interaction, and gesture injection.
    """
    
    def __init__(self, wda_url: str = "http://localhost:8100"):
        self.wda_url = wda_url
        self.client = wda.Client(wda_url)
        self.session = None
        
    def connect(self, bundle_id: Optional[str] = None) -> bool:
        """
        Connect to iOS device via WebDriverAgent.
        
        Args:
            bundle_id: Target app bundle ID, or None for SpringBoard
        """
        # Start WDA session
        if bundle_id:
            self.session = self.client.session(bundle_id)
        else:
            self.session = self.client.session()
        
        return self.session is not None
    
    def get_screenshot(self) -> np.ndarray:
        """Get screenshot as OpenCV image."""
        resp = requests.get(f"{self.wda_url}/screenshot")
        data = resp.json()
        
        # Decode base64 screenshot
        import base64
        img_data = base64.b64decode(data['value'])
        img = Image.open(io.BytesIO(img_data))
        
        # Convert to OpenCV format (BGR)
        return cv2.cvtColor(np.array(img), cv2.COLOR_RGB2BGR)
    
    def get_page_source(self) -> dict:
        """Get accessibility tree."""
        resp = requests.get(f"{self.wda_url}/source")
        return resp.json()
    
    def tap(self, x: int, y: int):
        """Tap at screen coordinates."""
        # WDA uses coordinate ratio (0.0-1.0)
        window_size = self.session.window_size()
        ratio_x = x / window_size.width
        ratio_y = y / window_size.height
        
        self.session.tap(ratio_x, ratio_y)
    
    def swipe(self, x1: int, y1: int, x2: int, y2: int, 
              duration: float = 0.5):
        """Swipe from (x1,y1) to (x2,y2)."""
        window_size = self.session.window_size()
        
        self.session.swipe(
            x1 / window_size.width, y1 / window_size.height,
            x2 / window_size.width, y2 / window_size.height,
            duration
        )
    
    def find_and_tap(self, predicate: str, timeout: float = 10.0) -> bool:
        """Find element by predicate and tap."""
        try:
            element = self.session(predicate=predicate, timeout=timeout)
            element.tap()
            return True
        except wda.WDAElementNotFoundError:
            return False
    
    def inject_text(self, text: str):
        """Type text."""
        self.session.send_keys(text)
    
    def start_video_stream(self, fps: int = 30) -> subprocess.Popen:
        """Start MJPEG stream for real-time capture."""
        # WDA provides /wda/mjpegServer for continuous stream
        return subprocess.Popen([
            "ffmpeg", "-i", f"{self.wda_url}/wda/mjpegServer",
            "-f", "rawvideo", "-pix_fmt", "bgr24",
            "pipe:"
        ], stdout=subprocess.PIPE)
    
    def get_performance_metrics(self) -> dict:
        """Get CPU, memory, FPS metrics."""
        resp = requests.get(f"{self.wda_url}/wda/performanceMonitor")
        return resp.json()
```

---

## 13. Complete Integration Architecture

### 13.1 System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    OPENCLAW ULTIMATE SYSTEM                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   WEB AUTOM  │  │ DESKTOP AUTOM│  │  MOBILE AUTOM│         │
│  │  Playwright  │  │  RobotGo     │  │  scrcpy/WDA  │         │
│  │  CDP Stage   │  │  pyautogui   │  │  Appium      │         │
│  │  Puppeteer   │  │  nut.js      │  │  UIAutomator2│         │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘         │
│         │                  │                  │                 │
│  ┌──────┴──────────────────┴──────────────────┴──────┐          │
│  │          UNIFIED ACTION ABSTRACTION LAYER         │          │
│  │  click(x,y) │ type(text) │ scroll() │ capture()    │          │
│  └──────────────────────┬────────────────────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │              OBSERVATION ENGINE                    │          │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │          │
│  │  │ LD_PRELOD│ │  LLHooks │ │ Accessibility API│  │          │
│  │  │ plthook  │ │  D-Bus   │ │   AXUIElement    │  │          │
│  │  └──────────┘ └──────────┘ └──────────────────┘  │          │
│  └──────────────────────┬────────────────────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │           CAPTURE ENGINE (Multi-Platform)          │          │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │          │
│  │  │  DXGI DD │ │KMS DMABUF│ │  ScreenCaptureKit│  │          │
│  │  │   NVFBC  │ │ PipeWire │ │   CGDisplay      │  │          │
│  │  │   WGC    │ │  X11 SHM │ │   IOSurface      │  │          │
│  │  └──────────┘ └──────────┘ └──────────────────┘  │          │
│  └──────────────────────┬────────────────────────────┘          │
│                         │ ZERO-COPY GPU TEXTURE                  │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │           PROCESSING PIPELINE (GPU-Accelerated)    │          │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │          │
│  │  │ OpenCV   │ │  Vulkan  │ │   CUDA/TensorRT  │  │          │
│  │  │ (CPU/GPU)│ │  Compute │ │   Inference      │  │          │
│  │  │ Template │ │  Shaders │ │   YOLO/OCR       │  │          │
│  │  │  Match   │ │  Image   │ │   Detection      │  │          │
│  │  └──────────┘ └──────────┘ └──────────────────┘  │          │
│  └──────────────────────┬────────────────────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │              VISION ANALYSIS ENGINE                 │          │
│  │  Element Detection │ OCR │ Icon Recognition │ State │          │
│  │  Change Detection  │ Flow│ Tracking         │ Machine         │
│  └──────────────────────┬────────────────────────────┘          │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────┐          │
│  │         INPUT INJECTION ENGINE                     │          │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │          │
│  │  │ uinput   │ │ SendInput│ │   CGEventPost    │  │          │
│  │  │ evdev    │ │ FakerInpt│ │   AXPerform      │  │          │
│  │  │ XTest    │ │ LLHooks  │ │   AppleScript    │  │          │
│  │  └──────────┘ └──────────┘ └──────────────────┘  │          │
│  └────────────────────────────────────────────────────┘          │
│                                                                  │
│  ┌────────────────────────────────────────────────────┐          │
│  │         RECORDING & STREAMING ENGINE                │          │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │          │
│  │  │ FFmpeg   │ │OBS libobs│ │   WebRTC/WHIP    │  │          │
│  │  │ GStreamer│ │NVENC/VAAPI│ │   WHEP Client   │  │          │
│  │  │ Segmented│ │Segmented │ │   <100ms        │  │          │
│  │  │ Recording│ │Recording │ │   Latency       │  │          │
│  │  └──────────┘ └──────────┘ └──────────────────┘  │          │
│  └────────────────────────────────────────────────────┘          │
│                                                                  │
│  ┌────────────────────────────────────────────────────┐          │
│  │         TUI AUTOMATION ENGINE                       │          │
│  │  node-pty │ xterm-headless │ pilotty │ tmux control │          │
│  │  Screen Buffer Analysis │ VT Sequence Injection      │          │
│  └────────────────────────────────────────────────────┘          │
│                                                                  │
│  ┌────────────────────────────────────────────────────┐          │
│  │              AGENT INTELLIGENCE LAYER               │          │
│  │  browser-use loop │ UI-TARS model │ Planning Engine  │          │
│  │  Error Recovery   │ DOM+Vision    │ State Management │          │
│  │  Context Compaction│ Hybrid       │ Goal Tracking    │          │
│  └────────────────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

### 13.2 Component Interaction Flow

```
[Application Under Automation]
         │
         │ 1. Screen Changes
         ▼
┌─────────────────┐
│  Capture Engine │──┐──┐
│  (GPU Texture)  │  │  │
└─────────────────┘  │  │
         │           │  │
         │ Zero-Copy │  │
         ▼           ▼  ▼
┌─────────────────────────────┐
│   Processing Pipeline       │
│   OpenCV/Vulkan/CUDA        │
│   (All on GPU)              │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│   Vision Analysis           │
│   - Element Detection       │
│   - OCR (PaddleOCR+TensorRT)│
│   - State Classification    │
│   - Change Detection        │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│   Agent Decision (LLM)      │
│   - DOM + Vision Context    │
│   - Goal Planning           │
│   - Action Selection        │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│   Action Execution          │
│   - Coordinate targeting    │
│   - Input injection         │
│   - Verification loop       │
└─────────────────────────────┘
         │
         ▼
┌─────────────────────────────┐     ┌───────────────┐
│   Recording Engine          │────▶│  File/Stream  │
│   - H264 NVENC/VAAPI        │     │  MP4/MKV/WebRTC│
│   - Segmented recording     │     └───────────────┘
│   - Screenshot pipeline     │
└─────────────────────────────┘
```

---

## 14. Step-by-Step Implementation Guide

### Phase 1: Foundation (Weeks 1-2)

#### Step 1.1: Build the Capture Engine

```bash
# Clone and build capture dependencies
git clone https://github.com/obsproject/obs-studio.git
# Reference: obs-studio/libobs/obs-source.c, obs-video.c

git clone https://github.com/w23/obs-kmsgrab.git
# Reference: DMA-BUF zero-copy capture implementation

# Build OpenClaw capture module
mkdir build && cd build
cmake -DENABLE_DXGI=ON -DENABLE_KMS=ON -DENABLE_PIPEWIRE=ON \
      -DENABLE_SCREENCAPTUREKIT=ON -DENABLE_NVENC=ON ..
make -j$(nproc)
```

**Key source files to reference:**
- `obs-studio/libobs/obs-source.c` - Source object architecture
- `obs-studio/plugins/win-capture/` - Windows capture implementations
- `obs-kmsgrab/kmsgrab.c` - KMS DMA-BUF capture
- `scrcpy/app/src/screen.c` - Android screen capture protocol

#### Step 1.2: Integrate OpenCV with CUDA Support

```bash
# Build OpenCV with CUDA (NVIDIA)
# Reference: docs.opencv.org/4.x/d2/dbc/cuda_intro.html

git clone https://github.com/opencv/opencv.git
git clone https://github.com/opencv/opencv_contrib.git

mkdir opencv/build && cd opencv/build
cmake -D CMAKE_BUILD_TYPE=RELEASE \
      -D CMAKE_INSTALL_PREFIX=/usr/local \
      -D WITH_CUDA=ON \
      -D CUDA_ARCH_BIN=7.5,8.0,8.6,8.9,9.0 \
      -D WITH_CUDNN=ON \
      -D OPENCV_DNN_CUDA=ON \
      -D ENABLE_FAST_MATH=ON \
      -D CUDA_FAST_MATH=ON \
      -D WITH_CUBLAS=ON \
      -D OPENCV_EXTRA_MODULES_PATH=../../opencv_contrib/modules \
      -D BUILD_opencv_cudacodec=ON \
      -D BUILD_opencv_cudaimgproc=ON \
      -D BUILD_opencv_cudaobjdetect=ON \
      -D BUILD_opencv_cudafeatures2d=ON \
      ..
make -j$(nproc)
sudo make install
```

#### Step 1.3: Set up TensorRT for Inference

```bash
# Install TensorRT (requires NVIDIA Developer account)
# Reference: developer.nvidia.com/tensorrt

# Download and install
sudo dpkg -i nv-tensorrt-local-repo-ubuntu2204-8.6.1-cuda-12.0_1.0-1_amd64.deb
sudo cp /var/nv-tensorrt-local-repo-ubuntu2204-8.6.1-cuda-12.0/*-keyring.gpg /usr/share/keyrings/
sudo apt-get update
sudo apt-get install tensorrt

# Verify installation
/usr/src/tensorrt/bin/trtexec --version
```

### Phase 2: Vision Pipeline (Weeks 3-4)

#### Step 2.1: Build GPU Analysis Pipeline

Implement the `GPUAnalysisPipeline` class from Section 3.2:

```cpp
// src/vision/gpu_pipeline.cpp - Reference implementation
// Key OpenCV GPU functions to use:
// - cv::cuda::cvtColor() - Color space conversion
// - cv::cuda::resize() - Fast GPU resize
// - cv::cuda::TemplateMatching - Element finding
// - cv::cuda::CannyEdgeDetector - Edge detection
// - cv::cuda::HOG - Object detection
// - cv::cuda::SURF_CUDA - Feature detection
```

#### Step 2.2: Build Vulkan Compute Fallback

```bash
# Build vkCompViz for reference
git clone --recursive https://github.com/ichlubna/vkCompViz.git
cd vkCompViz && mkdir build && cd build
cmake .. -G "Ninja"
ninja

# Study src/ for Vulkan compute patterns
# Reference: examples/SimpleBlending, examples/ParallelReduction
```

#### Step 2.3: Integrate OCR Pipeline

```python
# PaddleOCR with TensorRT acceleration
# Reference: github.com/PaddlePaddle/PaddleOCR

from paddleocr import PaddleOCR
import cv2
import numpy as np

class GPUOCREngine:
    def __init__(self, use_tensorrt=True):
        self.ocr = PaddleOCR(
            use_angle_cls=True,
            lang='en',
            use_gpu=True,
            enable_mkldnn=True,
            use_tensorrt=use_tensorrt,
            precision='fp16'  # Use FP16 for 2x speedup
        )
    
    def recognize_screen(self, gpu_frame: cv2.cuda.GpuMat) -> list:
        # Download minimal region for OCR (keep on GPU until needed)
        cpu_frame = gpu_frame.download()
        result = self.ocr.ocr(cpu_frame, cls=True)
        
        # Parse results: [box, (text, confidence)]
        elements = []
        for line in result[0]:
            box = line[0]
            text = line[1][0]
            conf = line[1][1]
            
            elements.append({
                'text': text,
                'confidence': conf,
                'bbox': {
                    'x': min(p[0] for p in box),
                    'y': min(p[1] for p in box),
                    'width': max(p[0] for p in box) - min(p[0] for p in box),
                    'height': max(p[1] for p in box) - min(p[1] for p in box)
                }
            })
        
        return elements
```

### Phase 3: Input System (Weeks 5-6)

#### Step 3.1: Linux uinput Implementation

Build the `UInputController` from Section 10.1:

```bash
# Build uinput module
gcc -shared -fPIC -o libopenclaw_input.so \
    src/input/linux/uinput_controller.cpp \
    src/input/linux/uinput_keyboard_map.cpp \
    -I/usr/local/include/opencv4 \
    -std=c++17

# Set permissions (required for uinput access)
sudo usermod -aG input $USER
# Or use polkit for capability management
```

#### Step 3.2: Windows Input Implementation

Build the `WindowsInputController` from Section 10.2:

```cpp
// Build with Visual Studio or MinGW
// Requirements: Windows SDK, DDK for FakerInput

cl /O2 /EHsc /Fe:openclaw_input.dll \
   /DUNICODE /D_UNICODE \
   src/input/windows/sendinput_controller.cpp \
   src/input/windows/fakerinput_bridge.cpp \
   user32.lib gdi32.lib
```

#### Step 3.3: macOS Input Implementation

Build using Xcode command line tools:

```bash
clang++ -dynamiclib -framework ApplicationServices \
        -framework Cocoa \
        -o libopenclaw_input.dylib \
        src/input/macos/cgevent_controller.mm \
        src/input/macos/axui_action.mm
```

### Phase 4: Recording System (Weeks 7-8)

#### Step 4.1: Build FFmpeg Pipeline

```python
# src/recording/ffmpeg_recorder.py
import ffmpeg
import subprocess
import threading
import queue

class FFmpegSegmentedRecorder:
    """
    Hardware-accelerated segmented recording.
    Creates continuous recordings with automatic segmentation.
    """
    
    def __init__(self, output_prefix: str, segment_duration: int = 300):
        self.output_prefix = output_prefix
        self.segment_duration = segment_duration
        self.process = None
        self.frame_queue = queue.Queue(maxsize=30)
        
    def start(self, width: int, height: int, fps: int = 30,
              encoder: str = 'h264_nvenc'):
        """
        Start recording.
        
        Encoders:
        - h264_nvenc: NVIDIA hardware (fastest)
        - h264_vaapi: Intel/AMD hardware
        - libx264: Software (most compatible)
        - hevc_nvenc: H265 NVIDIA
        """
        
        command = [
            'ffmpeg',
            '-y',  # Overwrite output
            '-f', 'rawvideo',
            '-vcodec', 'rawvideo',
            '-pix_fmt', 'bgr24',
            '-s', f'{width}x{height}',
            '-r', str(fps),
            '-i', '-',  # Read from stdin
            '-c:v', encoder,
            '-preset', 'p4',  # NVENC preset (p1-p7)
            '-rc', 'vbr',  # Variable bitrate
            '-cq', '23',  # Quality level
            '-bf', '2',  # B-frames
            '-segment_time', str(self.segment_duration),
            '-f', 'segment',
            '-reset_timestamps', '1',
            '-strftime', '1',
            f'{self.output_prefix}_%Y%m%d_%H%M%S.mkv'
        ]
        
        self.process = subprocess.Popen(
            command,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
        
        # Start frame writer thread
        self.writer_thread = threading.Thread(target=self._write_frames)
        self.writer_thread.start()
        
    def write_frame(self, frame: np.ndarray):
        """Write a frame (BGR format)."""
        try:
            self.frame_queue.put_nowait(frame)
        except queue.Full:
            pass  # Drop frame if encoder can't keep up
        
    def _write_frames(self):
        while self.process and self.process.poll() is None:
            try:
                frame = self.frame_queue.get(timeout=1.0)
                self.process.stdin.write(frame.tobytes())
            except queue.Empty:
                continue
                
    def stop(self):
        if self.process:
            self.process.stdin.close()
            self.process.wait(timeout=10)
            self.process = None
```

#### Step 4.2: Build Screenshot Pipeline

```cpp
// src/recording/screenshot_pipeline.cpp
// Continuous screenshot capture with analysis

class ScreenshotPipeline {
public:
    struct ScreenshotResult {
        cv::Mat image;
        uint64_t timestamp;
        std::vector<UIElement> detectedElements;
        std::vector<TextRegion> ocrRegions;
        cv::Mat diffMask;  // Changed regions since last capture
    };
    
    void StartContinuousCapture(int intervalMs = 100) {
        std::thread([this, intervalMs]() {
            cv::Mat lastFrame;
            
            while (running_) {
                auto start = std::chrono::steady_clock::now();
                
                // Capture frame (GPU zero-copy)
                GPUFrame gpuFrame = capture_->AcquireGPUFrame();
                
                // Process on GPU
                cv::cuda::GpuMat gpuMat = gpuFrame.GetCvGpuMat();
                
                // Download for analysis (async)
                cv::Mat cpuFrame;
                gpuMat.download(cpuFrame);
                
                // Compute diff if we have a previous frame
                cv::Mat diffMask;
                if (!lastFrame.empty()) {
                    cv::Mat diff;
                    cv::absdiff(lastFrame, cpuFrame, diff);
                    cv::cvtColor(diff, diffMask, cv::COLOR_BGR2GRAY);
                    cv::threshold(diffMask, diffMask, 30, 255, 
                                 cv::THRESH_BINARY);
                    
                    // Only analyze changed regions
                    if (cv::countNonZero(diffMask) > 100) {
                        auto result = AnalyzeChangedRegions(
                            cpuFrame, diffMask
                        );
                        onScreenshot_(result);
                    }
                }
                
                lastFrame = cpuFrame.clone();
                
                // Throttle to interval
                auto elapsed = std::chrono::steady_clock::now() - start;
                auto sleepMs = intervalMs - 
                    std::chrono::duration_cast<std::chrono::milliseconds>(
                        elapsed).count();
                if (sleepMs > 0) {
                    std::this_thread::sleep_for(
                        std::chrono::milliseconds(sleepMs)
                    );
                }
            }
        }).detach();
    }
};
```

### Phase 5: Hook System (Weeks 9-10)

#### Step 5.1: Build LD_PRELOAD Library

```bash
# Build the interceptor library
gcc -shared -fPIC -o libopenclaw_hook.so \
    src/hooks/linux/ld_preload_interceptor.c \
    src/hooks/linux/plt_hook_runtime.c \
    src/hooks/linux/hook_ipc.c \
    -ldl -lpthread -lrt

# Install system-wide or use per-process:
LD_PRELOAD=/path/to/libopenclaw_hook.so ./target_app
```

#### Step 5.2: Build Windows Hook DLL

```cpp
// Build with Visual Studio
// src/hooks/windows/ll_hooks.cpp + dllmain.cpp

// Inject into target process:
// Method 1: SetWindowsHookEx
HMODULE hDll = LoadLibraryA("openclaw_hook.dll");
HOOKPROC hookProc = (HOOKPROC)GetProcAddress(hDll, "KeyboardProc");
HHOOK hHook = SetWindowsHookEx(WH_KEYBOARD_LL, hookProc, hDll, 0);

// Method 2: Manual DLL injection
// OpenProcess -> VirtualAllocEx -> WriteProcessMemory -> CreateRemoteThread
```

### Phase 6: TUI Automation (Weeks 11-12)

#### Step 6.1: Integrate node-pty + xterm-headless

```bash
# Install dependencies
npm install node-pty xterm-headless xterm-addon-serialize xterm-addon-image

# Build native module
npm rebuild node-pty
```

#### Step 6.2: Build TUI Controller Service

```typescript
// src/tui/pty_service.ts
// Standalone service for TUI automation

import * as pty from 'node-pty';
import { Terminal } from 'xterm-headless';
import * as net from 'net';

class TUIService {
    private sessions = new Map<string, TUISession>();
    private server: net.Server;
    
    start(port: number = 7432) {
        this.server = net.createServer((socket) => {
            socket.on('data', (data) => {
                const cmd = JSON.parse(data.toString());
                this.handleCommand(socket, cmd);
            });
        });
        
        this.server.listen(port);
        console.log(`TUI Service listening on port ${port}`);
    }
    
    private handleCommand(socket: net.Socket, cmd: any) {
        switch (cmd.type) {
            case 'LAUNCH':
                const id = this.launch(cmd.command, cmd.args, cmd.options);
                socket.write(JSON.stringify({ id }));
                break;
                
            case 'INPUT':
                this.sendInput(cmd.sessionId, cmd.input);
                break;
                
            case 'SCREENSHOT':
                const png = this.getScreenshot(cmd.sessionId);
                socket.write(JSON.stringify({ png: png.toString('base64') }));
                break;
                
            case 'GET_TEXT':
                const text = this.getScreenText(cmd.sessionId);
                socket.write(JSON.stringify({ text }));
                break;
                
            case 'FIND':
                const found = this.findText(cmd.sessionId, cmd.pattern);
                socket.write(JSON.stringify(found));
                break;
                
            case 'NAVIGATE':
                this.navigateTo(cmd.sessionId, cmd.row, cmd.col);
                break;
                
            case 'DESTROY':
                this.destroy(cmd.sessionId);
                break;
        }
    }
}

// Usage from Python:
// sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
// sock.connect(('localhost', 7432))
// sock.send(json.dumps({'type': 'LAUNCH', 'command': 'htop'}).encode())
```

### Phase 7: Mobile Integration (Weeks 13-14)

#### Step 7.1: Android scrcpy Integration

```bash
# Setup scrcpy
# Ubuntu/Debian
sudo apt install scrcpy

# Or build from source
git clone https://github.com/Genymobile/scrcpy.git
cd scrcpy
meson setup x --buildtype=release --strip -Db_lto=true
ninja -Cx
sudo ninja -Cx install

# Reference server source for protocol:
# scrcpy/server/src/main/java/com/genymobile/scrcpy/
```

#### Step 7.2: iOS WebDriverAgent Setup

```bash
# Clone and build WebDriverAgent
git clone https://github.com/appium/WebDriverAgent.git
cd WebDriverAgent

# Install dependencies
./Scripts/bootstrap.sh

# Build and test
xcodebuild -project WebDriverAgent.xcodeproj \
           -scheme WebDriverAgentRunner \
           -destination 'platform=iOS Simulator,name=iPhone 15' \
           test

# For real device, need signing certificate
```

### Phase 8: Integration & Testing (Weeks 15-16)

#### Step 8.1: Unified API Layer

```typescript
// src/api/unified_automation.ts
// Single API for all automation targets

interface AutomationTarget {
    type: 'web' | 'desktop' | 'mobile' | 'tui' | 'api';
    connection: ConnectionConfig;
}

interface AutomationAction {
    type: 'click' | 'type' | 'scroll' | 'swipe' | 'keypress' | 
          'capture' | 'find' | 'wait' | 'exec';
    params: Record<string, any>;
}

class OpenClawAutomation {
    private engines: Map<string, IAutomationEngine> = new Map();
    private captureEngine: ICaptureEngine;
    private visionEngine: VisionAnalysisEngine;
    private recordingEngine: RecordingEngine;
    
    async connect(target: AutomationTarget): Promise<string> {
        let engine: IAutomationEngine;
        
        switch (target.type) {
            case 'web':
                engine = new WebAutomationEngine();
                break;
            case 'desktop':
                engine = new DesktopAutomationEngine();
                break;
            case 'mobile':
                engine = new MobileAutomationEngine(target.connection.platform);
                break;
            case 'tui':
                engine = new TUIAutomationEngine();
                break;
            case 'api':
                engine = new APIAutomationEngine();
                break;
        }
        
        await engine.connect(target.connection);
        const sessionId = generateId();
        this.engines.set(sessionId, engine);
        
        return sessionId;
    }
    
    async execute(sessionId: string, action: AutomationAction): Promise<any> {
        const engine = this.engines.get(sessionId);
        if (!engine) throw new Error('Session not found');
        
        // Pre-action: capture state
        const beforeCapture = await this.captureEngine.capture();
        
        // Execute action
        const result = await engine.execute(action);
        
        // Post-action: capture and verify
        const afterCapture = await this.captureEngine.capture();
        const changes = this.visionEngine.detectChanges(
            beforeCapture, afterCapture
        );
        
        // Record action + result
        await this.recordingEngine.recordAction({
            action,
            beforeState: beforeCapture,
            afterState: afterCapture,
            changes,
            result,
            timestamp: Date.now()
        });
        
        return { result, changes };
    }
    
    async startRecording(sessionId: string, config: RecordingConfig): 
        Promise<string> {
        const engine = this.engines.get(sessionId);
        
        return this.recordingEngine.start({
            source: engine.getVideoSource(),
            ...config
        });
    }
    
    async analyzeScreen(sessionId: string): Promise<ScreenAnalysis> {
        const capture = await this.captureEngine.capture();
        return this.visionEngine.analyze(capture);
    }
}
```

---

## 15. Source Code Reference Map

### 15.1 OpenCV GPU Modules

| File | Path | Purpose |
|------|------|---------|
| `cudaarithm.hpp` | `opencv/modules/cudaarithm/include/` | CUDA arithmetic operations |
| `cudaimgproc.hpp` | `opencv/modules/cudaimgproc/include/` | CUDA image processing |
| `cudafilters.hpp` | `opencv/modules/cudaimgproc/include/` | CUDA filters |
| `cudaobjdetect.hpp` | `opencv/modules/cudaobjdetect/include/` | CUDA object detection |
| `cudafeatures2d.hpp` | `opencv/modules/cudafeatures2d/include/` | CUDA feature detection |
| `match_template.cpp` | `opencv/modules/cudaimgproc/src/` | Template matching implementation |
| `canny.cpp` | `opencv/modules/cudaimgproc/src/` | Canny edge detection |

### 15.2 OBS Studio (Recording Architecture)

| File | Path | Purpose |
|------|------|---------|
| `obs-video.c` | `libobs/` | Graphics thread implementation |
| `video-io.c` | `libobs/media-io/` | Video encoding thread |
| `audio-io.c` | `libobs/media-io/` | Audio processing thread |
| `obs-source.c` | `libobs/` | Source object base class |
| `obs-encoder.c` | `libobs/` | Encoder interface |
| `obs-output.c` | `libobs/` | Output interface |
| `dc-capture.c` | `plugins/win-capture/` | Windows display capture |
| `kmsgrab.c` | `obs-kmsgrab/` | Linux KMS capture |

### 15.3 scrcpy (Android)

| File | Path | Purpose |
|------|------|---------|
| `server.c` | `app/src/` | Main server entry point |
| `screen.c` | `app/src/` | Video encoding and streaming |
| `controller.c` | `app/src/` | Input injection |
| `device.c` | `app/src/` | Device info and capabilities |
| `Server.java` | `server/src/main/java/com/genymobile/scrcpy/` | Java server |
| `Device.java` | Same | Device management |
| `Controller.java` | Same | Input event handling |

### 15.4 TensorRT

| File | Path | Purpose |
|------|------|---------|
| `NvInfer.h` | `include/` | Main API header |
| `NvOnnxParser.h` | `include/` | ONNX parser |
| `sampleONNXMNIST.cpp` | `samples/` | ONNX inference example |
| `trtexec.cpp` | `samples/trtexec/` | Command-line tool |

### 15.5 Vulkan Compute

| File | Path | Purpose |
|------|------|---------|
| `vkCompViz.h` | `vkCompViz/src/` | Library header |
| `SimpleBlending.cpp` | `vkCompViz/examples/` | Basic compute example |
| `ParallelReduction.cpp` | `vkCompViz/examples/` | Reduction pattern |

### 15.6 Linux Input

| File | Path | Purpose |
|------|------|---------|
| `uinput.c` | `drivers/input/misc/` | Kernel uinput driver |
| `evdev.c` | `drivers/input/` | Kernel evdev driver |
| `libevdev-uinput.c` | `libevdev/src/` | libevdev uinput wrapper |

### 15.7 Windows Capture

| File | Path | Purpose |
|------|------|---------|
| `duplicationapi.cpp` | Windows SDK Samples | DXGI Desktop Duplication |
| ` ScreenCapture.h` | Windows SDK | Screen capture API |

---

## Appendix A: Performance Benchmarks

### Expected Performance Targets

| Operation | Target Latency | Hardware Required |
|-----------|---------------|-------------------|
| Screen capture (GPU) | <5ms | Any GPU with DMA-BUF/DXGI |
| Screen capture + encode | <10ms | NVENC/VAAPI capable |
| OpenCV template matching (1080p) | <2ms | CUDA GPU |
| Full vision analysis pipeline | <16ms | RTX 3060 or better |
| Input injection | <1ms | Kernel uinput/SendInput |
| OCR (100 words) | <50ms | TensorRT + RTX |
| Web automation action cycle | <100ms | Network dependent |
| TUI interaction cycle | <50ms | CPU only |
| Mobile frame capture | <33ms | USB 3.0 |

### GPU Memory Requirements

| Resolution | Capture | Processing | Inference | Total |
|------------|---------|------------|-----------|-------|
| 1080p@60 | ~250MB | ~500MB | ~1GB | ~1.75GB |
| 1440p@60 | ~440MB | ~880MB | ~1.5GB | ~2.8GB |
| 4K@60 | ~1GB | ~2GB | ~2GB | ~5GB |

---

## Appendix B: Security Considerations

### Permission Requirements

| Platform | Feature | Permission |
|----------|---------|------------|
| Linux | KMS capture | `video` group or root |
| Linux | uinput | `input` group or udev rule |
| Linux | LD_PRELOAD | Target process permissions |
| Windows | DXGI capture | User-level |
| Windows | LLHooks | User-level (some apps block) |
| macOS | ScreenCaptureKit | Screen Recording TCC |
| macOS | AX access | Accessibility TCC |
| Android | ADB | Developer mode + USB debug |
| iOS | WebDriverAgent | Developer certificate |

---

*End of Document*
*Total Integration Points: 47 source modules*
*Estimated Implementation Timeline: 16 weeks*
*Technologies Integrated: 35+


---

## Additional Game-Changer Technologies

### A. NVIDIA Maxine SDK for Real-Time Video AI

**Source:** `github.com/NVIDIA-Maxine/VFX-SDK-Samples`, `catalog.ngc.nvidia.com`

NVIDIA Maxine provides GPU-accelerated real-time video processing that can enhance the recording pipeline:

```cpp
// File: src/recording/maxine_enhancer.cpp
// Integration of Maxine Video Effects into capture pipeline

#include "NvVFX.h"

class MaxineEnhancer {
    // Available effects:
    // - Video Super Resolution: 360p → 4K in real-time
    // - AI Green Screen: Background removal
    // - Video Denoising: Clean low-light captures
    // - Artifact Reduction: Remove compression artifacts
    // - Video Relighting: HDR-style lighting adjustment
};
```

### B. AMD ROCm for Cross-Vendor GPU Compute

**Source:** `github.com/ROCm`

For AMD GPUs, ROCm provides CUDA-compatible compute:

```cpp
// Hipify CUDA code for AMD
// Convert .cu files to .cpp with HIP APIs
hipify-perl cuda_code.cu > hip_code.cpp

// Build with hipcc
hipcc hip_code.cpp -o hip_binary

// Same API as CUDA but runs on AMD GPUs
```

### C. Intel oneAPI + OpenVINO

**Source:** `github.com/openvinotoolkit/openvino`

For Intel integrated GPUs, OpenVINO provides optimized inference:

```python
from openvino.runtime import Core

class OpenVINOInference:
    def __init__(self, model_path, device="GPU"):
        self.ie = Core()
        self.model = self.ie.read_model(model_path)
        self.compiled_model = self.ie.compile_model(self.model, device)
    
    def infer(self, input_data):
        return self.compiled_model.create_infer_request().infer(
            {0: input_data}
        )
```

### D. FFmpeg Filter Graph for Complex Processing

```python
# FFmpeg filter graph for real-time screen processing
import ffmpeg

# Create complex filter graph
filter_graph = (
    ffmpeg
    .input('desktop', f='gdigrab', framerate=60)
    .filter('hwupload_cuda')  # Upload to GPU
    .filter('scale_npp', 1920, 1080)  # GPU resize
    .filter('eq', saturation=1.2, contrast=1.1)  # GPU color adjust
    .filter('hwdownload')  # Back to CPU for encoding
    .output('output.mp4', vcodec='h264_nvenc', preset='p4')
)

filter_graph.run()
```

### E. PipeWire for Modern Linux Capture

**Source:** `pipewire.org`, `docs.pipewire.org`

```c
// PipeWire stream capture for Wayland/X11
#include <pipewire/pipewire.h>
#include <spa/param/video/format-utils.h>

class PipeWireCapture {
    // Zero-copy capture on modern Linux
    // Supports both X11 and Wayland
    // DMA-BUF sharing with Vulkan/OpenGL
};
```

### F. GStreamer Pipeline for Mobile/Embedded

```python
# GStreamer pipeline for mobile device capture
import gi
gi.require_version('Gst', '1.0')
from gi.repository import Gst

class GStreamerMobileCapture:
    def __init__(self):
        Gst.init(None)
        
        # Pipeline: v4l2src (Android USB camera) → h264parse → decode → appsink
        self.pipeline = Gst.parse_launch(
            "v4l2src device=/dev/video0 ! "
            "video/x-h264,width=1920,height=1080,framerate=30/1 ! "
            "h264parse ! "
            "nvdec (for NVIDIA) or vaapih264dec (for Intel/AMD) ! "
            "videoconvert ! "
            "video/x-raw,format=BGR ! "
            "appsink name=sink"
        )
```

---

## Appendix C: Complete Technology Stack Summary

### Core Technologies (37 total)

| # | Technology | Category | Purpose | Platform |
|---|-----------|----------|---------|----------|
| 1 | OpenCV (CPU) | Vision | Image processing | All |
| 2 | OpenCV (CUDA) | Vision | GPU image processing | NVIDIA |
| 3 | OpenCV (OpenCL) | Vision | Cross-vendor GPU | All GPU |
| 4 | Vulkan Compute | GPU Compute | Shader-based processing | All GPU |
| 5 | OpenGL Compute | GPU Compute | Legacy GPU compute | All GPU |
| 6 | CUDA | GPU Compute | NVIDIA compute | NVIDIA |
| 7 | TensorRT | Inference | Optimized inference | NVIDIA |
| 8 | NVIDIA Maxine | Video AI | Video enhancement | NVIDIA RTX |
| 9 | NVFBC | Capture | Framebuffer capture | NVIDIA |
| 10 | NVENC | Encoding | Hardware encoding | NVIDIA |
| 11 | AMD ROCm | GPU Compute | AMD GPU compute | AMD |
| 12 | AMF | Encoding | AMD hardware encoding | AMD |
| 13 | Intel oneAPI | GPU Compute | Intel GPU compute | Intel |
| 14 | OpenVINO | Inference | Intel optimized inference | Intel |
| 15 | QuickSync | Encoding | Intel hardware encoding | Intel |
| 16 | VAAPI | Encoding | Linux hardware encoding | Intel/AMD |
| 17 | DXGI Desktop Duplication | Capture | Windows screen capture | Windows |
| 18 | Windows Graphics Capture | Capture | Modern Windows capture | Windows 10+ |
| 19 | GDI BitBlt | Capture | Legacy Windows capture | Windows |
| 20 | KMS/DRM | Capture | Linux kernel capture | Linux |
| 21 | DMA-BUF | Memory | Zero-copy GPU sharing | Linux |
| 22 | PipeWire | Capture | Modern Linux capture | Linux |
| 23 | ScreenCaptureKit | Capture | macOS capture | macOS 12.3+ |
| 24 | IOSurface | Memory | macOS GPU sharing | macOS |
| 25 | evdev | Input | Linux raw input | Linux |
| 26 | uinput | Input | Linux virtual input | Linux |
| 27 | SendInput | Input | Windows input | Windows |
| 28 | FakerInput | Input | Undetectable Windows input | Windows |
| 29 | CGEventPost | Input | macOS input | macOS |
| 30 | AXUIElement | Input | macOS accessibility input | macOS |
| 31 | LD_PRELOAD | Hooking | Linux API interception | Linux |
| 32 | plthook | Hooking | Runtime PLT hooking | Linux |
| 33 | SetWindowsHookEx | Hooking | Windows input hooks | Windows |
| 34 | FFmpeg | Recording | Universal media framework | All |
| 35 | GStreamer | Recording | Pipeline-based media | All |
| 36 | OBS libobs | Recording | Professional recording | All |
| 37 | WebRTC | Streaming | Sub-second streaming | All |

### Mobile Technologies (6)

| # | Technology | Platform | Purpose |
|---|-----------|----------|---------|
| 38 | scrcpy | Android | Screen mirror + control |
| 39 | ADB | Android | Device communication |
| 40 | UIAutomator2 | Android | Native automation |
| 41 | WebDriverAgent | iOS | WebDriver for iOS |
| 42 | XCUITest | iOS | Native iOS testing |
| 43 | go-ios | iOS | iOS device management |

### TUI Technologies (4)

| # | Technology | Purpose |
|---|-----------|---------|
| 44 | node-pty | Pseudoterminal spawning |
| 45 | xterm-headless | Terminal rendering |
| 46 | pilotty | TUI automation framework |
| 47 | tmux control mode | Terminal multiplexer control |

---

## Appendix D: Recommended Hardware Configurations

### Minimum Configuration (Development)

| Component | Specification |
|-----------|--------------|
| CPU | 4 cores (Intel/AMD) |
| GPU | GTX 1060 6GB or RX 580 |
| RAM | 16GB |
| Storage | 256GB SSD |
| OS | Ubuntu 22.04 / Windows 11 / macOS 13 |

### Recommended Configuration (Production)

| Component | Specification |
|-----------|--------------|
| CPU | 8+ cores (Intel i7/Ryzen 7) |
| GPU | RTX 4070 12GB or better |
| RAM | 32GB |
| Storage | 1TB NVMe SSD |
| Network | Gigabit Ethernet |
| OS | Ubuntu 24.04 LTS |

### Maximum Performance (Enterprise)

| Component | Specification |
|-----------|--------------|
| CPU | 16+ cores (Threadripper/Xeon) |
| GPU | RTX 4090 24GB or A6000 |
| RAM | 64GB |
| Storage | 2TB NVMe RAID |
| Network | 10Gbps |
| Capture | Dedicated capture card (Magewell/Blackmagic) |

---

*End of OpenClaw Ultimate Capabilities Extension Document*
*Version: 1.0*
*Total Technologies: 47*
*Total Source Modules Referenced: 35+*
*Total Implementation Steps: 8 Phases, 48 Steps*
