#!/bin/bash
# setup-video-host.sh
# Prepares a host for HelixQA video processing workloads
# Supports Ubuntu, Debian, Fedora, macOS

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
OLLAMA_MODELS="llava:13b-v1.6 qwen2-vl:latest bakllava:latest"
MIN_GO_VERSION="1.21"
MIN_PYTHON_VERSION="3.9"

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     HelixQA Video Processing Host Setup                      ║"
echo "║     Open Source - Zero Cost Vision Pipeline                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Detect OS
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if [ -f /etc/os-release ]; then
            . /etc/os-release
            OS=$NAME
            OS_VERSION=$VERSION_ID
            OS_TYPE="debian"
            
            if [[ "$ID" == "fedora" ]] || [[ "$ID_LIKE" == *"fedora"* ]] || [[ "$ID" == "rhel" ]] || [[ "$ID" == "centos" ]]; then
                OS_TYPE="fedora"
            fi
        else
            OS="linux"
            OS_TYPE="unknown"
        fi
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macOS"
        OS_TYPE="macos"
        OS_VERSION=$(sw_vers -productVersion)
    else
        log_error "Unsupported OS: $OSTYPE"
        exit 1
    fi
    
    log_info "Detected OS: $OS ($OS_VERSION)"
}

# Check if running as root (warn, don't exit - we prefer rootless)
check_root() {
    if [ "$EUID" -eq 0 ]; then
        log_warn "Running as root. Consider running as regular user for rootless containers."
    fi
}

# Install Podman (preferred) or Docker
install_containers() {
    log_info "Installing container runtime..."
    
    if command -v podman &> /dev/null; then
        log_success "Podman already installed: $(podman --version)"
        return 0
    fi
    
    if command -v docker &> /dev/null; then
        log_warn "Docker found, but Podman is preferred for rootless operation"
        log_info "Will continue with Docker, but consider migrating to Podman"
        return 0
    fi
    
    case $OS_TYPE in
        debian)
            log_info "Installing Podman on Debian/Ubuntu..."
            sudo apt-get update
            sudo apt-get install -y podman podman-compose slirp4netns uidmap
            
            # Enable rootless mode
            if [ "$EUID" -ne 0 ]; then
                log_info "Configuring rootless Podman..."
                podman system migrate || true
            fi
            ;;
            
        fedora)
            log_info "Installing Podman on Fedora/RHEL..."
            sudo dnf install -y podman podman-compose slirp4netns
            ;;
            
        macos)
            if command -v brew &> /dev/null; then
                log_info "Installing Podman via Homebrew..."
                brew install podman podman-compose
                
                # Initialize podman machine
                log_info "Initializing Podman machine..."
                podman machine init || true
                podman machine start || true
            else
                log_error "Homebrew not found. Please install Homebrew first."
                exit 1
            fi
            ;;
            
        *)
            log_error "Cannot install Podman on this OS automatically"
            exit 1
            ;;
    esac
    
    if command -v podman &> /dev/null; then
        log_success "Podman installed: $(podman --version)"
    elif command -v docker &> /dev/null; then
        log_success "Docker available: $(docker --version)"
    else
        log_error "Failed to install container runtime"
        exit 1
    fi
}

# Install GStreamer
install_gstreamer() {
    log_info "Installing GStreamer..."
    
    if command -v gst-launch-1.0 &> /dev/null; then
        log_success "GStreamer already installed: $(gst-launch-1.0 --version | head -1)"
        return 0
    fi
    
    case $OS_TYPE in
        debian)
            sudo apt-get update
            sudo apt-get install -y \
                libgstreamer1.0-0 \
                gstreamer1.0-plugins-base \
                gstreamer1.0-plugins-good \
                gstreamer1.0-plugins-bad \
                gstreamer1.0-plugins-ugly \
                gstreamer1.0-libav \
                gstreamer1.0-tools \
                gstreamer1.0-x \
                gstreamer1.0-alsa \
                gstreamer1.0-gl \
                gstreamer1.0-gtk3 \
                libgstreamer-plugins-base1.0-dev \
                libgstreamer1.0-dev
            ;;
            
        fedora)
            sudo dnf install -y \
                gstreamer1 \
                gstreamer1-plugins-base \
                gstreamer1-plugins-good \
                gstreamer1-plugins-bad-free \
                gstreamer1-plugins-bad-nonfree \
                gstreamer1-plugins-ugly-free \
                gstreamer1-libav \
                gstreamer1-devel
            ;;
            
        macos)
            brew install gstreamer gst-plugins-base gst-plugins-good \
                        gst-plugins-bad gst-plugins-ugly gst-libav
            ;;
    esac
    
    if command -v gst-launch-1.0 &> /dev/null; then
        log_success "GStreamer installed successfully"
    else
        log_warn "GStreamer installation may have failed"
    fi
}

