// Package streaming provides video streaming capabilities for HelixQA
package streaming

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/capture"
)

// ScrcpyRTSPBridge converts scrcpy output to RTSP stream
// This allows multiple consumers without re-capture
type ScrcpyRTSPBridge struct {
	deviceID    string
	streamPath  string
	resolution  capture.Resolution
	fps         int
	
	// Capture
	capture     *capture.AndroidCapture
	
	// FFmpeg process for RTSP encoding
	ffmpegCmd   *exec.Cmd
	ffmpegStdin io.WriteCloser
	
	// Control
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
	running     bool
	
	// RTSP server info
	rtspURL     string
	rtspHost    string
	rtspPort    int
}

// BridgeConfig configuration for the bridge
type BridgeConfig struct {
	DeviceID   string
	StreamPath string           // e.g., "android_tv", "mobile_app"
	Resolution capture.Resolution
	FPS        int
	RTSPHost   string
	RTSPPort   int
}

// DefaultBridgeConfig returns default configuration
func DefaultBridgeConfig(deviceID, streamPath string) BridgeConfig {
	return BridgeConfig{
		DeviceID:   deviceID,
		StreamPath: streamPath,
		Resolution: capture.Resolution{Width: 1920, Height: 1080},
		FPS:        30,
		RTSPHost:   "localhost",
		RTSPPort:   8554,
	}
}

// NewScrcpyRTSPBridge creates a new bridge
func NewScrcpyRTSPBridge(config BridgeConfig) *ScrcpyRTSPBridge {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ScrcpyRTSPBridge{
		deviceID:   config.DeviceID,
		streamPath: config.StreamPath,
		resolution: config.Resolution,
		fps:        config.FPS,
		rtspHost:   config.RTSPHost,
		rtspPort:   config.RTSPPort,
		rtspURL:    fmt.Sprintf("rtsp://%s:%d/%s", config.RTSPHost, config.RTSPPort, config.StreamPath),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins the bridge
func (sb *ScrcpyRTSPBridge) Start() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	
	if sb.running {
		return fmt.Errorf("bridge already running")
	}
	
	// Start FFmpeg first (it will create the RTSP server endpoint)
	if err := sb.startFFmpeg(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}
	
	// Start scrcpy capture
	captureConfig := capture.AndroidCaptureConfig{
		DeviceID:   sb.deviceID,
		Resolution: sb.resolution,
		FPS:        sb.fps,
		BitRate:    8000000,
	}
	
	sb.capture = capture.NewAndroidCapture(captureConfig)
	
	if err := sb.capture.Start(); err != nil {
		sb.stopFFmpeg()
		return fmt.Errorf("failed to start capture: %w", err)
	}
	
	sb.running = true
	
	// Start frame forwarding
	sb.wg.Add(1)
	go sb.forwardFrames()
	
	return nil
}

// startFFmpeg starts FFmpeg to encode and serve RTSP
func (sb *ScrcpyRTSPBridge) startFFmpeg() error {
	// FFmpeg command to:
	// 1. Read raw H.264 from stdin
	// 2. Encode to H.264 (if needed)
	// 3. Output to RTSP
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "h264",          // Input format: H.264
		"-i", "-",             // Read from stdin
		"-c:v", "copy",        // Copy video stream (no re-encode)
		"-f", "rtsp",          // Output format: RTSP
		"-rtsp_transport", "tcp",
		sb.rtspURL,
	}
	
	sb.ffmpegCmd = exec.CommandContext(sb.ctx, "ffmpeg", args...)
	
	// Get stdin pipe
	stdin, err := sb.ffmpegCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	sb.ffmpegStdin = stdin
	
	// Start FFmpeg
	if err := sb.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}
	
	return nil
}

// stopFFmpeg stops FFmpeg
func (sb *ScrcpyRTSPBridge) stopFFmpeg() {
	if sb.ffmpegStdin != nil {
		sb.ffmpegStdin.Close()
	}
	
	if sb.ffmpegCmd != nil && sb.ffmpegCmd.Process != nil {
		sb.ffmpegCmd.Process.Kill()
		sb.ffmpegCmd.Wait()
	}
}

// forwardFrames forwards frames from scrcpy to FFmpeg
func (sb *ScrcpyRTSPBridge) forwardFrames() {
	defer sb.wg.Done()
	
	frameChan := sb.capture.GetFrameChan()
	errorChan := sb.capture.GetErrorChan()
	
	for {
		select {
		case <-sb.ctx.Done():
			return
			
		case frame, ok := <-frameChan:
			if !ok {
				return
			}
			
			// Write H.264 data to FFmpeg
			if _, err := sb.ffmpegStdin.Write(frame.Data); err != nil {
				// Log error but continue
				continue
			}
			
		case err, ok := <-errorChan:
			if !ok {
				return
			}
			// Log error
			_ = err
		}
	}
}

// Stop stops the bridge
func (sb *ScrcpyRTSPBridge) Stop() error {
	sb.mu.Lock()
	if !sb.running {
		sb.mu.Unlock()
		return nil
	}
	sb.mu.Unlock()
	
	// Cancel context
	sb.cancel()
	
	// Stop capture
	if sb.capture != nil {
		sb.capture.Stop()
	}
	
	// Stop FFmpeg
	sb.stopFFmpeg()
	
	// Wait for goroutines
	sb.wg.Wait()
	
	sb.mu.Lock()
	sb.running = false
	sb.mu.Unlock()
	
	return nil
}

// IsRunning returns true if bridge is active
func (sb *ScrcpyRTSPBridge) IsRunning() bool {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.running
}

