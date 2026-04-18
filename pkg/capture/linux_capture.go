//go:build linux
// +build linux

package capture

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// linuxCapture implements desktop capture for Linux using PipeWire/portal
type linuxCapture struct {
	parent     *DesktopCapture
	config     DesktopCaptureConfig
	cmd        *exec.Cmd
	stdout     io.ReadCloser
	pipewireFd int
}

// newLinuxCapture creates a new Linux capture instance
func newLinuxCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return &linuxCapture{
		parent: parent,
		config: config,
	}, nil
}

// Start begins capturing video using GStreamer with PipeWire
func (lc *linuxCapture) Start() error {
	// Check for PipeWire or fallback to X11
	if CommandExists("pipewire") && CommandExists("xdg-desktop-portal") {
		return lc.startPipeWireCapture()
	}

	// Fallback to X11 capture
	return lc.startX11Capture()
}

// startPipeWireCapture uses PipeWire portal for Wayland/X11 capture
func (lc *linuxCapture) startPipeWireCapture() error {
	// Use GStreamer with pipewiresrc
	// This works on both Wayland and X11
	args := []string{
		"-q", // Quiet mode
	}

	// Build pipeline
	pipeline := lc.buildPipeWirePipeline()
	args = append(args, pipeline)

	lc.cmd = exec.CommandContext(lc.parent.ctx, "gst-launch-1.0", args...)

	// Get stdout pipe for video data
	stdout, err := lc.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	lc.stdout = stdout

	// Start GStreamer
	if err := lc.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GStreamer: %w", err)
	}

	// Start reading frames
	go lc.readFrames()

	return nil
}

// buildPipeWirePipeline builds GStreamer pipeline for PipeWire capture
func (lc *linuxCapture) buildPipeWirePipeline() string {
	// PipeWire source -> converter -> encoder -> output
	pipeline := fmt.Sprintf(
		"pipewiresrc ! "+
			"video/x-raw,framerate=%d/1,width=%d,height=%d ! "+
			"videoconvert ! "+
			"x264enc tune=zerolatency speed-preset=ultrafast ! "+
			"video/x-h264,stream-format=byte-stream ! "+
			"fdsink fd=1",
		lc.config.FPS,
		lc.config.Resolution.Width,
		lc.config.Resolution.Height,
	)

	return pipeline
}

// startX11Capture uses ximagesrc for X11 capture
func (lc *linuxCapture) startX11Capture() error {
	args := []string{
		"-q",
	}

	pipeline := lc.buildX11Pipeline()
	args = append(args, pipeline)

	lc.cmd = exec.CommandContext(lc.parent.ctx, "gst-launch-1.0", args...)

	stdout, err := lc.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	lc.stdout = stdout

	// Set X11 display
	if lc.config.Display != "" {
		lc.cmd.Env = append(os.Environ(), "DISPLAY="+lc.config.Display)
	}

	if err := lc.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GStreamer: %w", err)
	}

	go lc.readFrames()

	return nil
}

// buildX11Pipeline builds GStreamer pipeline for X11 capture
func (lc *linuxCapture) buildX11Pipeline() string {
	source := "ximagesrc"

	// Add window ID if capturing specific window
	if lc.config.Source == "window" && lc.config.WindowID != "" {
		source = fmt.Sprintf("ximagesrc xid=%s", lc.config.WindowID)
	} else if lc.config.Display != "" {
		source = fmt.Sprintf("ximagesrc display-name=%s", lc.config.Display)
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
		lc.config.FPS,
		lc.config.Resolution.Width,
		lc.config.Resolution.Height,
	)

	return pipeline
}

