# HelixQA Base Runtime Image
# Contains: OpenCV + GStreamer + Tesseract + PaddleOCR + Python
# Go binaries are mounted at runtime

FROM docker.io/library/ubuntu:22.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Set environment variables
ENV PYTHONUNBUFFERED=1
ENV PIP_NO_CACHE_DIR=1
ENV PIP_DISABLE_PIP_VERSION_CHECK=1
ENV OPENCV_LOG_LEVEL=ERROR

# Install dependencies in a single layer
RUN apt-get update && apt-get install -y --no-install-recommends \
    # GStreamer + plugins
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
    gstreamer1.0-pulseaudio \
    gstreamer1.0-vaapi \
    libgstreamer-plugins-base1.0-dev \
    libgstreamer1.0-dev \
    # OpenCV (pre-built from apt - faster)
    libopencv-dev \
    python3-opencv \
    # Tesseract OCR
    tesseract-ocr \
    tesseract-ocr-eng \
    tesseract-ocr-deu \
    tesseract-ocr-fra \
    tesseract-ocr-spa \
    libtesseract-dev \
    libleptonica-dev \
    # Python
    python3 \
    python3-pip \
    python3-dev \
    python3-numpy \
    python3-venv \
    # Go (for running binaries)
    golang-go \
    ca-certificates \
    curl \
    git \
    ffmpeg \
    # Video codecs
    libx265-199 \
    libde265-0 \
    libvpx7 \
    # Utilities
    wget \
    netcat \
    iputils-ping \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Install Python packages for PaddleOCR and ML
RUN pip3 install --no-cache-dir --upgrade pip setuptools wheel && \
    pip3 install --no-cache-dir \
    # PaddlePaddle (CPU version - smaller)
    paddlepaddle==2.6.2 \
    # PaddleOCR
    paddleocr \
    # ML/Computer Vision
    numpy \
    pillow \
    scikit-image \
    # gRPC for services
    grpcio \
    grpcio-tools \
    protobuf \
    # HTTP server
    fastapi \
    uvicorn \
    # Utilities
    pyyaml \
    requests \
    tqdm \
    && rm -rf /root/.cache/pip

# Create app directory structure
RUN mkdir -p /app/bin /app/models /app/cache /app/tmp /app/config

# Copy entrypoint script
COPY docker/base-opencv-gstreamer/entrypoint.sh /app/
RUN chmod +x /app/entrypoint.sh

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD python3 -c "import cv2; from paddleocr import PaddleOCR; print('OK')" || exit 1

# Working directory
WORKDIR /app

# Expose common ports
EXPOSE 11434 8080 8554 50051

# Entry point
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["bash"]

# Labels
LABEL maintainer="HelixQA Team"
LABEL description="HelixQA Runtime: OpenCV + GStreamer + Tesseract + PaddleOCR"
LABEL version="1.0"
