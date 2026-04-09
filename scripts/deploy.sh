#!/bin/bash
# HelixQA Production Deployment Script
# Usage: ./scripts/deploy.sh [environment]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
ENVIRONMENT="${1:-production}"
COMPOSE_FILE="docker-compose.stack.yml"
STACK_NAME="helixqa"

echo -e "${BLUE}============================================${NC}"
echo -e "${BLUE}  HelixQA Deployment Script${NC}"
echo -e "${BLUE}  Environment: $ENVIRONMENT${NC}"
echo -e "${BLUE}============================================${NC}"
echo

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

# Check Docker/Podman
if command -v docker &> /dev/null; then
    RUNTIME="docker"
    COMPOSE_CMD="docker compose"
elif command -v podman &> /dev/null; then
    RUNTIME="podman"
    COMPOSE_CMD="podman-compose"
else
    echo -e "${RED}Error: Neither Docker nor Podman found${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Container runtime: $RUNTIME${NC}"

# Check compose file exists
if [ ! -f "$COMPOSE_FILE" ]; then
    echo -e "${RED}Error: $COMPOSE_FILE not found${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Compose file found${NC}"

# Functions
pre_deploy() {
    echo
    echo -e "${YELLOW}Pre-deployment checks...${NC}"
    
    # Create necessary directories
    mkdir -p data/{mediamtx,nats,ollama,prometheus,grafana,redis}
    
    # Check ports are available
    local ports=("8554" "1935" "8888" "8889" "4222" "11434" "9090" "3000" "8080")
    for port in "${ports[@]}"; do
        if lsof -Pi :"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
            echo -e "${RED}Warning: Port $port is already in use${NC}"
        fi
    done
    
    echo -e "${GREEN}✓ Pre-deployment checks complete${NC}"
}

build_images() {
    echo
    echo -e "${YELLOW}Building container images...${NC}"
    
    $COMPOSE_CMD -f "$COMPOSE_FILE" build --parallel
    
    echo -e "${GREEN}✓ Images built successfully${NC}"
}

pull_models() {
    echo
    echo -e "${YELLOW}Pulling Ollama models...${NC}"
    
    # Start Ollama service first
    $COMPOSE_CMD -f "$COMPOSE_FILE" up -d ollama
    
    # Wait for Ollama to be ready
    echo -e "${YELLOW}Waiting for Ollama to be ready...${NC}"
    until curl -s http://localhost:11434/api/tags > /dev/null 2>&1; do
        sleep 2
    done
    
    # Pull vision models
    echo -e "${YELLOW}Pulling llava model...${NC}"
    curl -X POST http://localhost:11434/api/pull -d '{"name": "llava"}' || true
    
    echo -e "${GREEN}✓ Models pulled successfully${NC}"
}

deploy_stack() {
    echo
    echo -e "${YELLOW}Deploying HelixQA stack...${NC}"
    
    # Deploy the full stack
    $COMPOSE_CMD -f "$COMPOSE_FILE" up -d
    
    echo -e "${GREEN}✓ Stack deployed${NC}"
}

wait_for_healthy() {
    echo
    echo -e "${YELLOW}Waiting for services to be healthy...${NC}"
    
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        local healthy=true
        
        # Check each service
        services=("mediamtx" "nats" "ollama" "helixqa-api" "redis")
        for service in "${services[@]}"; do
            if ! $COMPOSE_CMD -f "$COMPOSE_FILE" ps "$service" | grep -q "healthy\|running"; then
                healthy=false
                break
            fi
        done
        
        if [ "$healthy" = true ]; then
            echo -e "${GREEN}✓ All services are healthy${NC}"
            return 0
        fi
        
        echo -e "${YELLOW}Attempt $attempt/$max_attempts: Waiting for services...${NC}"
        sleep 5
        attempt=$((attempt + 1))
    done
    
    echo -e "${RED}✗ Services failed to become healthy${NC}"
    return 1
}

verify_deployment() {
    echo
    echo -e "${YELLOW}Verifying deployment...${NC}"
    
    # Test API endpoint
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ API is responding${NC}"
    else
        echo -e "${YELLOW}⚠ API health check failed (may still be starting)${NC}"
    fi
    
    # Test MediaMTX
    if curl -s http://localhost:9998/metrics > /dev/null 2>&1; then
        echo -e "${GREEN}✓ MediaMTX is responding${NC}"
    else
        echo -e "${YELLOW}⚠ MediaMTX metrics check failed${NC}"
    fi
    
    # Test NATS
    if curl -s http://localhost:8222/healthz > /dev/null 2>&1; then
        echo -e "${GREEN}✓ NATS is responding${NC}"
    else
        echo -e "${YELLOW}⚠ NATS health check failed${NC}"
    fi
    
    echo -e "${GREEN}✓ Verification complete${NC}"
}

print_info() {
    echo
    echo -e "${GREEN}============================================${NC}"
    echo -e "${GREEN}  Deployment Complete!${NC}"
    echo -e "${GREEN}============================================${NC}"
    echo
    echo -e "${BLUE}Service Endpoints:${NC}"
    echo -e "  API:          http://localhost:8080"
    echo -e "  MediaMTX:     http://localhost:9998 (metrics)"
    echo -e "  RTSP:         rtsp://localhost:8554"
    echo -e "  HLS:          http://localhost:8888"
    echo -e "  WebRTC:       http://localhost:8889"
    echo -e "  NATS:         nats://localhost:4222"
    echo -e "  Ollama:       http://localhost:11434"
    echo -e "  Prometheus:   http://localhost:9090"
    echo -e "  Grafana:      http://localhost:3000 (admin/admin)"
    echo
    echo -e "${BLUE}Useful Commands:${NC}"
    echo -e "  View logs:    $COMPOSE_CMD -f $COMPOSE_FILE logs -f"
    echo -e "  Scale:        $COMPOSE_CMD -f $COMPOSE_FILE up -d --scale helixqa-vision=3"
    echo -e "  Stop:         $COMPOSE_CMD -f $COMPOSE_FILE down"
    echo -e "  Update:       ./scripts/deploy.sh"
    echo
    echo -e "${BLUE}Stream Paths:${NC}"
    echo -e "  Android TV:   rtsp://localhost:8554/android_tv"
    echo -e "  Desktop:      rtsp://localhost:8554/desktop_linux"
    echo -e "  Web Browser:  rtsp://localhost:8554/web_browser"
    echo
}

# Main deployment flow
main() {
    pre_deploy
    build_images
    pull_models
    deploy_stack
    wait_for_healthy
    verify_deployment
    print_info
}

# Run main function
main