// readFrames reads H.264 frames from GStreamer output
func (lc *linuxCapture) readFrames() {
	reader := bufio.NewReader(lc.stdout)
	frameCount := 0

	for {
		select {
		case <-lc.parent.ctx.Done():
			return
		default:
		}

		// Read H.264 NAL units
		data := make([]byte, 1024*1024)
		n, err := reader.Read(data)
		if err != nil {
			if err != io.EOF {
				select {
				case lc.parent.errorChan <- fmt.Errorf("read error: %w", err):
				default:
				}
			}
			return
		}

		if n > 0 {
			frameCount++
			frame := &Frame{
				ID:        fmt.Sprintf("desktop-frame-%d-%d", time.Now().UnixNano(), frameCount),
				Timestamp: time.Now(),
				Data:      data[:n],
				Format:    FormatH264,
				Width:     lc.config.Resolution.Width,
				Height:    lc.config.Resolution.Height,
			}

			select {
			case lc.parent.frameChan <- frame:
			case <-lc.parent.ctx.Done():
				return
			}
		}
	}
}

// Stop stops the capture
func (lc *linuxCapture) Stop() error {
	if lc.cmd != nil && lc.cmd.Process != nil {
		lc.cmd.Process.Kill()
		lc.cmd.Wait()
	}

	if lc.stdout != nil {
		lc.stdout.Close()
	}

	return nil
}

// IsRunning returns true if capture is active
func (lc *linuxCapture) IsRunning() bool {
	return lc.cmd != nil && lc.cmd.Process != nil
}

// GetFrameChan returns the frame channel
func (lc *linuxCapture) GetFrameChan() <-chan *Frame {
	return lc.parent.frameChan
}

// listLinuxDisplays lists available X11/Wayland displays
func listLinuxDisplays() ([]Display, error) {
	var displays []Display

	// Try xrandr first
	if CommandExists("xrandr") {
		cmd := exec.Command("xrandr", "--listmonitors")
		output, err := cmd.Output()
		if err == nil {
			displays = parseXrandrOutput(string(output))
		}
	}

	// If no displays found, add default
	if len(displays) == 0 {
		displays = append(displays, Display{
			ID:      ":0",
			Name:    "Default Display",
			Primary: true,
		})
	}

	return displays, nil
}

// parseXrandrOutput parses xrandr --listmonitors output
func parseXrandrOutput(output string) []Display {
	var displays []Display
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse:  0: +*DP-1 1920/531x1080/299+0+0  DP-1
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			display := Display{
				ID:      parts[0],
				Name:    parts[len(parts)-1],
				Primary: strings.Contains(line, "*"),
			}

			// Parse resolution (e.g., "1920/531x1080/299")
			resParts := strings.Split(parts[3], "x")
			if len(resParts) == 2 {
				widthStr := strings.Split(resParts[0], "/")[0]
				heightStr := strings.Split(resParts[1], "/")[0]
				display.Width, _ = strconv.Atoi(widthStr)
				display.Height, _ = strconv.Atoi(heightStr)
			}

			displays = append(displays, display)
		}
	}

	return displays
}

// listLinuxWindows lists available windows using xdotool or wmctrl
func listLinuxWindows() ([]Window, error) {
	var windows []Window

	if CommandExists("xdotool") {
		// Get window list
		cmd := exec.Command("xdotool", "search", "--onlyvisible", "--class", ".*")
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		windowIDs := strings.Fields(string(output))
		for _, id := range windowIDs {
			window, err := getWindowInfo(id)
			if err == nil && window.Title != "" {
				windows = append(windows, window)
			}
		}
	} else if CommandExists("wmctrl") {
		cmd := exec.Command("wmctrl", "-l", "-G")
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		windows = parseWmctrlOutput(string(output))
	}

	return windows, nil
}

// getWindowInfo gets detailed info about a window
func getWindowInfo(windowID string) (Window, error) {
	window := Window{ID: windowID}

	// Get window title
	cmd := exec.Command("xdotool", "getwindowname", windowID)
	output, err := cmd.Output()
	if err == nil {
		window.Title = strings.TrimSpace(string(output))
	}

	// Get window geometry
	cmd = exec.Command("xdotool", "getwindowgeometry", windowID)
	output, err = cmd.Output()
	if err == nil {
		parseXdotoolGeometry(string(output), &window)
	}

	// Get window class (app name)
	cmd = exec.Command("xprop", "-id", windowID, "WM_CLASS")
	output, err = cmd.Output()
	if err == nil {
		window.AppName = parseWindowClass(string(output))
	}

	return window, nil
}

