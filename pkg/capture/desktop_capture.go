// Package capture provides desktop capture implementations for Linux, Windows, and macOS
package capture

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// DesktopCapture captures video from desktop (screen or window)
type DesktopCapture struct {
	captureImpl desktopCaptureImpl
	
	// Common properties
	source      string  // "screen" or "window"
	windowID    string  // For window capture
	resolution  Resolution
	fps         int
	
	// Output
	frameChan   chan *Frame
	errorChan   chan error
	
	// Control
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	running     bool
}

// desktopCaptureImpl is the platform-specific implementation interface
type desktopCaptureImpl interface {
	Start() error
	Stop() error
	IsRunning() bool
	GetFrameChan() <-chan *Frame
}

// DesktopCaptureConfig configuration for desktop capture
type DesktopCaptureConfig struct {
	Source     string     // "screen" or "window"
	WindowID   string     // Window ID for window capture
	Resolution Resolution
	FPS        int
	Display    string     // Display identifier (e.g., ":0", "DP-1")
}

// DefaultDesktopConfig returns default configuration
func DefaultDesktopConfig() DesktopCaptureConfig {
	return DesktopCaptureConfig{
		Source:     "screen",
		Resolution: Resolution{Width: 1920, Height: 1080},
		FPS:        30,
		Display:    "",
	}
}

// NewDesktopCapture creates a new desktop capture instance
func NewDesktopCapture(config DesktopCaptureConfig) (*DesktopCapture, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	dc := &DesktopCapture{
		source:     config.Source,
		windowID:   config.WindowID,
		resolution: config.Resolution,
		fps:        config.FPS,
		frameChan:  make(chan *Frame, 30),
		errorChan:  make(chan error, 10),
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// Create platform-specific implementation
	var err error
	switch runtime.GOOS {
	case "linux":
		dc.captureImpl, err = newLinuxCapture(dc, config)
	case "windows":
		dc.captureImpl, err = newWindowsCapture(dc, config)
	case "darwin":
		dc.captureImpl, err = newMacOSCapture(dc, config)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	
	if err != nil {
		return nil, err
	}
	
	return dc, nil
}

// Start begins capturing video from desktop
func (dc *DesktopCapture) Start() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	if dc.running {
		return fmt.Errorf("capture already running")
	}
	
	if err := dc.captureImpl.Start(); err != nil {
		return err
	}
	
	dc.running = true
	return nil
}

// Stop stops the capture
func (dc *DesktopCapture) Stop() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	if !dc.running {
		return nil
	}
	
	dc.cancel()
	
	if err := dc.captureImpl.Stop(); err != nil {
		return err
	}
	
	dc.running = false
	return nil
}

// IsRunning returns true if capture is active
func (dc *DesktopCapture) IsRunning() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.running
}

// GetFrameChan returns the frame channel
func (dc *DesktopCapture) GetFrameChan() <-chan *Frame {
	return dc.frameChan
}

// GetErrorChan returns the error channel
func (dc *DesktopCapture) GetErrorChan() <-chan error {
	return dc.errorChan
}

// GetSource returns the capture source
func (dc *DesktopCapture) GetSource() string {
	return dc.source
}

// GetPlatform returns the current platform
func GetPlatform() string {
	return runtime.GOOS
}

// ListDisplays lists available displays (platform-specific)
func ListDisplays() ([]Display, error) {
	switch runtime.GOOS {
	case "linux":
		return listLinuxDisplays()
	case "windows":
		return listWindowsDisplays()
	case "darwin":
		return listMacOSDisplays()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Display represents a display/monitor
type Display struct {
	ID       string
	Name     string
	Width    int
	Height   int
	Primary  bool
	X        int
	Y        int
}

// ListWindows lists available windows (platform-specific)
func ListWindows() ([]Window, error) {
	switch runtime.GOOS {
	case "linux":
		return listLinuxWindows()
	case "windows":
		return listWindowsWindows()
	case "darwin":
		return listMacOSWindows()
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// Window represents a window
type Window struct {
	ID       string
	Title    string
	AppName  string
	X        int
	Y        int
	Width    int
	Height   int
}

// String returns string representation
func (w Window) String() string {
	return fmt.Sprintf("%s: %s (%dx%d)", w.ID, w.Title, w.Width, w.Height)
}

// FindWindow finds a window by title or app name
func FindWindow(query string) (*Window, error) {
	windows, err := ListWindows()
	if err != nil {
		return nil, err
	}
	
	query = strings.ToLower(query)
	for _, w := range windows {
		if strings.Contains(strings.ToLower(w.Title), query) ||
		   strings.Contains(strings.ToLower(w.AppName), query) {
			return &w, nil
		}
	}
	
	return nil, fmt.Errorf("window not found: %s", query)
}

// CaptureScreenshot captures a single screenshot (platform-specific)
func CaptureScreenshot(outputPath string) error {
	switch runtime.GOOS {
	case "linux":
		return captureLinuxScreenshot(outputPath)
	case "windows":
		return captureWindowsScreenshot(outputPath)
	case "darwin":
		return captureMacOSScreenshot(outputPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// VerifyPlatformSupport checks if the current platform supports capture
func VerifyPlatformSupport() error {
	switch runtime.GOOS {
	case "linux":
		return verifyLinuxSupport()
	case "windows":
		return verifyWindowsSupport()
	case "darwin":
		return verifyMacOSSupport()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// IsPlatformSupported returns true if platform is supported
func IsPlatformSupported() bool {
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		return true
	default:
		return false
	}
}

// CommandExists checks if a command exists
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
