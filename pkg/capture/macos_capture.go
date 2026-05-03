//go:build darwin
// +build darwin

package capture

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// macOS capture using ScreenCaptureKit (macOS 12.3+) or CoreMediaIO
type macOSCapture struct {
	parent *DesktopCapture
	config DesktopCaptureConfig
	cmd    *exec.Cmd
}

// newMacOSCapture creates a new macOS capture instance
func newMacOSCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return &macOSCapture{
		parent: parent,
		config: config,
	}, nil
}

// Start begins capturing video from macOS desktop
func (mc *macOSCapture) Start() error {
	// Try GStreamer with avfvideosrc first
	if CommandExists("gst-launch-1.0") {
		return mc.startGStreamerCapture()
	}

	return fmt.Errorf("GStreamer required for macOS capture")
}

// startGStreamerCapture uses GStreamer for macOS capture
func (mc *macOSCapture) startGStreamerCapture() error {
	args := []string{
		"-q",
	}

	pipeline := mc.buildPipeline()
	args = append(args, pipeline)

	mc.cmd = exec.CommandContext(mc.parent.ctx, "gst-launch-1.0", args...)

	stdout, err := mc.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := mc.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GStreamer: %w", err)
	}

	go mc.readFrames(stdout)

	return nil
}

// buildPipeline builds GStreamer pipeline for macOS capture
func (mc *macOSCapture) buildPipeline() string {
	// Use avfvideosrc for macOS screen capture
	var source string

	if mc.config.Source == "window" && mc.config.WindowID != "" {
		// Window capture (requires CGO for ScreenCaptureKit)
		source = "avfvideosrc capture-screen=false capture-screen-cursor=false"
	} else {
		// Screen capture
		source = "avfvideosrc capture-screen=true capture-screen-cursor=true"
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
		mc.config.FPS,
		mc.config.Resolution.Width,
		mc.config.Resolution.Height,
	)

	return pipeline
}

// readFrames reads H.264 frames from GStreamer output
func (mc *macOSCapture) readFrames(stdout *exec.Cmd) {
	_ = mc
	_ = stdout
	// TODO: implement macOS frame reading (similar to Linux implementation)
}

// Stop stops the capture
func (mc *macOSCapture) Stop() error {
	if mc.cmd != nil && mc.cmd.Process != nil {
		mc.cmd.Process.Kill()
		mc.cmd.Wait()
	}
	return nil
}

// IsRunning returns true if capture is active
func (mc *macOSCapture) IsRunning() bool {
	return mc.cmd != nil && mc.cmd.Process != nil
}

// GetFrameChan returns the frame channel
func (mc *macOSCapture) GetFrameChan() <-chan *Frame {
	return mc.parent.frameChan
}

// listMacOSDisplays lists available displays on macOS
func listMacOSDisplays() ([]Display, error) {
	var displays []Display

	// Use system_profiler to get display info
	cmd := exec.Command("system_profiler", "SPDisplaysDataType", "-json")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to defaults
		displays = append(displays, Display{
			ID:      "0",
			Name:    "Built-in Display",
			Primary: true,
			Width:   1920,
			Height:  1080,
		})
		return displays, nil
	}

	// Parse JSON output
	// This is simplified - full implementation would parse the JSON
	_ = output

	return displays, nil
}

// listMacOSWindows lists available windows on macOS
func listMacOSWindows() ([]Window, error) {
	var windows []Window

	// Use AppleScript to get window list
	script := `
		tell application "System Events"
			set windowList to {}
			repeat with proc in (get processes whose background only is false)
				set procName to name of proc
				repeat with win in (get windows of proc)
					set winName to name of win
					if winName is not "" then
						set end of windowList to (procName & "|" & winName)
					end if
				end repeat
			end repeat
			return windowList as string
		end tell
	`

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse output
	windowStrs := strings.Split(string(output), ", ")
	for i, ws := range windowStrs {
		parts := strings.Split(ws, "|")
		if len(parts) >= 2 {
			windows = append(windows, Window{
				ID:      strconv.Itoa(i),
				AppName: parts[0],
				Title:   parts[1],
			})
		}
	}

	return windows, nil
}

// captureMacOSScreenshot captures screenshot on macOS
func captureMacOSScreenshot(outputPath string) error {
	// Use screencapture command
	cmd := exec.Command("screencapture", "-x", outputPath)
	return cmd.Run()
}

