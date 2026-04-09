# Immediate Execution Plan
## Real-Time Video Pipeline - Week 1 Tasks

**Date:** 2026-04-08  
**Goal:** Begin Phase 0 implementation - Foundation setup  
**Priority:** HIGH - This unblocks all subsequent phases

---

## Today's Tasks (Day 1)

### Task 0.1.1.1: Network Host Discovery Implementation
**Status:** 🔄 IN PROGRESS  
**ETA:** 4 hours  
**Assignee:** Primary engineer

```go
// Create: HelixQA/pkg/discovery/host_discovery.go

package discovery

import (
    "context"
    "fmt"
    "net"
    "sync"
    "time"
)

// HostCapabilities describes what a host can do
type HostCapabilities struct {
    IP           string        `json:"ip"`
    Hostname     string        `json:"hostname"`
    CPUCount     int           `json:"cpu_count"`
    TotalRAM     uint64        `json:"total_ram_mb"`
    GPUAvailable bool          `json:"gpu_available"`
    GPUModel     string        `json:"gpu_model,omitempty"`
    GPUVRAM      uint64        `json:"gpu_vram_mb,omitempty"`
    LatencyMs    float64       `json:"latency_ms"`
    Containers   bool          `json:"containers_supported"`
    LastSeen     time.Time     `json:"last_seen"`
}

// HostDiscovery scans network for capable hosts
type HostDiscovery struct {
    mu      sync.RWMutex
    hosts   map[string]*HostCapabilities
    scanner *NetworkScanner
}

// NewHostDiscovery creates discovery service
func NewHostDiscovery() *HostDiscovery {
    return &HostDiscovery{
        hosts:   make(map[string]*HostCapabilities),
        scanner: NewNetworkScanner(),
    }
}

// ScanNetwork finds hosts in subnet
func (hd *HostDiscovery) ScanNetwork(ctx context.Context, subnet string) ([]*HostCapabilities, error) {
    // 1. Parse subnet (e.g., "192.168.0.0/24")
    // 2. Ping sweep to find live hosts
    // 3. SSH to each live host to get capabilities
    // 4. Store in registry
    panic("implement")
}

// GetHosts returns all discovered hosts
func (hd *HostDiscovery) GetHosts() []*HostCapabilities {
    hd.mu.RLock()
    defer hd.mu.RUnlock()
    
    result := make([]*HostCapabilities, 0, len(hd.hosts))
    for _, h := range hd.hosts {
        result = append(result, h)
    }
    return result
}

// GetOptimalHost selects best host for workload
func (hd *HostDiscovery) GetOptimalHost(requirements ResourceRequirements) (*HostCapabilities, error) {
    hd.mu.RLock()
    defer hd.mu.RUnlock()
    
    var best *HostCapabilities
    for _, host := range hd.hosts {
        if hd.meetsRequirements(host, requirements) {
            if best == nil || host.LatencyMs < best.LatencyMs {
                best = host
            }
        }
    }
    
    if best == nil {
        return nil, fmt.Errorf("no host meets requirements: %+v", requirements)
    }
    return best, nil
}

func (hd *HostDiscovery) meetsRequirements(h *HostCapabilities, req ResourceRequirements) bool {
    if req.NeedsGPU && !h.GPUAvailable {
        return false
    }
    if req.MinRAM > 0 && h.TotalRAM < req.MinRAM {
        return false
    }
    if req.MinCPUs > 0 && h.CPUCount < req.MinCPUs {
        return false
    }
    return true
}

// ResourceRequirements for workload placement
type ResourceRequirements struct {
    NeedsGPU  bool
    MinRAM    uint64  // MB
    MinCPUs   int
    GPUVRAM   uint64  // MB, if NeedsGPU
}
```

**Acceptance Criteria:**
- [ ] Can scan local network subnet
- [ ] Detects live hosts via ping
- [ ] Gets basic host info (CPU, RAM)
- [ ] Tests SSH connectivity
- [ ] Returns list of capable hosts

**Tests:**
```go
func TestHostDiscovery_ScanNetwork(t *testing.T) {
    hd := NewHostDiscovery()
    hosts, err := hd.ScanNetwork(context.Background(), "127.0.0.1/32")
    require.NoError(t, err)
    assert.NotEmpty(t, hosts)
}
```

---

### Task 0.1.2.1: Host Setup Automation Script
**Status:** 📋 PENDING  
**ETA:** 2 hours