// GetRTSPURL returns the RTSP URL for this stream
func (sb *ScrcpyRTSPBridge) GetRTSPURL() string {
	return sb.rtspURL
}

// GetStreamPath returns the stream path
func (sb *ScrcpyRTSPBridge) GetStreamPath() string {
	return sb.streamPath
}

// MultiStreamManager manages multiple RTSP streams
type MultiStreamManager struct {
	bridges map[string]*ScrcpyRTSPBridge
	mu      sync.RWMutex
}

// NewMultiStreamManager creates a new stream manager
func NewMultiStreamManager() *MultiStreamManager {
	return &MultiStreamManager{
		bridges: make(map[string]*ScrcpyRTSPBridge),
	}
}

// CreateStream creates a new stream bridge
func (sm *MultiStreamManager) CreateStream(config BridgeConfig) (*ScrcpyRTSPBridge, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	key := fmt.Sprintf("%s-%s", config.DeviceID, config.StreamPath)
	
	if _, exists := sm.bridges[key]; exists {
		return nil, fmt.Errorf("stream already exists: %s", key)
	}
	
	bridge := NewScrcpyRTSPBridge(config)
	
	if err := bridge.Start(); err != nil {
		return nil, err
	}
	
	sm.bridges[key] = bridge
	
	return bridge, nil
}

// GetStream returns an existing stream bridge
func (sm *MultiStreamManager) GetStream(deviceID, streamPath string) (*ScrcpyRTSPBridge, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	key := fmt.Sprintf("%s-%s", deviceID, streamPath)
	bridge, ok := sm.bridges[key]
	return bridge, ok
}

// StopStream stops and removes a stream
func (sm *MultiStreamManager) StopStream(deviceID, streamPath string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	key := fmt.Sprintf("%s-%s", deviceID, streamPath)
	
	bridge, ok := sm.bridges[key]
	if !ok {
		return fmt.Errorf("stream not found: %s", key)
	}
	
	delete(sm.bridges, key)
	
	return bridge.Stop()
}

// StopAll stops all streams
func (sm *MultiStreamManager) StopAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	for key, bridge := range sm.bridges {
		bridge.Stop()
		delete(sm.bridges, key)
	}
}

// ListStreams returns a list of active streams
func (sm *MultiStreamManager) ListStreams() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	streams := make([]string, 0, len(sm.bridges))
	for key := range sm.bridges {
		streams = append(streams, key)
	}
	
	return streams
}

// RTSPClient connects to an RTSP stream and reads frames
type RTSPClient struct {
	url         string
	ffmpegCmd   *exec.Cmd
	stdout      io.ReadCloser
	frameChan   chan *capture.Frame
	errorChan   chan error
	
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
	running     bool
}

// NewRTSPClient creates a new RTSP client
func NewRTSPClient(url string) *RTSPClient {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &RTSPClient{
		url:       url,
		frameChan: make(chan *capture.Frame, 30),
		errorChan: make(chan error, 10),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins reading from the RTSP stream
func (rc *RTSPClient) Start() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	if rc.running {
		return fmt.Errorf("client already running")
	}
	
	// FFmpeg command to read RTSP and output raw frames
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", rc.url,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-s", "1920x1080",
		"-",
	}
	
	rc.ffmpegCmd = exec.CommandContext(rc.ctx, "ffmpeg", args...)
	
	// Get stdout pipe
	stdout, err := rc.ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	rc.stdout = stdout
	
	// Start FFmpeg
	if err := rc.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}
	
	rc.running = true
	
	// Start frame reading
	rc.wg.Add(1)
	go rc.readFrames()
	
	return nil
}

// readFrames reads raw video frames from FFmpeg
func (rc *RTSPClient) readFrames() {
	defer rc.wg.Done()
	
	// Calculate frame size: 1920 * 1080 * 3 bytes (RGB24)
	frameSize := 1920 * 1080 * 3
	
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}
		
		// Read frame
		data := make([]byte, frameSize)
		n, err := io.ReadFull(rc.stdout, data)
		if err != nil {
			if err != io.EOF {
				select {
				case rc.errorChan <- fmt.Errorf("read error: %w", err):
				default:
				}
			}
			return
		}
		
		if n == frameSize {
			frame := &capture.Frame{
				ID:        fmt.Sprintf("rtsp-frame-%d", time.Now().UnixNano()),
				Timestamp: time.Now(),
				Data:      data,
				Width:     1920,
				Height:    1080,
				Format:    capture.FormatRGB,
			}
			
			select {
			case rc.frameChan <- frame:
			case <-rc.ctx.Done():
				return
			}
		}
	}
}

// Stop stops the client
func (rc *RTSPClient) Stop() error {
	rc.mu.Lock()
	if !rc.running {
		rc.mu.Unlock()
		return nil
	}
	rc.mu.Unlock()
	
	rc.cancel()
	
	if rc.ffmpegCmd != nil && rc.ffmpegCmd.Process != nil {
		rc.ffmpegCmd.Process.Kill()
		rc.ffmpegCmd.Wait()
	}
	
	rc.wg.Wait()
	
	close(rc.frameChan)
	close(rc.errorChan)
	
	rc.mu.Lock()
	rc.running = false
	rc.mu.Unlock()
	
	return nil
}

// GetFrameChan returns the frame channel
func (rc *RTSPClient) GetFrameChan() <-chan *capture.Frame {
	return rc.frameChan
}

// IsRunning returns true if client is active
func (rc *RTSPClient) IsRunning() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.running
}