// verifyMacOSSupport checks if macOS system supports capture
func verifyMacOSSupport() error {
	// Check for GStreamer
	if !CommandExists("gst-launch-1.0") {
		return fmt.Errorf("GStreamer not found. Install with: brew install gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad gst-libav")
	}

	// Check for avfvideosrc
	cmd := exec.Command("gst-inspect-1.0", "avfvideosrc")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("avfvideosrc not found. Install with: brew install gst-plugins-bad")
	}

	return nil
}

// ScreenCaptureKit integration (for future CGO implementation)
// ScreenCaptureKit is available on macOS 12.3+

// ScreenCaptureKitFrame represents a frame captured using ScreenCaptureKit
type ScreenCaptureKitFrame struct {
	Data   []byte
	Width  int
	Height int
}

// CaptureWithScreenCaptureKit captures using ScreenCaptureKit
// This is a placeholder for future CGO implementation
func CaptureWithScreenCaptureKit() (*ScreenCaptureKitFrame, error) {
	return nil, fmt.Errorf("ScreenCaptureKit requires CGO and macOS 12.3+")
}

// IsScreenCaptureKitAvailable returns true if ScreenCaptureKit is available
func IsScreenCaptureKitAvailable() bool {
	// Check macOS version
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	version := strings.TrimSpace(string(output))
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		major, _ := strconv.Atoi(parts[0])
		minor, _ := strconv.Atoi(parts[1])

		// ScreenCaptureKit available on macOS 12.3+
		if major > 12 || (major == 12 && minor >= 3) {
			return true
		}
	}

	return false
}

// RequestScreenRecordingPermission requests screen recording permission on macOS
func RequestScreenRecordingPermission() error {
	// Open System Preferences to Security & Privacy
	cmd := exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenRecording")
	return cmd.Run()
}

// CheckScreenRecordingPermission checks if screen recording permission is granted
func CheckScreenRecordingPermission() bool {
	// Try to capture a test screenshot
	tmpFile := "/tmp/helixqa_permission_test.png"
	cmd := exec.Command("screencapture", "-x", tmpFile)
	err := cmd.Run()

	// Clean up
	exec.Command("rm", "-f", tmpFile).Run()

	return err == nil
}

// CGDisplay capture using CoreGraphics (for future CGO implementation)

// CGDisplayID represents a CoreGraphics display ID
type CGDisplayID uint32

// GetMainDisplay returns the main display ID
func GetMainDisplay() CGDisplayID {
	// Would use CGMainDisplayID() via CGO
	return 0
}

// GetOnlineDisplays returns all online displays
func GetOnlineDisplays() ([]CGDisplayID, error) {
	return nil, fmt.Errorf("CoreGraphics requires CGO")
}

// CaptureDisplay captures a display using CoreGraphics
func CaptureDisplay(displayID CGDisplayID) (*Frame, error) {
	return nil, fmt.Errorf("CoreGraphics capture requires CGO")
}

// Stub functions for Linux and Windows (only compiled on macOS)

func newLinuxCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return nil, fmt.Errorf("Linux capture not available on macOS")
}

func listLinuxDisplays() ([]Display, error) {
	return nil, fmt.Errorf("Linux displays not available on macOS")
}

func listLinuxWindows() ([]Window, error) {
	return nil, fmt.Errorf("Linux windows not available on macOS")
}

func captureLinuxScreenshot(outputPath string) error {
	return fmt.Errorf("Linux screenshot not available on macOS")
}

func verifyLinuxSupport() error {
	return fmt.Errorf("Linux capture not available on macOS")
}

func newWindowsCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return nil, fmt.Errorf("Windows capture not available on macOS")
}

func listWindowsDisplays() ([]Display, error) {
	return nil, fmt.Errorf("Windows displays not available on macOS")
}

func listWindowsWindows() ([]Window, error) {
	return nil, fmt.Errorf("Windows windows not available on macOS")
}

func captureWindowsScreenshot(outputPath string) error {
	return fmt.Errorf("Windows screenshot not available on macOS")
}

func verifyWindowsSupport() error {
	return fmt.Errorf("Windows capture not available on macOS")
}

func IsWayland() bool {
	return false
}

func GetDesktopEnvironment() string {
	return ""
}