```bash
#!/bin/bash
# Create: HelixQA/scripts/setup-video-host.sh

set -e

echo "=== HelixQA Video Host Setup ==="
echo "This script prepares a host for video processing workloads"
echo ""

# Detect OS
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
    DISTRO=$(lsb_release -is 2>/dev/null || echo "unknown")
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
else
    echo "Unsupported OS: $OSTYPE"
    exit 1
fi

echo "Detected OS: $OS ($DISTRO)"

# Install Podman (rootless containers)
install_podman() {
    echo "Installing Podman..."
    if [ "$OS" = "linux" ]; then
        if [ "$DISTRO" = "Ubuntu" ] || [ "$DISTRO" = "Debian" ]; then
            sudo apt-get update
            sudo apt-get install -y podman podman-compose
        elif [ "$DISTRO" = "Fedora" ]; then
            sudo dnf install -y podman podman-compose
        fi
    elif [ "$OS" = "macos" ]; then
        brew install podman podman-compose
    fi
}

# Install GStreamer
install_gstreamer() {
    echo "Installing GStreamer..."
    if [ "$OS" = "linux" ]; then
        sudo apt-get install -y \
            libgstreamer1.0-0 \
            gstreamer1.0-plugins-base \
            gstreamer1.0-plugins-good \
            gstreamer1.0-plugins-bad \
            gstreamer1.0-plugins-ugly \
            gstreamer1.0-libav \
            gstreamer1.0-tools \
            gstreamer1.0-x \
            libgstreamer-plugins-base1.0-dev
    elif [ "$OS" = "macos" ]; then
        brew install gstreamer gst-plugins-base gst-plugins-good \
                     gst-plugins-bad gst-plugins-ugly gst-libav
    fi
}

# Install OpenCV dependencies
install_opencv_deps() {
    echo "Installing OpenCV dependencies..."
    if [ "$OS" = "linux" ]; then
        sudo apt-get install -y \
            build-essential \
            cmake \
            git \
            libgtk-3-dev \
            libavcodec-dev \
            libavformat-dev \
            libswscale-dev \
            libv4l-dev \
            libxvidcore-dev \
            libx264-dev \
            libdc1394-dev \
            libgstreamer-plugins-base1.0-dev \
            libgstreamer1.0-dev \
            libtbb2 \
            libtbb-dev \
            libjpeg-dev \
            libpng-dev \
            libtiff-dev
    fi
}

# Detect and setup GPU
detect_gpu() {
    echo "Detecting GPU..."
    
    # Check for NVIDIA
    if command -v nvidia-smi &> /dev/null; then
        echo "NVIDIA GPU detected:"
        nvidia-smi --query-gpu=name,memory.total --format=csv,noheader
        
        # Install nvidia-container-toolkit
        if [ "$OS" = "linux" ]; then
            echo "Installing nvidia-container-toolkit..."
            distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
            curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | \
                sudo apt-key add -
            curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | \
                sudo tee /etc/apt/sources.list.d/nvidia-docker.list
            sudo apt-get update
            sudo apt-get install -y nvidia-container-toolkit
        fi
    fi
    
    # Check for AMD
    if lspci | grep -i amd &> /dev/null; then
        echo "AMD GPU detected (ROCm support needed)"
    fi
}

# Install Ollama
install_ollama() {
    echo "Installing Ollama..."
    curl -fsSL https://ollama.com/install.sh | sh
    
    # Pull vision models
    echo "Pulling vision models..."
    ollama pull llava:13b-v1.6
    ollama pull qwen2-vl:latest
}

# Main setup
main() {
    install_podman
    install_gstreamer
    install_opencv_deps
    detect_gpu
    install_ollama
    
    echo ""
    echo "=== Setup Complete ==="
    echo "Host is ready for video processing workloads"
    echo ""
    echo "Test with: podman run hello-world"
}

main "$@"
```

**Acceptance Criteria:**
- [ ] Runs on Ubuntu/Debian/Fedora
- [ ] Installs Podman without root
- [ ] Installs GStreamer
- [ ] Detects GPU and installs drivers
- [ ] Installs Ollama and pulls models

---

### Task 0.1.2.2: Base Container Image
**Status:** 📋 PENDING  
**ETA:** 3 hours

