// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constructor tests ---

func TestNewDualDisplayDetector_Defaults(t *testing.T) {
	d := NewDualDisplayDetector("device123")
	assert.Equal(t, "device123", d.device)
	assert.Equal(t, 0, d.primaryDisplayID)
	assert.Equal(t, -1, d.secondaryDisplayID)
	assert.Equal(t, "evidence", d.evidenceDir)
	assert.NotNil(t, d.cmdRunner)
}

func TestNewDualDisplayDetector_WithOptions(t *testing.T) {
	mock := newMockRunner()
	d := NewDualDisplayDetector(
		"device123",
		WithSecondaryDisplayID(3),
		WithDualDisplayCommandRunner(mock),
		WithDualDisplayEvidenceDir("/tmp/ev"),
	)
	assert.Equal(t, 3, d.secondaryDisplayID)
	assert.Equal(t, "/tmp/ev", d.evidenceDir)
}

// --- DetectDisplays tests ---

func TestDetectDisplays_TwoDisplays(t *testing.T) {
	mock := newMockRunner()
	dumpsys := `Displays:
  Display Devices: size=2
  DisplayDeviceInfo
    mDisplayId=0
    mName=HDMI-A-2
    1024 x 600
  DisplayDeviceInfo
    mDisplayId=3
    mName=HDMI-A-1
    1920 x 1080
`
	mock.On(
		"adb -s dev1 shell dumpsys display",
		[]byte(dumpsys),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	displays, err := d.DetectDisplays(context.Background())
	require.NoError(t, err)
	assert.Len(t, displays, 2)

	assert.Equal(t, 0, displays[0].ID)
	assert.Equal(t, "HDMI-A-2", displays[0].Name)
	assert.True(t, displays[0].Connected)
	assert.Equal(t, "PRIMARY", displays[0].Type)

	assert.Equal(t, 3, displays[1].ID)
	assert.Equal(t, "HDMI-A-1", displays[1].Name)
	assert.True(t, displays[1].Connected)
	assert.Equal(t, "EXTERNAL", displays[1].Type)
}

func TestDetectDisplays_SingleDisplay(t *testing.T) {
	mock := newMockRunner()
	dumpsys := `Displays:
  Display Devices: size=1
  DisplayDeviceInfo
    mDisplayId=0
    mName=Built-in Screen
    1920 x 1080
`
	mock.On(
		"adb -s dev1 shell dumpsys display",
		[]byte(dumpsys),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	displays, err := d.DetectDisplays(context.Background())
	require.NoError(t, err)
	assert.Len(t, displays, 1)
	assert.Equal(t, "PRIMARY", displays[0].Type)
}

func TestDetectDisplays_Error(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s dev1 shell dumpsys display",
		nil,
		fmt.Errorf("device offline"),
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	_, err := d.DetectDisplays(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dumpsys display")
}

// --- ScreenshotDisplay tests ---

func TestScreenshotDisplay_Success(t *testing.T) {
	mock := newMockRunner()
	pngData := []byte{0x89, 0x50, 0x4E, 0x47}
	mock.On(
		"adb -s dev1 shell screencap -d 0 -p",
		pngData,
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	data, err := d.ScreenshotDisplay(
		context.Background(), 0,
	)
	require.NoError(t, err)
	assert.Equal(t, pngData, data)
}

func TestScreenshotDisplay_Error(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s dev1 shell screencap -d 3 -p",
		nil,
		fmt.Errorf("display not found"),
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	_, err := d.ScreenshotDisplay(
		context.Background(), 3,
	)
	assert.Error(t, err)
}

// --- CheckFrozenFrame tests ---

func TestCheckFrozenFrame_Detected(t *testing.T) {
	mock := newMockRunner()
	logOutput := `03-20 12:00:01.123 W/C2BqBuffer: last successful dequeue was 5000000 us ago
03-20 12:00:01.456 W/C2BqBuffer: last successful dequeue was 5200000 us ago
`
	mock.On(
		"adb -s dev1 logcat -d -s C2BqBuffer:W",
		[]byte(logOutput),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	frozen, duration, err := d.CheckFrozenFrame(
		context.Background(),
	)
	require.NoError(t, err)
	assert.True(t, frozen)
	assert.True(t, duration.Seconds() > 2.0)
}

func TestCheckFrozenFrame_NotFrozen(t *testing.T) {
	mock := newMockRunner()
	logOutput := `03-20 12:00:01.123 W/C2BqBuffer: last successful dequeue was 500000 us ago
`
	mock.On(
		"adb -s dev1 logcat -d -s C2BqBuffer:W",
		[]byte(logOutput),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	frozen, _, err := d.CheckFrozenFrame(
		context.Background(),
	)
	require.NoError(t, err)
	assert.False(t, frozen)
}

func TestCheckFrozenFrame_NoLogs(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s dev1 logcat -d -s C2BqBuffer:W",
		[]byte(""),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	frozen, duration, err := d.CheckFrozenFrame(
		context.Background(),
	)
	require.NoError(t, err)
	assert.False(t, frozen)
	assert.Zero(t, duration)
}

// --- CheckPresenter tests ---

// presenterPkgForTest is a test-only fixture identifying the app
// whose service state is probed by presenter-dependent tests.
// Declared here (once) so the tests can share a single value and
// keep the library code itself project-agnostic (HelixQA
// Constitution §1).
const presenterPkgForTest = "com.atmosphere.presenter"

func TestCheckPresenter_Running(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s dev1 shell pidof com.atmosphere.presenter",
		[]byte("1234"),
		nil,
	)
	dumpsys := `Service com.atmosphere.presenter:
  videoMode=true
  albumCoverMode=false
  secondaryDisplayId=3
`
	mock.On(
		"adb -s dev1 shell dumpsys activity services com.atmosphere.presenter",
		[]byte(dumpsys),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
		WithPresenterPackage(presenterPkgForTest),
	)

	status, err := d.CheckPresenter(context.Background())
	require.NoError(t, err)
	assert.True(t, status.ServiceAlive)
	assert.True(t, status.VideoMode)
	assert.False(t, status.AlbumCoverMode)
	assert.Equal(t, 3, status.SecondaryDisplayID)
}

func TestCheckPresenter_NotRunning(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s dev1 shell pidof com.atmosphere.presenter",
		[]byte(""),
		fmt.Errorf("exit code 1"),
	)
	mock.On(
		"adb -s dev1 shell dumpsys activity services com.atmosphere.presenter",
		[]byte("(nothing)"),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
		WithPresenterPackage(presenterPkgForTest),
	)

	status, err := d.CheckPresenter(context.Background())
	require.NoError(t, err)
	assert.False(t, status.ServiceAlive)
}

// --- CheckMediaSession tests ---

func TestCheckMediaSession_Playing(t *testing.T) {
	mock := newMockRunner()
	dumpsys := `Media Session Service
  Sessions Stack - these are the current sessions:
    package=com.google.android.youtube (uid=10123)
      state=PlaybackState {state=3, position=12345}
      metadata: description=title=Never Gonna Give You Up, subtitle=Rick Astley, bitmap
`
	mock.On(
		"adb -s dev1 shell dumpsys media_session",
		[]byte(dumpsys),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	info, err := d.CheckMediaSession(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "com.google.android.youtube", info.PackageName)
	assert.Equal(t, "PLAYING", info.State)
	assert.Equal(t, "Never Gonna Give You Up", info.Title)
	assert.Equal(t, "Rick Astley", info.Artist)
}

func TestCheckMediaSession_Stopped(t *testing.T) {
	mock := newMockRunner()
	dumpsys := `Media Session Service
  Sessions Stack - these are the current sessions:
    package=com.spotify.music (uid=10200)
      state=PlaybackState {state=1, position=0}
`
	mock.On(
		"adb -s dev1 shell dumpsys media_session",
		[]byte(dumpsys),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	info, err := d.CheckMediaSession(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "STOPPED", info.State)
}

func TestCheckMediaSession_NoSession(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"adb -s dev1 shell dumpsys media_session",
		[]byte("Media Session Service\n  Sessions Stack:\n"),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
	)

	info, err := d.CheckMediaSession(context.Background())
	require.NoError(t, err)
	assert.Empty(t, info.PackageName)
	assert.Empty(t, info.State)
}

// --- CheckVideoRouting tests ---

func TestCheckVideoRouting_Active(t *testing.T) {
	mock := newMockRunner()
	// MediaSession check.
	sessionDumpsys := `Media Session Service
  Sessions Stack - these are the current sessions:
    package=com.google.android.youtube (uid=10123)
      state=PlaybackState {state=3, position=100}
`
	mock.On(
		"adb -s dev1 shell dumpsys media_session",
		[]byte(sessionDumpsys),
		nil,
	)
	// Presenter check.
	presenterDumpsys := `Service com.atmosphere.presenter:
  activeDecoder=c2.android.hevc.decoder
  secondaryDisplayId=3
  surfaceValid=true
`
	mock.On(
		"adb -s dev1 shell dumpsys activity services com.atmosphere.presenter",
		[]byte(presenterDumpsys),
		nil,
	)
	// Frozen frame check.
	mock.On(
		"adb -s dev1 logcat -d -s C2BqBuffer:W",
		[]byte(""),
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
		WithPresenterPackage(presenterPkgForTest),
	)

	result, err := d.CheckVideoRouting(
		context.Background(),
	)
	require.NoError(t, err)
	assert.True(t, result.VideoPlaying)
	assert.Equal(t, "c2.android.hevc.decoder", result.ActiveDecoder)
	assert.True(t, result.SurfaceValid)
}

// --- CheckAll tests ---

func TestCheckAll_FullCheck(t *testing.T) {
	mock := newMockRunner()
	// Display detection.
	displayDumpsys := `Displays:
  Display Devices: size=2
  DisplayDeviceInfo
    mDisplayId=0
    mName=HDMI-A-2
    1024 x 600
  DisplayDeviceInfo
    mDisplayId=3
    mName=HDMI-A-1
    1920 x 1080
`
	mock.On(
		"adb -s dev1 shell dumpsys display",
		[]byte(displayDumpsys),
		nil,
	)
	// Presenter.
	mock.On(
		"adb -s dev1 shell pidof com.atmosphere.presenter",
		[]byte("5678"),
		nil,
	)
	presenterDumpsys := `Service com.atmosphere.presenter:
  videoMode=false
  albumCoverMode=true
  secondaryDisplayId=3
`
	mock.On(
		"adb -s dev1 shell dumpsys activity services com.atmosphere.presenter",
		[]byte(presenterDumpsys),
		nil,
	)
	// MediaSession.
	sessionDumpsys := `Media Session Service
  Sessions Stack - these are the current sessions:
    package=com.spotify.music (uid=10200)
      state=PlaybackState {state=3, position=500}
      metadata: description=title=Bohemian Rhapsody, subtitle=Queen
`
	mock.On(
		"adb -s dev1 shell dumpsys media_session",
		[]byte(sessionDumpsys),
		nil,
	)
	// Frozen frame.
	mock.On(
		"adb -s dev1 logcat -d -s C2BqBuffer:W",
		[]byte(""),
		nil,
	)
	// Screenshots.
	mock.On(
		"adb -s dev1 shell screencap -d 0 -p",
		[]byte{0x89, 0x50},
		nil,
	)
	mock.On(
		"adb -s dev1 shell screencap -d 3 -p",
		[]byte{0x89, 0x50},
		nil,
	)

	d := NewDualDisplayDetector(
		"dev1",
		WithDualDisplayCommandRunner(mock),
		WithDualDisplayEvidenceDir(t.TempDir()),
		WithPresenterPackage("com.atmosphere.presenter"),
	)

	result, err := d.CheckAll(context.Background())
	require.NoError(t, err)
	assert.True(t, result.SecondaryDisplayConnected)
	assert.True(t, result.PresenterServiceAlive)
	assert.True(t, result.AlbumCoverVisible)
	assert.False(t, result.FrozenFrame)
	assert.Equal(t, "PLAYING", result.MediaSessionState)
}
