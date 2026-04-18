//go:build windows
// +build windows

package capture

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Windows capture using DXGI Desktop Duplication
// Note: This requires CGO and Windows headers
// For pure Go, we use GStreamer with d3d11screencapturesrc or fallback to gdigrab

type windowsCapture struct {
	parent *DesktopCapture
	config DesktopCaptureConfig
	cmd    *exec.Cmd
}

// newWindowsCapture creates a new Windows capture instance
func newWindowsCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return &windowsCapture{
		parent: parent,
		config: config,
	}, nil
}

// Start begins capturing video from Windows desktop
func (wc *windowsCapture) Start() error {
	// Try GStreamer d3d11screencapturesrc first (best quality)
	if CommandExists("gst-launch-1.0") {
		return wc.startGStreamerCapture()
	}

	return fmt.Errorf("GStreamer required for Windows capture")
}

// startGStreamerCapture uses GStreamer for Windows capture
func (wc *windowsCapture) startGStreamerCapture() error {
	args := []string{
		"-q",
	}

	pipeline := wc.buildPipeline()
	args = append(args, pipeline)

	wc.cmd = exec.CommandContext(wc.parent.ctx, "gst-launch-1.0", args...)

	stdout, err := wc.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := wc.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GStreamer: %w", err)
	}

	go wc.readFrames(stdout)

	return nil
}

// buildPipeline builds GStreamer pipeline for Windows capture
func (wc *windowsCapture) buildPipeline() string {
	var source string

	// Try d3d11screencapturesrc (best for Windows 10+)
	// Fallback to gdigrab
	if wc.config.Source == "window" && wc.config.WindowID != "" {
		// Window capture using d3d11screencapturesrc
		source = fmt.Sprintf("d3d11screencapturesrc window-handle=%s", wc.config.WindowID)
	} else {
		// Screen capture
		source = "d3d11screencapturesrc"
	}

	pipeline := fmt.Sprintf(
		"%s ! "+
			"video/x-raw,framerate=%d/1 ! "+
			"videoscale ! "+
			"video/x-raw,width=%d,height=%d ! "+
			"videoconvert ! "+
			"x264enc tune=zerolatency speed-preset=ultrafast ! "+
			"video/x-h264,stream-format=byte-stream ! "+
			"fdsink fd=1",
		source,
		wc.config.FPS,
		wc.config.Resolution.Width,
		wc.config.Resolution.Height,
	)

	// If d3d11screencapturesrc fails, pipeline will error out
	// We could try gdigrab as fallback
	return pipeline
}

// readFrames reads H.264 frames from GStreamer output
func (wc *windowsCapture) readFrames(stdout *exec.Cmd) {
	// Similar to Linux implementation
	// Read from stdout pipe and send to frameChan
}

// Stop stops the capture
func (wc *windowsCapture) Stop() error {
	if wc.cmd != nil && wc.cmd.Process != nil {
		wc.cmd.Process.Kill()
		wc.cmd.Wait()
	}
	return nil
}

// IsRunning returns true if capture is active
func (wc *windowsCapture) IsRunning() bool {
	return wc.cmd != nil && wc.cmd.Process != nil
}

// GetFrameChan returns the frame channel
func (wc *windowsCapture) GetFrameChan() <-chan *Frame {
	return wc.parent.frameChan
}

// listWindowsDisplays lists available displays on Windows
func listWindowsDisplays() ([]Display, error) {
	var displays []Display

	// Use wmic to get display info
	cmd := exec.Command("wmic", "path", "Win32_VideoController", "get", "Name,CurrentHorizontalResolution,CurrentVerticalResolution", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to defaults
		displays = append(displays, Display{
			ID:      "0",
			Name:    "Primary Display",
			Primary: true,
			Width:   1920,
			Height:  1080,
		})
		return displays, nil
	}

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) >= 4 {
			display := Display{
				ID:   strconv.Itoa(i - 1),
				Name: strings.TrimSpace(parts[3]),
			}

			if len(parts) >= 5 {
				display.Width, _ = strconv.Atoi(strings.TrimSpace(parts[4]))
			}
			if len(parts) >= 6 {
				display.Height, _ = strconv.Atoi(strings.TrimSpace(parts[5]))
			}

			if i == 1 {
				display.Primary = true
			}

			displays = append(displays, display)
		}
	}

	return displays, nil
}

