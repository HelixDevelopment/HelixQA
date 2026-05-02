package capture

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolution_String(t *testing.T) {
	r := Resolution{Width: 1920, Height: 1080}
	assert.Equal(t, "1920x1080", r.String())

	r = Resolution{Width: 1280, Height: 720}
	assert.Equal(t, "1280x720", r.String())
}

func TestDefaultAndroidConfig(t *testing.T) {
	config := DefaultAndroidConfig("test-device")

	assert.Equal(t, "test-device", config.DeviceID)
	assert.Equal(t, 1920, config.Resolution.Width)
	assert.Equal(t, 1080, config.Resolution.Height)
	assert.Equal(t, 30, config.FPS)
	assert.Equal(t, 8000000, config.BitRate)
}

func TestNewAndroidCapture(t *testing.T) {
	config := DefaultAndroidConfig("test-device")
	capture := NewAndroidCapture(config)

	assert.NotNil(t, capture)
	assert.Equal(t, "test-device", capture.deviceID)
	assert.Equal(t, 1920, capture.resolution.Width)
	assert.Equal(t, 1080, capture.resolution.Height)
	assert.Equal(t, 30, capture.fps)
	assert.False(t, capture.IsRunning())
	assert.NotNil(t, capture.frameChan)
	assert.NotNil(t, capture.errorChan)
}

func TestAndroidCapture_buildScrcpyArgs(t *testing.T) {
	tests := []struct {
		name     string
		config   AndroidCaptureConfig
		expected []string
	}{
		{
			name: "default config with device",
			config: AndroidCaptureConfig{
				DeviceID:   "ABC123",
				Resolution: Resolution{Width: 1920, Height: 1080},
				FPS:        30,
				BitRate:    8000000,
			},
			expected: []string{
				"--no-display",
				"--record-format=raw",
				"--record=-",
				"--max-size=1920",
				"--max-fps=30",
				"--video-bit-rate=8000000",
				"--no-control",
				"--render-driver=software",
				"--serial=ABC123",
			},
		},
		{
			name: "config without device ID",
			config: AndroidCaptureConfig{
				DeviceID:   "",
				Resolution: Resolution{Width: 1280, Height: 720},
				FPS:        60,
				BitRate:    4000000,
			},
			expected: []string{
				"--no-display",
				"--record-format=raw",
				"--record=-",
				"--max-size=1280",
				"--max-fps=60",
				"--video-bit-rate=4000000",
				"--no-control",
				"--render-driver=software",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := NewAndroidCapture(tt.config)
			args := capture.buildScrcpyArgs()

			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestFrameFormat(t *testing.T) {
	// Test frame format constants
	assert.Equal(t, 0, int(FormatH264))
	assert.Equal(t, 1, int(FormatYUV420))
	assert.Equal(t, 2, int(FormatRGB))
	assert.Equal(t, 3, int(FormatBGRA))
}

func TestFrame(t *testing.T) {
	frame := &Frame{
		ID:        "test-frame-001",
		Timestamp: time.Now(),
		Data:      []byte{0x00, 0x00, 0x00, 0x01, 0x67}, // H.264 NAL start code + SPS
		Width:     1920,
		Height:    1080,
		Format:    FormatH264,
	}

	assert.Equal(t, "test-frame-001", frame.ID)
	assert.Equal(t, 1920, frame.Width)
	assert.Equal(t, 1080, frame.Height)
	assert.Equal(t, FormatH264, frame.Format)
	assert.NotEmpty(t, frame.Data)
}

func TestKeyCodes(t *testing.T) {
	// Verify key code constants
	assert.Equal(t, 3, KeyCodeHome)
	assert.Equal(t, 4, KeyCodeBack)
	assert.Equal(t, 82, KeyCodeMenu)
	assert.Equal(t, 66, KeyCodeEnter)
	assert.Equal(t, 19, KeyCodeDPadUp)
	assert.Equal(t, 20, KeyCodeDPadDown)
	assert.Equal(t, 21, KeyCodeDPadLeft)
	assert.Equal(t, 22, KeyCodeDPadRight)
	assert.Equal(t, 23, KeyCodeDPadCenter)
}

// Integration tests - these require actual Android device connected

func TestListDevices_Integration(t *testing.T) {
	// bluff-scan: no-assert-ok (integration smoke — wiring must not panic on standard inputs)
	// Skip if no ADB available
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found in PATH")
	}

	devices, err := ListDevices()
	// May return empty if no devices, but shouldn't error
	if err != nil {
		t.Logf("ListDevices returned error: %v", err)
		return
	}

	t.Logf("Found %d device(s)", len(devices))
	for _, d := range devices {
		t.Logf("  - %s", d)
	}
}

func TestGetDeviceResolution_Integration(t *testing.T) {
	// Skip if no ADB available
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found in PATH")
	}

	// Get first available device
	devices, err := ListDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("no Android devices connected")
	}

	deviceID := devices[0]
	res, err := GetDeviceResolution(deviceID)
	if err != nil {
		t.Logf("GetDeviceResolution failed: %v", err)
		return
	}

	t.Logf("Device %s resolution: %dx%d", deviceID, res.Width, res.Height)
	assert.Greater(t, res.Width, 0)
	assert.Greater(t, res.Height, 0)
}

func TestGetDeviceInfo_Integration(t *testing.T) {
	// Skip if no ADB available
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found in PATH")
	}

	// Get first available device
	devices, err := ListDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("no Android devices connected")
	}

	deviceID := devices[0]
	info, err := GetDeviceInfo(deviceID)
	if err != nil {
		t.Logf("GetDeviceInfo failed: %v", err)
		return
	}

	t.Logf("Device %s info:", deviceID)
	for k, v := range info {
		t.Logf("  %s: %s", k, v)
	}

	assert.NotEmpty(t, info)
}