# Install OpenCV dependencies
install_opencv_deps() {
    log_info "Installing OpenCV dependencies..."
    
    case $OS_TYPE in
        debian)
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
                libtbb2 \
                libtbb-dev \
                libjpeg-dev \
                libpng-dev \
                libtiff-dev \
                libatlas-base-dev
            ;;
            
        fedora)
            sudo dnf install -y \
                gcc-c++ \
                cmake \
                git \
                gtk3-devel \
                ffmpeg-devel \
                libv4l-devel \
                libjpeg-turbo-devel \
                libpng-devel \
                libtiff-devel \
                tbb-devel
            ;;
            
        macos)
            brew install cmake git gtk+3 jpeg libpng libtiff
            ;;
    esac
    
    log_success "OpenCV dependencies installed"
}

# Install Go
install_go() {
    log_info "Installing Go..."
    
    if command -v go &> /dev/null; then
        GO_CURRENT=$(go version | awk '{print $3}' | sed 's/go//')
        log_info "Go already installed: $GO_CURRENT"
        
        # Check version
        if [ "$(printf '%s\n' "$MIN_GO_VERSION" "$GO_CURRENT" | sort -V | head -n1)" = "$MIN_GO_VERSION" ]; then
            log_success "Go version is sufficient"
            return 0
        else
            log_warn "Go version $GO_CURRENT is older than required $MIN_GO_VERSION"
        fi
    fi
    
    # Download and install latest Go
    GO_VERSION="1.22.0"
    case $(uname -m) in
        x86_64)
            GO_ARCH="amd64"
            ;;
        aarch64|arm64)
            GO_ARCH="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    
    case $OS_TYPE in
        linux*)
            GO_OS="linux"
            ;;
        macos)
            GO_OS="darwin"
            ;;
    esac
    
    log_info "Downloading Go $GO_VERSION..."
    curl -L "https://go.dev/dl/go${GO_VERSION}.${GO_OS}-${GO_ARCH}.tar.gz" -o /tmp/go.tar.gz
    
    log_info "Installing Go..."
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    
    # Add to PATH if not already there
    if ! grep -q "/usr/local/go/bin" "$HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> "$HOME/.bashrc"
        log_info "Added Go to PATH in .bashrc"
    fi
    
    export PATH=$PATH:/usr/local/go/bin
    
    if command -v go &> /dev/null; then
        log_success "Go installed: $(go version)"
    else
        log_error "Go installation failed"
        exit 1
    fi
}

# Install Python and dependencies
install_python() {
    log_info "Installing Python..."
    
    case $OS_TYPE in
        debian)
            sudo apt-get install -y python3 python3-pip python3-venv python3-dev
            
            # Install PaddleOCR dependencies
            log_info "Installing PaddleOCR Python dependencies..."
            pip3 install --user --upgrade pip
            pip3 install --user paddlepaddle paddleocr grpcio protobuf pillow numpy
            ;;
            
        fedora)
            sudo dnf install -y python3 python3-pip python3-devel
            pip3 install --user --upgrade pip
            pip3 install --user paddlepaddle paddleocr grpcio protobuf pillow numpy
            ;;
            
        macos)
            brew install python@3.11
            pip3 install --upgrade pip
            pip3 install paddlepaddle paddleocr grpcio protobuf pillow numpy
            ;;
    esac
    
    if command -v python3 &> /dev/null; then
        PYTHON_VERSION=$(python3 --version | awk '{print $2}')
        log_success "Python installed: $PYTHON_VERSION"
    fi
}

# Install Tesseract OCR
install_tesseract() {
    log_info "Installing Tesseract OCR..."
    
    if command -v tesseract &> /dev/null; then
        log_success "Tesseract already installed: $(tesseract --version | head -1)"
        return 0
    fi
    
    case $OS_TYPE in
        debian)
            sudo apt-get install -y tesseract-ocr tesseract-ocr-eng libtesseract-dev libleptonica-dev
            # Install additional language packs as needed
            # sudo apt-get install -y tesseract-ocr-deu tesseract-ocr-fra
            ;;
            
        fedora)
            sudo dnf install -y tesseract tesseract-langpack-eng tesseract-devel leptonica-devel
            ;;
            
        macos)
            brew install tesseract
            ;;
    esac
    
    if command -v tesseract &> /dev/null; then
        log_success "Tesseract installed: $(tesseract --version | head -1)"
    else
        log_warn "Tesseract installation may have failed"
    fi
}