// listWindowsWindows lists available windows on Windows
func listWindowsWindows() ([]Window, error) {
	var windows []Window

	// Use PowerShell to get window list
	psScript := `
		Get-Process | Where-Object {$_.MainWindowTitle -ne ""} | ForEach-Object {
			"$($_.Id)|$($_.MainWindowTitle)|$($_.ProcessName)"
		}
	`

	cmd := exec.Command("powershell", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			window := Window{
				ID:      strconv.Itoa(pid),
				Title:   strings.TrimSpace(parts[1]),
				AppName: strings.TrimSpace(parts[2]),
			}

			// Get window geometry using WinAPI (would need CGO)
			// For now, leave at 0,0

			windows = append(windows, window)
		}
	}

	return windows, nil
}

// captureWindowsScreenshot captures screenshot on Windows
func captureWindowsScreenshot(outputPath string) error {
	// Try PowerShell first
	psScript := fmt.Sprintf(`
		Add-Type -AssemblyName System.Windows.Forms
		$screen = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
		$bitmap = New-Object System.Drawing.Bitmap($screen.Width, $screen.Height)
		$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
		$graphics.CopyFromScreen($screen.Location, [System.Drawing.Point]::Empty, $screen.Size)
		$bitmap.Save("%s")
		$graphics.Dispose()
		$bitmap.Dispose()
	`, outputPath)

	cmd := exec.Command("powershell", "-Command", psScript)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback to GStreamer
	pipeline := fmt.Sprintf(
		"d3d11screencapturesrc ! videoconvert ! pngenc ! filesink location=%s",
		outputPath,
	)
	cmd = exec.Command("gst-launch-1.0", "-q", pipeline)
	return cmd.Run()
}

// verifyWindowsSupport checks if Windows system supports capture
func verifyWindowsSupport() error {
	// Check for GStreamer
	if !CommandExists("gst-launch-1.0") {
		return fmt.Errorf("GStreamer not found. Install from: https://gstreamer.freedesktop.org/download/")
	}

	return nil
}

// Windows-specific window handle type
type HWND syscall.Handle

// GetForegroundWindow gets the foreground window handle
// Note: This requires CGO and would be in a separate .c file
func GetForegroundWindow() (HWND, error) {
	return 0, fmt.Errorf("CGO required for WinAPI access")
}

// GetWindowRect gets window rectangle
func GetWindowRect(hwnd HWND) (rect Rect, err error) {
	return Rect{}, fmt.Errorf("CGO required for WinAPI access")
}

// Rect represents a rectangle
type Rect struct {
	Left, Top, Right, Bottom int32
}

// Width returns width
func (r Rect) Width() int {
	return int(r.Right - r.Left)
}

// Height returns height
func (r Rect) Height() int {
	return int(r.Bottom - r.Top)
}

// DXGI capture structures (for future CGO implementation)
type DXGIOutputDuplication struct {
	// Would contain ID3D11Texture2D, etc.
}

// DXGIFrame represents a captured frame using DXGI
type DXGIFrame struct {
	Data      []byte
	Width     int
	Height    int
	Timestamp time.Time
}

// CaptureFrameDXGI captures a frame using DXGI Desktop Duplication
// This is a placeholder for future CGO implementation
func CaptureFrameDXGI() (*DXGIFrame, error) {
	return nil, fmt.Errorf("DXGI capture requires CGO and Windows SDK")
}

// Stub functions for Linux and macOS (only compiled on Windows)

func newLinuxCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return nil, fmt.Errorf("Linux capture not available on Windows")
}

func listLinuxDisplays() ([]Display, error) {
	return nil, fmt.Errorf("Linux displays not available on Windows")
}

func listLinuxWindows() ([]Window, error) {
	return nil, fmt.Errorf("Linux windows not available on Windows")
}

func captureLinuxScreenshot(outputPath string) error {
	return fmt.Errorf("Linux screenshot not available on Windows")
}

func verifyLinuxSupport() error {
	return fmt.Errorf("Linux capture not available on Windows")
}

func newMacOSCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return nil, fmt.Errorf("macOS capture not available on Windows")
}

func listMacOSDisplays() ([]Display, error) {
	return nil, fmt.Errorf("macOS displays not available on Windows")
}

func listMacOSWindows() ([]Window, error) {
	return nil, fmt.Errorf("macOS windows not available on Windows")
}

func captureMacOSScreenshot(outputPath string) error {
	return fmt.Errorf("macOS screenshot not available on Windows")
}

func verifyMacOSSupport() error {
	return fmt.Errorf("macOS capture not available on Windows")
}

func IsWayland() bool {
	return false
}

func GetDesktopEnvironment() string {
	return ""
}

func IsScreenCaptureKitAvailable() bool {
	return false
}

func CheckScreenRecordingPermission() bool {
	return false
}