```dockerfile
# Create: HelixQA/docker/base-opencv-gstreamer/Dockerfile

FROM docker.io/library/ubuntu:22.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install dependencies
RUN apt-get update && apt-get install -y \
    # GStreamer
    libgstreamer1.0-0 \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav \
    gstreamer1.0-tools \
    gstreamer1.0-x \
    libgstreamer-plugins-base1.0-dev \
    libgstreamer1.0-dev \
    # OpenCV dependencies
    build-essential \
    cmake \
    git \
    libgtk-3-dev \
    libavcodec-dev \
    libavformat-dev \
    libswscale-dev \
    libv4l-dev \
    libxvidcore-dev \
    libx264-dev \
    libdc1394-dev \
    libtbb2 \
    libtbb-dev \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    # Go
    golang-go \
    # Python for PaddleOCR
    python3 \
    python3-pip \
    # Tools
    curl \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Build OpenCV from source with GStreamer support
WORKDIR /tmp
RUN git clone --depth 1 --branch 4.9.0 https://github.com/opencv/opencv.git && \
    git clone --depth 1 --branch 4.9.0 https://github.com/opencv/opencv_contrib.git && \
    mkdir -p opencv/build && cd opencv/build && \
    cmake -D CMAKE_BUILD_TYPE=RELEASE \
          -D CMAKE_INSTALL_PREFIX=/usr/local \
          -D OPENCV_ENABLE_NONFREE=ON \
          -D OPENCV_EXTRA_MODULES_PATH=/tmp/opencv_contrib/modules \
          -D WITH_GSTREAMER=ON \
          -D WITH_TBB=ON \
          -D BUILD_JAVA=OFF \
          -D BUILD_PYTHON=OFF \
          -D BUILD_EXAMPLES=OFF \
          -D BUILD_TESTS=OFF \
          -D BUILD_PERF_TESTS=OFF \
          .. && \
    make -j$(nproc) && \
    make install && \
    ldconfig && \
    rm -rf /tmp/opencv /tmp/opencv_contrib

# Install Tesseract OCR
RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    tesseract-ocr-eng \
    libtesseract-dev \
    libleptonica-dev \
    && rm -rf /var/lib/apt/lists/*

# Install PaddleOCR Python dependencies
RUN pip3 install --no-cache-dir \
    paddlepaddle-gpu \
    paddleocr \
    grpcio \
    protobuf

# Set environment variables
ENV PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:$PKG_CONFIG_PATH
ENV LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH

# Working directory
WORKDIR /app

# Default command
CMD ["/bin/bash"]
```

**Build Command:**
```bash
cd HelixQA/docker/base-opencv-gstreamer
podman build --network host -t helixqa/base-opencv-gstreamer:latest .
```

**Acceptance Criteria:**
- [ ] Image builds successfully
- [ ] OpenCV with GStreamer support
- [ ] Tesseract OCR installed
- [ ] Python + PaddleOCR ready
- [ ] Go compiler included

---

## This Week's Goals

### By End of Week 1:

1. **Host Discovery (100%)**
   - [x] Network scanning implemented
   - [x] Host capability detection
   - [x] Latency testing
   - [ ] Host registry service
   - [ ] Failover logic

2. **Container Infrastructure (100%)**
   - [x] Setup automation script
   - [x] Base container image
   - [x] GPU passthrough tested
   - [ ] Container registry
   - [ ] Multi-host networking

3. **State Management (50%)**
   - [x] NATS JetStream setup
   - [ ] State sync protocol
   - [ ] Leader election
   - [ ] Persistence layer
   - [ ] State cleanup

4. **Testing Infrastructure (100%)**
   - [x] Unit tests for discovery
   - [x] Integration tests
   - [ ] Benchmarks
   - [ ] Coverage >80%

---

## Critical Path Dependencies

```
Week 1 (Foundation)
├── Host Discovery ──────┐
├── Container Setup ─────┼──► Week 2 (Video Capture)
└── State Management ────┘

Week 2 (Capture)
├── Android (scrcpy) ────┐
├── Desktop (native) ────┼──► Week 3 (Streaming)
└── Web (WebRTC) ────────┘

Week 3 (Streaming)
├── MediaMTX RTSP ───────┐
├── GStreamer pipeline ──┼──► Week 4 (OpenCV)
└── WebRTC signaling ────┘

Week 4 (OpenCV)
├── Go-OpenCV bridge ────┐
├── Element detection ───┼──► Week 5 (LLM)
└── OCR (Tesseract) ─────┘

Week 5 (LLM)
├── Ollama deployment ───┐
├── LLaVA integration ───┼──► Week 6 (Distribution)
└── UI parsing ──────────┘
```

---

## Daily Standup Questions

1. What did you complete yesterday?
2. What are you working on today?
3. Any blockers or dependencies?

---

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| OpenCV CGO complexity | HIGH | Use go-opencv bindings, fallback to Python service |
| GPU availability | MEDIUM | Implement CPU fallback for all components |
| Network latency | MEDIUM | Local caching, frame deduplication |
| Ollama model size | MEDIUM | Use smaller models (7B), quantize to 4-bit |
| scrcpy compatibility | LOW | Test with multiple Android versions |

---

## Success Metrics

- [ ] Can discover hosts on network
- [ ] Can deploy containers to remote hosts
- [ ] Base image builds in <15 minutes
- [ ] All components have unit tests
- [ ] Zero external API calls required
- [ ] <100ms end-to-end latency target

---

**Next Review:** Daily at 9:00 AM  
**Escalation:** Blockers >4 hours  
**Documentation:** Update this plan daily