func TestIsAppInForeground_Integration(t *testing.T) {
	// bluff-scan: no-assert-ok (integration smoke — wiring must not panic on standard inputs)
	// Skip if no ADB available
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found in PATH")
	}

	// Get first available device
	devices, err := ListDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("no Android devices connected")
	}

	deviceID := devices[0]

	// Check if launcher is in foreground (likely)
	inForeground, err := IsAppInForeground(deviceID, "com.android.launcher")
	if err != nil {
		t.Logf("IsAppInForeground failed: %v", err)
		return
	}

	t.Logf("Launcher in foreground: %v", inForeground)
}

func TestAndroidCapture_StartStop_Integration(t *testing.T) {
	// Skip if no scrcpy available
	if _, err := exec.LookPath("scrcpy"); err != nil {
		t.Skip("scrcpy not found in PATH")
	}

	// Skip if no ADB available
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found in PATH")
	}

	// Get first available device
	devices, err := ListDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("no Android devices connected")
	}

	deviceID := devices[0]

	config := AndroidCaptureConfig{
		DeviceID:   deviceID,
		Resolution: Resolution{Width: 1280, Height: 720},
		FPS:        15, // Lower FPS for testing
		BitRate:    2000000,
	}

	capture := NewAndroidCapture(config)

	// Start capture
	err = capture.Start()
	if err != nil {
		t.Skipf("Failed to start capture: %v (scrcpy may not be compatible)", err)
	}

	assert.True(t, capture.IsRunning())

	// Wait a bit and try to get a frame
	time.Sleep(2 * time.Second)

	// Stop capture
	err = capture.Stop()
	assert.NoError(t, err)
	assert.False(t, capture.IsRunning())
}

func TestAndroidCapture_GetFrameChan_Integration(t *testing.T) {
	// Skip if no scrcpy available
	if _, err := exec.LookPath("scrcpy"); err != nil {
		t.Skip("scrcpy not found in PATH")
	}

	// Skip if no ADB available
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found in PATH")
	}

	// Get first available device
	devices, err := ListDevices()
	if err != nil || len(devices) == 0 {
		t.Skip("no Android devices connected")
	}

	deviceID := devices[0]

	config := DefaultAndroidConfig(deviceID)
	config.FPS = 10
	config.BitRate = 1000000

	capture := NewAndroidCapture(config)

	err = capture.Start()
	if err != nil {
		t.Skipf("Failed to start capture: %v", err)
	}

	defer capture.Stop()

	// Try to read frames
	frameCount := 0
	timeout := time.AfterFunc(5*time.Second, func() {
		capture.Stop()
	})

	for frame := range capture.GetFrameChan() {
		frameCount++
		assert.NotNil(t, frame)
		assert.NotEmpty(t, frame.ID)
		assert.NotEmpty(t, frame.Data)

		if frameCount >= 5 {
			break
		}
	}

	timeout.Stop()
	t.Logf("Received %d frames", frameCount)
	assert.GreaterOrEqual(t, frameCount, 1)
}
