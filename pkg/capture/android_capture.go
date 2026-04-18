// Package capture provides video capture implementations for different platforms
package capture

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Resolution represents video resolution
type Resolution struct {
	Width  int
	Height int
}

// String returns resolution as string (e.g., "1920x1080")
func (r Resolution) String() string {
	return fmt.Sprintf("%dx%d", r.Width, r.Height)
}

// Frame represents a captured video frame
type Frame struct {
	ID        string
	Timestamp time.Time
	Data      []byte // Raw frame data (H.264 NAL units or decoded)
	Width     int
	Height    int
	Format    FrameFormat
}

// FrameFormat represents the pixel format
type FrameFormat int

const (
	FormatH264 FrameFormat = iota
	FormatYUV420
	FormatRGB
	FormatBGRA
)

// AndroidCapture captures video from Android devices using scrcpy
type AndroidCapture struct {
	deviceID   string
	resolution Resolution
	fps        int
	bitRate    int

	// scrcpy process
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser

	// Frame output
	frameChan chan *Frame
	errorChan chan error

	// Control
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	running bool
}

// AndroidCaptureConfig configuration for Android capture
type AndroidCaptureConfig struct {
	DeviceID   string
	Resolution Resolution
	FPS        int
	BitRate    int // bits per second
}

// DefaultAndroidConfig returns default configuration
func DefaultAndroidConfig(deviceID string) AndroidCaptureConfig {
	return AndroidCaptureConfig{
		DeviceID:   deviceID,
		Resolution: Resolution{Width: 1920, Height: 1080},
		FPS:        30,
		BitRate:    8000000, // 8 Mbps
	}
}

// NewAndroidCapture creates a new Android capture instance
func NewAndroidCapture(config AndroidCaptureConfig) *AndroidCapture {
	ctx, cancel := context.WithCancel(context.Background())

	return &AndroidCapture{
		deviceID:   config.DeviceID,
		resolution: config.Resolution,
		fps:        config.FPS,
		bitRate:    config.BitRate,
		frameChan:  make(chan *Frame, 30), // Buffer ~1 second at 30fps
		errorChan:  make(chan error, 10),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins capturing video from the Android device
func (ac *AndroidCapture) Start() error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.running {
		return fmt.Errorf("capture already running")
	}

	// Check if device is connected
	if err := ac.checkDevice(); err != nil {
		return fmt.Errorf("device check failed: %w", err)
	}

	// Build scrcpy command with raw H.264 output
	args := ac.buildScrcpyArgs()

	ac.cmd = exec.CommandContext(ac.ctx, "scrcpy", args...)

	// Get stdout pipe for video data
	stdout, err := ac.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	ac.stdout = stdout

	// Get stderr pipe for logging
	stderr, err := ac.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	ac.stderr = stderr

	// Start scrcpy
	if err := ac.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scrcpy: %w", err)
	}

	ac.running = true

	// Start goroutines
	ac.wg.Add(3)
	go ac.readFrames()
	go ac.readErrors()
	go ac.monitorProcess()

	return nil
}

// buildScrcpyArgs builds command line arguments for scrcpy
func (ac *AndroidCapture) buildScrcpyArgs() []string {
	args := []string{
		"--no-display",        // Don't show window
		"--record-format=raw", // Output raw H.264
		"--record=-",          // Output to stdout
		fmt.Sprintf("--max-size=%d", ac.resolution.Width),
		fmt.Sprintf("--max-fps=%d", ac.fps),
		fmt.Sprintf("--video-bit-rate=%d", ac.bitRate),
		"--no-control",             // Don't forward input
		"--render-driver=software", // Software rendering (headless)
	}

	if ac.deviceID != "" {
		args = append(args, "--serial="+ac.deviceID)
	}

	return args
}

// checkDevice verifies the Android device is connected
func (ac *AndroidCapture) checkDevice() error {
	cmd := exec.Command("adb", "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("adb devices failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("no devices found")
	}

	// If deviceID is specified, check it's connected
	if ac.deviceID != "" {
		found := false
		for _, line := range lines[1:] {
			if strings.HasPrefix(line, ac.deviceID) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("device %s not found", ac.deviceID)
		}
	}

	return nil
}

// readFrames reads H.264 frames from scrcpy stdout
func (ac *AndroidCapture) readFrames() {
	defer ac.wg.Done()

	reader := bufio.NewReader(ac.stdout)
	frameCount := 0

	for {
		select {
		case <-ac.ctx.Done():
			return
		default:
		}

		// Read H.264 NAL units
		// scrcpy outputs raw H.264 Annex B format
		data := make([]byte, 1024*1024) // 1MB buffer
		n, err := reader.Read(data)
		if err != nil {
			if err != io.EOF {
				select {
				case ac.errorChan <- fmt.Errorf("read error: %w", err):
				default:
				}
			}
			return
		}

		if n > 0 {
			frameCount++
			frame := &Frame{
				ID:        fmt.Sprintf("frame-%d-%d", time.Now().UnixNano(), frameCount),
				Timestamp: time.Now(),
				Data:      data[:n],
				Format:    FormatH264,
				Width:     ac.resolution.Width,
				Height:    ac.resolution.Height,
			}

			select {
			case ac.frameChan <- frame:
			case <-ac.ctx.Done():
				return
			}
		}
	}
}

// readErrors reads and logs stderr from scrcpy
func (ac *AndroidCapture) readErrors() {
	defer ac.wg.Done()

	reader := bufio.NewReader(ac.stderr)

	for {
		select {
		case <-ac.ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return
			}
			return
		}

		// Log stderr for debugging
		// In production, this could be sent to a logger
		_ = line
	}
}

