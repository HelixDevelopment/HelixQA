#!/bin/bash
# Entrypoint for HelixQA base container

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Print banner
cat << 'EOF'
╔══════════════════════════════════════════════════════════════╗
║     HelixQA Video Processing Container                       ║
║     OpenCV + GStreamer + Tesseract + PaddleOCR              ║
╚══════════════════════════════════════════════════════════════╝
EOF

# Verify GPU availability
check_gpu() {
    log_info "Checking GPU availability..."
    
    if command -v nvidia-smi &> /dev/null; then
        log_success "NVIDIA GPU detected:"
        nvidia-smi --query-gpu=name,memory.total --format=csv,noheader
        
        # Verify CUDA is working
        if python3 -c "import paddle; paddle.utils.run_check()" 2>/dev/null; then
            log_success "PaddlePaddle GPU check passed"
        else
            log_warn "PaddlePaddle GPU check failed, will use CPU"
        fi
    else
        log_warn "No NVIDIA GPU detected, running in CPU mode"
    fi
}

# Verify OpenCV installation
check_opencv() {
    log_info "Verifying OpenCV installation..."
    
    if python3 -c "import cv2; print(f'OpenCV {cv2.__version__}')" 2>/dev/null; then
        log_success "OpenCV is available"
        
        # Check CUDA support
        python3 << 'PYEOF'
import cv2
print(f"  CUDA devices: {cv2.cuda.getCudaEnabledDeviceCount()}")
if cv2.cuda.getCudaEnabledDeviceCount() > 0:
    cv2.cuda.printCudaDeviceInfo(0)
PYEOF
    else
        log_error "OpenCV not properly installed"
        return 1
    fi
}

# Verify Tesseract
check_tesseract() {
    log_info "Verifying Tesseract OCR..."
    
    if command -v tesseract &> /dev/null; then
        TESS_VERSION=$(tesseract --version 2>&1 | head -1)
        log_success "Tesseract: $TESS_VERSION"
        
        # List available languages
        LANGUAGES=$(tesseract --list-langs 2>&1 | grep -v "List of" | tr '\n' ' ')
        log_info "Available languages: $LANGUAGES"
    else
        log_warn "Tesseract not found"
    fi
}

# Verify PaddleOCR
check_paddleocr() {
    log_info "Verifying PaddleOCR..."
    
    if python3 -c "from paddleocr import PaddleOCR; print('PaddleOCR imported successfully')" 2>/dev/null; then
        log_success "PaddleOCR is available"
    else
        log_warn "PaddleOCR import failed"
    fi
}

# Verify GStreamer
check_gstreamer() {
    log_info "Verifying GStreamer..."
    
    if command -v gst-launch-1.0 &> /dev/null; then
        GST_VERSION=$(gst-launch-1.0 --version | head -1)
        log_success "GStreamer: $GST_VERSION"
        
        # List available plugins
        log_info "Available plugins:"
        gst-inspect-1.0 | grep -E "(x264|vaapi|nvcodec|openh264)" || true
    else
        log_warn "GStreamer not found"
    fi
}

# Setup environment
setup_env() {
    log_info "Setting up environment..."
    
    # Create necessary directories
    mkdir -p /app/tmp /app/cache /app/models
    
    # Set permissions
    chmod -R 777 /app/tmp /app/cache
    
    # Update library cache
    ldconfig
}

# Run health check
health_check() {
    log_info "Running health checks..."
    
    local FAIL=0
    
    # Check Python imports
    python3 << 'PYEOF' || FAIL=1
import sys
try:
    import cv2
    import paddle
    import numpy
    from paddleocr import PaddleOCR
    print("✓ All Python imports successful")
except ImportError as e:
    print(f"✗ Import failed: {e}")
    sys.exit(1)
PYEOF

    # Check OpenCV version
    python3 << 'PYEOF' || FAIL=1
import cv2
print(f"✓ OpenCV version: {cv2.__version__}")
PYEOF

    if [ $FAIL -eq 0 ]; then
        log_success "All health checks passed"
        return 0
    else
        log_error "Some health checks failed"
        return 1
    fi
}

# Handle different run modes
handle_mode() {
    case "${1:-bash}" in
        worker)
            log_info "Starting in worker mode..."
            exec /app/bin/worker "$@"
            ;;
            
        ocr-server)
            log_info "Starting OCR server..."
            exec python3 -m paddleocr.serve "$@"
            ;;
            
        cv-server)
            log_info "Starting CV server..."
            exec /app/bin/cv-server "$@"
            ;;
            
        test)
            log_info "Running tests..."
            check_gpu
            check_opencv
            check_tesseract
            check_paddleocr
            check_gstreamer
            health_check
            ;;
            
        bash|shell|sh)
            log_info "Starting shell..."
            exec bash
            ;;
            
        *)
            # Execute whatever command was passed
            exec "$@"
            ;;
    esac
}

# Main
main() {
    setup_env
    
    # Run checks on startup
    check_gpu
    check_opencv
    check_tesseract
    check_paddleocr
    check_gstreamer
    
    log_success "Container is ready!"
    
    # Handle the mode
    handle_mode "$@"
}

# Run main
main "$@"