// parseXdotoolGeometry parses xdotool getwindowgeometry output
func parseXdotoolGeometry(output string, window *Window) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Position:") {
			// Parse: Position: 100,200 (screen: 0)
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				pos := strings.TrimSuffix(parts[1], ",")
				window.X, _ = strconv.Atoi(pos)
			}
			if len(parts) >= 3 {
				pos := strings.TrimSuffix(parts[2], " (screen:")
				window.Y, _ = strconv.Atoi(pos)
			}
		} else if strings.Contains(line, "Geometry:") {
			// Parse: Geometry: 1920x1080
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				geom := strings.TrimSpace(parts[1])
				dims := strings.Split(geom, "x")
				if len(dims) == 2 {
					window.Width, _ = strconv.Atoi(dims[0])
					window.Height, _ = strconv.Atoi(dims[1])
				}
			}
		}
	}
}

// parseWindowClass parses xprop WM_CLASS output
func parseWindowClass(output string) string {
	// Parse: WM_CLASS(STRING) = "firefox", "Firefox"
	if idx := strings.Index(output, "\""); idx != -1 {
		parts := strings.Split(output[idx:], "\"")
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return ""
}

// parseWmctrlOutput parses wmctrl -l -G output
func parseWmctrlOutput(output string) []Window {
	var windows []Window
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 7 {
			window := Window{
				ID:    parts[0],
				X:     parseInt(parts[1]),
				Y:     parseInt(parts[2]),
				Width: parseInt(parts[5]),
			}
			// Height includes decoration, so we use a different field
			window.Height = parseInt(parts[6])

			// Title is everything after column 7
			if len(parts) > 7 {
				window.Title = strings.Join(parts[7:], " ")
			}

			windows = append(windows, window)
		}
	}

	return windows
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// captureLinuxScreenshot captures screenshot using various methods
func captureLinuxScreenshot(outputPath string) error {
	// Try gnome-screenshot first
	if CommandExists("gnome-screenshot") {
		cmd := exec.Command("gnome-screenshot", "-f", outputPath)
		return cmd.Run()
	}

	// Try import (ImageMagick)
	if CommandExists("import") {
		cmd := exec.Command("import", "-window", "root", outputPath)
		return cmd.Run()
	}

	// Try scrot
	if CommandExists("scrot") {
		cmd := exec.Command("scrot", outputPath)
		return cmd.Run()
	}

	// Fallback to GStreamer
	pipeline := fmt.Sprintf(
		"ximagesrc ! videoconvert ! pngenc ! filesink location=%s",
		outputPath,
	)
	cmd := exec.Command("gst-launch-1.0", "-q", pipeline)
	return cmd.Run()
}

// verifyLinuxSupport checks if Linux system supports capture
func verifyLinuxSupport() error {
	// Check for GStreamer
	if !CommandExists("gst-launch-1.0") {
		return fmt.Errorf("GStreamer not found. Install with: sudo apt install gstreamer1.0-tools")
	}

	// Check for X11 or Wayland
	display := os.Getenv("DISPLAY")
	wayland := os.Getenv("WAYLAND_DISPLAY")

	if display == "" && wayland == "" {
		return fmt.Errorf("no display found (set DISPLAY or WAYLAND_DISPLAY)")
	}

	// Check for capture source
	hasX11 := CommandExists("ximagesrc") || display != ""
	hasPipeWire := CommandExists("pipewire")

	if !hasX11 && !hasPipeWire {
		return fmt.Errorf("no capture source available (need ximagesrc or pipewire)")
	}

	return nil
}

// PipeWireMonitor monitors PipeWire for display changes
type PipeWireMonitor struct {
	events chan PipeWireEvent
}

// PipeWireEvent represents a PipeWire event
type PipeWireEvent struct {
	Type   string // "add", "remove", "change"
	NodeID int
	Name   string
}

// NewPipeWireMonitor creates a new PipeWire monitor
func NewPipeWireMonitor() (*PipeWireMonitor, error) {
	if !CommandExists("pw-dump") {
		return nil, fmt.Errorf("pw-dump not found")
	}

	return &PipeWireMonitor{
		events: make(chan PipeWireEvent, 100),
	}, nil
}

// Start begins monitoring PipeWire
func (pm *PipeWireMonitor) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "pw-dump", "--monitor")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go pm.parseEvents(stdout)

	return cmd.Wait()
}