// monitorProcess monitors the scrcpy process
func (ac *AndroidCapture) monitorProcess() {
	defer ac.wg.Done()

	err := ac.cmd.Wait()

	ac.mu.Lock()
	ac.running = false
	ac.mu.Unlock()

	if err != nil && ac.ctx.Err() == nil {
		select {
		case ac.errorChan <- fmt.Errorf("scrcpy exited: %w", err):
		default:
		}
	}
}

// Stop stops the capture
func (ac *AndroidCapture) Stop() error {
	ac.mu.Lock()
	if !ac.running {
		ac.mu.Unlock()
		return nil
	}
	ac.mu.Unlock()

	// Cancel context
	ac.cancel()

	// Kill process if still running
	if ac.cmd != nil && ac.cmd.Process != nil {
		ac.cmd.Process.Kill()
	}

	// Wait for goroutines
	ac.wg.Wait()

	// Close channels
	close(ac.frameChan)
	close(ac.errorChan)

	return nil
}

// GetFrame returns the next captured frame
func (ac *AndroidCapture) GetFrame() (*Frame, bool) {
	frame, ok := <-ac.frameChan
	return frame, ok
}

// GetFrameChan returns the frame channel for reading
func (ac *AndroidCapture) GetFrameChan() <-chan *Frame {
	return ac.frameChan
}

// GetErrorChan returns the error channel
func (ac *AndroidCapture) GetErrorChan() <-chan error {
	return ac.errorChan
}

// IsRunning returns true if capture is active
func (ac *AndroidCapture) IsRunning() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.running
}

// GetDeviceInfo returns information about the connected device
func GetDeviceInfo(deviceID string) (map[string]string, error) {
	args := []string{"-s", deviceID, "shell", "getprop"}
	cmd := exec.Command("adb", args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("adb shell failed: %w", err)
	}

	info := make(map[string]string)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		// Parse properties like: [ro.product.model]: [MIBOX4]
		if strings.HasPrefix(line, "[") && strings.Contains(line, "]: [") {
			parts := strings.SplitN(line, "]: [", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], "[")
				value := strings.TrimSuffix(parts[1], "]")
				info[key] = value
			}
		}
	}

	return info, nil
}

// GetDeviceResolution gets the device screen resolution
func GetDeviceResolution(deviceID string) (Resolution, error) {
	args := []string{"-s", deviceID, "shell", "wm", "size"}
	cmd := exec.Command("adb", args...)

	output, err := cmd.Output()
	if err != nil {
		return Resolution{}, fmt.Errorf("failed to get resolution: %w", err)
	}

	// Parse: Physical size: 1920x1080
	line := strings.TrimSpace(string(output))
	if strings.HasPrefix(line, "Physical size: ") {
		size := strings.TrimPrefix(line, "Physical size: ")
		parts := strings.Split(size, "x")
		if len(parts) == 2 {
			width, _ := strconv.Atoi(parts[0])
			height, _ := strconv.Atoi(parts[1])
			return Resolution{Width: width, Height: height}, nil
		}
	}

	return Resolution{}, fmt.Errorf("could not parse resolution: %s", line)
}

// ListDevices returns a list of connected Android devices
func ListDevices() ([]string, error) {
	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices failed: %w", err)
	}

	var devices []string
	lines := strings.Split(string(output), "\n")

	// Skip first line (header)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "device" {
			devices = append(devices, parts[0])
		}
	}

	return devices, nil
}

// IsAppInForeground checks if an app is in the foreground
func IsAppInForeground(deviceID, packageName string) (bool, error) {
	args := []string{"-s", deviceID, "shell", "dumpsys", "activity", "activities"}
	cmd := exec.Command("adb", args...)

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("dumpsys failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "mResumedActivity") && strings.Contains(line, packageName) {
			return true, nil
		}
	}

	return false, nil
}

// Tap simulates a tap at the given coordinates
func Tap(deviceID string, x, y int) error {
	args := []string{"-s", deviceID, "shell", "input", "tap",
		strconv.Itoa(x), strconv.Itoa(y)}
	cmd := exec.Command("adb", args...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tap failed: %w", err)
	}

	return nil
}

// Swipe simulates a swipe gesture
func Swipe(deviceID string, x1, y1, x2, y2 int, durationMs int) error {
	args := []string{"-s", deviceID, "shell", "input", "swipe",
		strconv.Itoa(x1), strconv.Itoa(y1), strconv.Itoa(x2), strconv.Itoa(y2),
		strconv.Itoa(durationMs)}
	cmd := exec.Command("adb", args...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("swipe failed: %w", err)
	}

	return nil
}

// KeyEvent sends a key event
func KeyEvent(deviceID string, keyCode int) error {
	args := []string{"-s", deviceID, "shell", "input", "keyevent",
		strconv.Itoa(keyCode)}
	cmd := exec.Command("adb", args...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keyevent failed: %w", err)
	}

	return nil
}

// Text types text
func Text(deviceID string, text string) error {
	args := []string{"-s", deviceID, "shell", "input", "text", text}
	cmd := exec.Command("adb", args...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("text input failed: %w", err)
	}

	return nil
}

// Common key codes
const (
	KeyCodeHome       = 3
	KeyCodeBack       = 4
	KeyCodeMenu       = 82
	KeyCodeEnter      = 66
	KeyCodeDPadUp     = 19
	KeyCodeDPadDown   = 20
	KeyCodeDPadLeft   = 21
	KeyCodeDPadRight  = 22
	KeyCodeDPadCenter = 23
)
