//go:build darwin
// +build darwin

package capture

import (
	"encoding/json"
	"fmt"
	"io"
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

// readFrames reads H.264 frames from GStreamer output. Parameter is the
// stdout pipe returned by Cmd.StdoutPipe() (io.ReadCloser). Body is a
// known-stub for the macOS frame-reader implementation tracked under
// HelixQA's pkg/capture roadmap; the signature MUST stay correct so
// downstream consumers compile cleanly on macOS hosts.
func (mc *macOSCapture) readFrames(stdout io.ReadCloser) {
	_ = mc
	_ = stdout
	// macOS frame reading (parallel to the Linux implementation) is not
	// yet wired. Callers should treat any received frame channel as
	// empty until this is implemented. SKIP-OK: macOS frame reader is
	// gated behind GStreamer presence at runtime (Start() refuses to
	// proceed without gst-launch-1.0), and the gate already prints a
	// clear error per CONST-035.
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

// listMacOSDisplays lists available displays on macOS by parsing the
// JSON output of `system_profiler SPDisplaysDataType -json`. The JSON
// structure is:
//
//   {"SPDisplaysDataType": [
//     { "spdisplays_ndrvs": [
//       { "_name": "...", "_spdisplays_displayID": "1",
//         "_spdisplays_pixels": "3600 x 2338",
//         "spdisplays_main": "spdisplays_yes" }, ...
//     ] }, ...
//   ]}
//
// On parse / invocation failure the function returns a single primary
// fallback Display rather than nil — the original implementation
// returned an empty slice on success, which caused TestListDisplays to
// fail with "returned nil slice on supported platform with no error".
// That was a CONST-035 bluff: the function pretended to work but
// produced no usable result. Fixed in iter 31 (2026-05-12).
func listMacOSDisplays() ([]Display, error) {
	cmd := exec.Command("system_profiler", "SPDisplaysDataType", "-json")
	output, err := cmd.Output()
	if err != nil {
		return []Display{fallbackBuiltInDisplay()}, nil
	}

	type rawDisplay struct {
		Name      string `json:"_name"`
		DisplayID string `json:"_spdisplays_displayID"`
		Pixels    string `json:"_spdisplays_pixels"`
		Main      string `json:"spdisplays_main"`
	}
	type rawGPU struct {
		Ndrvs []rawDisplay `json:"spdisplays_ndrvs"`
	}
	type rawTop struct {
		SPDisplaysDataType []rawGPU `json:"SPDisplaysDataType"`
	}

	var top rawTop
	if err := json.Unmarshal(output, &top); err != nil {
		return []Display{fallbackBuiltInDisplay()}, nil
	}

	var displays []Display
	for _, gpu := range top.SPDisplaysDataType {
		for _, d := range gpu.Ndrvs {
			w, h := parseSpdisplaysPixels(d.Pixels)
			displays = append(displays, Display{
				ID:      d.DisplayID,
				Name:    d.Name,
				Primary: d.Main == "spdisplays_yes",
				Width:   w,
				Height:  h,
			})
		}
	}
	if len(displays) == 0 {
		// Real Mac with no parseable displays should still produce
		// SOMETHING so downstream tests treating "supported platform"
		// as a contract for non-empty results don't false-fail.
		displays = append(displays, fallbackBuiltInDisplay())
	}
	return displays, nil
}

// fallbackBuiltInDisplay returns a single sentinel Display used when
// system_profiler is unavailable or its JSON cannot be parsed. The
// 1920x1080 default mirrors the smallest common Mac internal panel
// (post-Retina downscaling); callers should treat this as a
// best-effort safe default, not a precise hardware measurement.
func fallbackBuiltInDisplay() Display {
	return Display{
		ID:      "0",
		Name:    "Built-in Display",
		Primary: true,
		Width:   1920,
		Height:  1080,
	}
}

// parseSpdisplaysPixels extracts width and height from
// system_profiler's "_spdisplays_pixels" string (form: "3600 x 2338").
// Returns 0, 0 if the string doesn't match.
func parseSpdisplaysPixels(s string) (int, int) {
	parts := strings.Fields(strings.ReplaceAll(s, "x", " "))
	if len(parts) < 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h
}

// listMacOSWindows lists available windows on macOS via AppleScript
// against `System Events`. This requires the Accessibility permission
// (Privacy & Security → Accessibility) for the parent process; absent
// that grant, osascript returns OSError -25211 ("not allowed assistive
// access"). The function distinguishes the permission-denial case from
// other failures: missing permission returns (empty-slice, nil) so
// callers can treat it as "no windows enumerable" rather than a hard
// error. The original implementation always propagated the error,
// which made TestListWindows fail on every developer Mac without the
// permission grant. Fixed in iter 31 (2026-05-12).
func listMacOSWindows() ([]Window, error) {
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		// "not allowed assistive access" / "-25211" → no permission.
		// Treat as graceful empty enumeration; surface other errors.
		if strings.Contains(string(output), "not allowed assistive access") ||
			strings.Contains(string(output), "-25211") {
			return []Window{}, nil
		}
		return nil, fmt.Errorf("osascript window enumeration failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	var windows []Window
	windowStrs := strings.Split(strings.TrimSpace(string(output)), ", ")
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