// parseEvents parses PipeWire events
func (pm *PipeWireMonitor) parseEvents(r io.Reader) {
	decoder := json.NewDecoder(r)

	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			return
		}

		// Parse event and send to channel
		// This is a simplified version
		_ = event
	}
}

// GetEvents returns the event channel
func (pm *PipeWireMonitor) GetEvents() <-chan PipeWireEvent {
	return pm.events
}

// GetDesktopEnvironment returns the current desktop environment
func GetDesktopEnvironment() string {
	de := os.Getenv("XDG_CURRENT_DESKTOP")
	if de == "" {
		de = os.Getenv("DESKTOP_SESSION")
	}
	if de == "" {
		de = os.Getenv("GNOME_DESKTOP_SESSION_ID")
		if de != "" {
			return "GNOME"
		}
	}
	return de
}

// IsWayland returns true if running on Wayland
func IsWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

// ScreenshotOptions options for screenshot
type ScreenshotOptions struct {
	WindowID string
	X, Y     int
	Width    int
	Height   int
	Output   string
}

// CaptureRegion captures a specific region
func CaptureRegion(opts ScreenshotOptions) error {
	// Use GStreamer for region capture
	pipeline := fmt.Sprintf(
		"ximagesrc startx=%d starty=%d endx=%d endy=%d ! "+
			"videoconvert ! pngenc ! filesink location=%s",
		opts.X, opts.Y,
		opts.X+opts.Width, opts.Y+opts.Height,
		opts.Output,
	)

	cmd := exec.Command("gst-launch-1.0", "-q", pipeline)
	return cmd.Run()
}

// SaveFrame saves a frame to disk
func SaveFrame(frame *Frame, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("frame-%s.h264", frame.ID))

	// If frame is H.264, save directly
	if frame.Format == FormatH264 {
		if err := os.WriteFile(filename, frame.Data, 0644); err != nil {
			return "", err
		}
		return filename, nil
	}

	// Otherwise convert using GStreamer
	return filename, fmt.Errorf("frame format not supported: %v", frame.Format)
}

// Stub functions for Windows and macOS (only compiled on Linux)

func newWindowsCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return nil, fmt.Errorf("Windows capture not available on Linux")
}

func listWindowsDisplays() ([]Display, error) {
	return nil, fmt.Errorf("Windows displays not available on Linux")
}

func listWindowsWindows() ([]Window, error) {
	return nil, fmt.Errorf("Windows windows not available on Linux")
}

func captureWindowsScreenshot(outputPath string) error {
	return fmt.Errorf("Windows screenshot not available on Linux")
}

func verifyWindowsSupport() error {
	return fmt.Errorf("Windows capture not available on Linux")
}

func newMacOSCapture(parent *DesktopCapture, config DesktopCaptureConfig) (desktopCaptureImpl, error) {
	return nil, fmt.Errorf("macOS capture not available on Linux")
}

func listMacOSDisplays() ([]Display, error) {
	return nil, fmt.Errorf("macOS displays not available on Linux")
}

func listMacOSWindows() ([]Window, error) {
	return nil, fmt.Errorf("macOS windows not available on Linux")
}

func captureMacOSScreenshot(outputPath string) error {
	return fmt.Errorf("macOS screenshot not available on Linux")
}

func verifyMacOSSupport() error {
	return fmt.Errorf("macOS capture not available on Linux")
}

func IsScreenCaptureKitAvailable() bool {
	return false
}

func CheckScreenRecordingPermission() bool {
	return false
}