# Detect and configure GPU
detect_gpu() {
    log_info "Detecting GPU..."
    
    GPU_FOUND=false
    
    # Check for NVIDIA
    if command -v nvidia-smi &> /dev/null; then
        log_success "NVIDIA GPU detected:"
        nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader
        GPU_FOUND=true
        
        # Install nvidia-container-toolkit for container GPU access
        if [ "$OS_TYPE" = "debian" ] && [ "$EUID" -eq 0 ]; then
            log_info "Installing NVIDIA Container Toolkit..."
            distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
            curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add - 2>/dev/null || true
            curl -s -L "https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list" | \
                sudo tee /etc/apt/sources.list.d/nvidia-docker.list
            sudo apt-get update
            sudo apt-get install -y nvidia-container-toolkit || true
        fi
    fi
    
    # Check for AMD ROCm
    if command -v rocm-smi &> /dev/null; then
        log_success "AMD GPU detected:"
        rocm-smi --showproductname 2>/dev/null || true
        GPU_FOUND=true
    fi
    
    # Check for Intel GPU
    if [ -d /sys/class/drm ] && grep -q "i915" /sys/class/drm/*/device/driver/name 2>/dev/null; then
        log_success "Intel GPU detected"
        GPU_FOUND=true
    fi
    
    if [ "$GPU_FOUND" = false ]; then
        log_warn "No GPU detected. CPU-only mode will be used."
    fi
}

# Install Ollama
install_ollama() {
    log_info "Installing Ollama..."
    
    if command -v ollama &> /dev/null; then
        log_success "Ollama already installed: $(ollama --version)"
        
        # Check if already running
        if curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
            log_success "Ollama is running"
        else
            log_warn "Ollama installed but not running. Start with: ollama serve"
        fi
        
        return 0
    fi
    
    log_info "Downloading and installing Ollama..."
    curl -fsSL https://ollama.com/install.sh | sh
    
    if command -v ollama &> /dev/null; then
        log_success "Ollama installed"
        
        # Create systemd service or launchd plist
        if [ "$OS_TYPE" != "macos" ] && [ "$EUID" -eq 0 ]; then
            log_info "Creating Ollama systemd service..."
            
            sudo tee /etc/systemd/system/ollama.service > /dev/null << 'EOF'
[Unit]
Description=Ollama Service
After=network-online.target

[Service]
ExecStart=/usr/local/bin/ollama serve
User=ollama
Group=ollama
Restart=always
RestartSec=3
Environment="OLLAMA_KEEP_ALIVE=24h"

[Install]
WantedBy=default.target
EOF
            
            sudo systemctl daemon-reload
            sudo systemctl enable ollama
            sudo systemctl start ollama
        else
            log_info "Start Ollama manually with: ollama serve"
        fi
    else
        log_error "Ollama installation failed"
        return 1
    fi
}

# Pull vision models
pull_vision_models() {
    log_info "Pulling vision models..."
    
    # Check if Ollama is running
    if ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
        log_warn "Ollama not running. Starting temporarily..."
        ollama serve &
        OLLAMA_PID=$!
        sleep 5
    fi
    
    for model in $OLLAMA_MODELS; do
        log_info "Pulling model: $model"
        ollama pull "$model" || log_warn "Failed to pull $model"
    done
    
    # Kill temporary Ollama if we started it
    if [ -n "$OLLAMA_PID" ]; then
        kill $OLLAMA_PID 2>/dev/null || true
    fi
    
    log_success "Vision models ready"
}

# Install scrcpy for Android capture
install_scrcpy() {
    log_info "Installing scrcpy..."
    
    if command -v scrcpy &> /dev/null; then
        log_success "scrcpy already installed: $(scrcpy --version | head -1)"
        return 0
    fi
    
    case $OS_TYPE in
        debian)
            sudo apt-get install -y scrcpy adb
            ;;
        fedora)
            sudo dnf install -y scrcpy android-tools
            ;;
        macos)
            brew install scrcpy android-platform-tools
            ;;
    esac
    
    if command -v scrcpy &> /dev/null; then
        log_success "scrcpy installed"
    else
        log_warn "scrcpy installation may have failed. Install manually if needed."
    fi
}

# Create necessary directories
setup_directories() {
    log_info "Setting up directories..."
    
    mkdir -p "$HOME/.helixqa"
    mkdir -p "$HOME/.helixqa/models"
    mkdir -p "$HOME/.helixqa/recordings"
    mkdir -p "$HOME/.helixqa/cache"
    
    log_success "Directories created"
}

# Verify installation
verify_installation() {
    log_info "Verifying installation..."
    
    local FAIL=0
    
    # Check container runtime
    if command -v podman &> /dev/null || command -v docker &> /dev/null; then
        log_success "✓ Container runtime available"
    else
        log_error "✗ Container runtime not found"
        FAIL=1
    fi
    
    # Check GStreamer
    if command -v gst-launch-1.0 &> /dev/null; then
        log_success "✓ GStreamer available"
    else
        log_warn "✗ GStreamer not found"
    fi
    
    # Check Go
    if command -v go &> /dev/null; then
        log_success "✓ Go available: $(go version | awk '{print $3}')"
    else
        log_error "✗ Go not found"
        FAIL=1
    fi
    
    # Check Python
    if command -v python3 &> /dev/null; then
        log_success "✓ Python available: $(python3 --version)"
    else
        log_error "✗ Python not found"
        FAIL=1
    fi
    
    # Check Tesseract
    if command -v tesseract &> /dev/null; then
        log_success "✓ Tesseract OCR available"
    else
        log_warn "✗ Tesseract not found"
    fi
    
    # Check Ollama
    if command -v ollama &> /dev/null; then
        log_success "✓ Ollama available"
    else
        log_warn "✗ Ollama not found"
    fi
    
    # Check scrcpy
    if command -v scrcpy &> /dev/null; then
        log_success "✓ scrcpy available"
    else
        log_warn "✗ scrcpy not found (Android capture will not work)"
    fi
    
    if [ $FAIL -eq 0 ]; then
        log_success "All critical components installed successfully!"
        return 0
    else
        log_error "Some critical components are missing. Please review errors above."
        return 1
    fi
}

# Print summary
print_summary() {
    echo ""
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║     Setup Complete!                                          ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo ""
    echo "Installed Components:"
    echo "  • Container runtime (Podman/Docker)"
    echo "  • GStreamer (video streaming)"
    echo "  • OpenCV dependencies"
    echo "  • Go $(go version 2>/dev/null | awk '{print $3}' || echo 'N/A')"
    echo "  • Python $(python3 --version 2>/dev/null | awk '{print $2}' || echo 'N/A')"
    echo "  • Tesseract OCR"
    echo "  • Ollama with vision models"
    echo "  • scrcpy (Android capture)"
    echo ""
    
    echo "GPU Status:"
    if command -v nvidia-smi &> /dev/null; then
        echo "  ✓ NVIDIA GPU detected"
        nvidia-smi --query-gpu=name,memory.total --format=csv,noheader | head -1
    elif command -v rocm-smi &> /dev/null; then
        echo "  ✓ AMD GPU detected"
    else
        echo "  ⚠ No GPU detected (CPU-only mode)"
    fi
    echo ""
    
    echo "Next Steps:"
    echo "  1. Start Ollama: ollama serve"
    echo "  2. Verify: curl http://localhost:11434/api/tags"
    echo "  3. Test scan: cd HelixQA && go test ./pkg/discovery/..."
    echo "  4. Run discovery: go run cmd/discover/main.go"
    echo ""
    echo "For distributed processing, run this script on all hosts."
    echo ""
}

# Main installation flow
main() {
    detect_os
    check_root
    
    log_info "Starting installation..."
    
    install_containers
    install_gstreamer
    install_opencv_deps
    install_go
    install_python
    install_tesseract
    detect_gpu
    install_ollama
    pull_vision_models
    install_scrcpy
    setup_directories
    
    verify_installation
    print_summary
}

# Handle command line arguments
case "${1:-}" in
    --help|-h)
        echo "HelixQA Video Host Setup"
        echo ""
        echo "Usage: $0 [options]"
        echo ""
        echo "Options:"
        echo "  --help, -h      Show this help message"
        echo "  --verify        Only verify existing installation"
        echo "  --skip-gpu      Skip GPU detection and setup"
        echo "  --skip-ollama   Skip Ollama installation"
        echo ""
        exit 0
        ;;
        
    --verify)
        verify_installation
        exit $?
        ;;
        
    --skip-gpu)
        SKIP_GPU=1
        main
        ;;
        
    --skip-ollama)
        SKIP_OLLAMA=1
        main
        ;;
        
    *)
        main
        ;;
esac
