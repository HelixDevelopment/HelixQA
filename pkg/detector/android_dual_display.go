// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// presenterPackage is the ATMOSphere Presenter app package.
const presenterPackage = "com.atmosphere.presenter"

// Playback state constants matching Android PlaybackState.
const (
	PlaybackStateStopped = "STOPPED"
	PlaybackStatePaused  = "PAUSED"
	PlaybackStatePlaying = "PLAYING"
)

// displayType constants for DisplayInfo.
const (
	DisplayTypePrimary  = "PRIMARY"
	DisplayTypeExternal = "EXTERNAL"
)

// DisplayInfo describes a connected display.
type DisplayInfo struct {
	// ID is the Android display ID.
	ID int `json:"id"`

	// Name is the display name from DisplayManager.
	Name string `json:"name"`

	// Resolution is the display resolution (e.g. "1920x1080").
	Resolution string `json:"resolution"`

	// Connected indicates whether the display is active.
	Connected bool `json:"connected"`

	// Type is PRIMARY or EXTERNAL.
	Type string `json:"type"`
}

// VideoRoutingResult captures the state of video routing to
// secondary display via VideoOutputManagerService.
type VideoRoutingResult struct {
	// ActiveDecoder is the codec name handling video output.
	ActiveDecoder string `json:"active_decoder,omitempty"`

	// SecondaryDisplayID is the display receiving video.
	SecondaryDisplayID int `json:"secondary_display_id"`

	// SurfaceValid indicates the secondary surface is valid.
	SurfaceValid bool `json:"surface_valid"`

	// VideoPlaying indicates video is actively rendering.
	VideoPlaying bool `json:"video_playing"`
}

// PresenterStatus captures the Presenter service state.
type PresenterStatus struct {
	// ServiceAlive indicates Presenter service is running.
	ServiceAlive bool `json:"service_alive"`

	// VideoMode indicates Presenter is in video mode
	// (Presentation hidden, video on secondary).
	VideoMode bool `json:"video_mode"`

	// AlbumCoverMode indicates album art is shown on
	// secondary display.
	AlbumCoverMode bool `json:"album_cover_mode"`

	// SecondaryDisplayID is the display Presenter targets.
	SecondaryDisplayID int `json:"secondary_display_id"`
}

// MediaSessionInfo captures active media session state.
type MediaSessionInfo struct {
	// PackageName is the app owning the session.
	PackageName string `json:"package_name"`

	// State is PLAYING, PAUSED, or STOPPED.
	State string `json:"state"`

	// Title is the media title from metadata.
	Title string `json:"title,omitempty"`

	// Artist is the media artist from metadata.
	Artist string `json:"artist,omitempty"`

	// HasAlbumArt indicates album art is present.
	HasAlbumArt bool `json:"has_album_art"`
}

// DualDisplayResult captures the combined state of a
// dual-display Android device.
type DualDisplayResult struct {
	DetectionResult

	// PrimaryScreenshot is the local path to the primary
	// display screenshot.
	PrimaryScreenshot string `json:"primary_screenshot,omitempty"`

	// SecondaryScreenshot is the local path to the secondary
	// display screenshot.
	SecondaryScreenshot string `json:"secondary_screenshot,omitempty"`

	// SecondaryDisplayConnected indicates the TV is connected.
	SecondaryDisplayConnected bool `json:"secondary_display_connected"`

	// SecondaryDisplayResolution is the TV resolution.
	SecondaryDisplayResolution string `json:"secondary_display_resolution,omitempty"`

	// VideoOnSecondary indicates video is playing on TV.
	VideoOnSecondary bool `json:"video_on_secondary"`

	// FrozenFrame indicates a frozen frame was detected.
	FrozenFrame bool `json:"frozen_frame"`

	// FrozenFrameDuration is how long the frame has been
	// frozen.
	FrozenFrameDuration time.Duration `json:"frozen_frame_duration,omitempty"`

	// MediaSessionState is the current playback state.
	MediaSessionState string `json:"media_session_state,omitempty"`

	// ActiveCodec is the active video codec name.
	ActiveCodec string `json:"active_codec,omitempty"`

	// AlbumCoverVisible indicates album art is on secondary.
	AlbumCoverVisible bool `json:"album_cover_visible"`

	// PresenterServiceAlive indicates Presenter is running.
	PresenterServiceAlive bool `json:"presenter_service_alive"`
}

// DualDisplayOption configures a DualDisplayDetector.
type DualDisplayOption func(*DualDisplayDetector)

// DualDisplayDetector extends the base Detector with
// dual-display awareness for the ATMOSphere device (Orange Pi
// 5 Max with two HDMI outputs).
type DualDisplayDetector struct {
	detector           *Detector
	primaryDisplayID   int
	secondaryDisplayID int
	device             string
	evidenceDir        string
	cmdRunner          CommandRunner
}

// WithSecondaryDisplayID sets the secondary display ID. By
// default, the secondary display is discovered dynamically
// from dumpsys display output.
func WithSecondaryDisplayID(id int) DualDisplayOption {
	return func(d *DualDisplayDetector) {
		d.secondaryDisplayID = id
	}
}

// WithDualDisplayCommandRunner sets a custom command runner
// for testing.
func WithDualDisplayCommandRunner(
	runner CommandRunner,
) DualDisplayOption {
	return func(d *DualDisplayDetector) {
		d.cmdRunner = runner
	}
}

// WithDualDisplayEvidenceDir sets the directory for saving
// evidence files (screenshots, logs).
func WithDualDisplayEvidenceDir(
	dir string,
) DualDisplayOption {
	return func(d *DualDisplayDetector) {
		d.evidenceDir = dir
	}
}

// NewDualDisplayDetector creates a DualDisplayDetector for the
// specified ADB device serial. The primary display defaults to
// ID 0 (touch screen). The secondary display ID is discovered
// dynamically unless overridden via WithSecondaryDisplayID.
func NewDualDisplayDetector(
	device string,
	opts ...DualDisplayOption,
) *DualDisplayDetector {
	d := &DualDisplayDetector{
		primaryDisplayID:   0,
		secondaryDisplayID: -1,
		device:             device,
		evidenceDir:        "evidence",
		cmdRunner:          &execRunner{},
	}
	for _, opt := range opts {
		opt(d)
	}
	d.detector = New(
		"android",
		WithDevice(device),
		WithCommandRunner(d.cmdRunner),
		WithEvidenceDir(d.evidenceDir),
	)
	return d
}

// DetectDisplays queries the device for all connected displays
// and returns their info. It parses dumpsys display output to
// find display IDs, names, and resolutions.
func (d *DualDisplayDetector) DetectDisplays(
	ctx context.Context,
) ([]DisplayInfo, error) {
	args := d.adbArgs("shell", "dumpsys", "display")
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		return nil, fmt.Errorf("dumpsys display: %w", err)
	}

	return parseDisplays(string(output)), nil
}

// parseDisplays extracts DisplayInfo entries from dumpsys
// display output. It looks for mDisplayId=, name, resolution,
// and connection state.
func parseDisplays(output string) []DisplayInfo {
	var displays []DisplayInfo

	// Match display blocks in "Display Devices:" section.
	idRe := regexp.MustCompile(
		`mDisplayId=(\d+)`,
	)
	nameRe := regexp.MustCompile(
		`mName=(.+)`,
	)
	resRe := regexp.MustCompile(
		`(\d{3,5})\s*x\s*(\d{3,5})`,
	)

	// Split into blocks starting with "DisplayDeviceInfo".
	blocks := strings.Split(output, "DisplayDeviceInfo")
	for _, block := range blocks {
		idMatch := idRe.FindStringSubmatch(block)
		if idMatch == nil {
			continue
		}

		id, err := strconv.Atoi(idMatch[1])
		if err != nil {
			continue
		}

		info := DisplayInfo{
			ID:        id,
			Connected: true,
		}

		if nameMatch := nameRe.FindStringSubmatch(
			block,
		); nameMatch != nil {
			info.Name = strings.TrimSpace(nameMatch[1])
		}

		if resMatch := resRe.FindStringSubmatch(
			block,
		); resMatch != nil {
			info.Resolution = resMatch[1] + "x" + resMatch[2]
		}

		if id == 0 {
			info.Type = DisplayTypePrimary
		} else {
			info.Type = DisplayTypeExternal
		}

		displays = append(displays, info)
	}

	// Fallback: if no DisplayDeviceInfo blocks found, try
	// parsing Display ID lines directly.
	if len(displays) == 0 {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if idMatch := idRe.FindStringSubmatch(
				line,
			); idMatch != nil {
				id, err := strconv.Atoi(idMatch[1])
				if err != nil {
					continue
				}
				info := DisplayInfo{
					ID:        id,
					Connected: true,
				}
				if id == 0 {
					info.Type = DisplayTypePrimary
				} else {
					info.Type = DisplayTypeExternal
				}
				if resMatch := resRe.FindStringSubmatch(
					line,
				); resMatch != nil {
					info.Resolution = resMatch[1] +
						"x" + resMatch[2]
				}
				displays = append(displays, info)
			}
		}
	}

	return displays
}

// ScreenshotDisplay captures a screenshot of the specified
// display and returns the raw PNG bytes.
func (d *DualDisplayDetector) ScreenshotDisplay(
	ctx context.Context,
	displayID int,
) ([]byte, error) {
	args := d.adbArgs(
		"shell", "screencap",
		"-d", strconv.Itoa(displayID), "-p",
	)
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		return nil, fmt.Errorf(
			"screencap display %d: %w", displayID, err,
		)
	}
	return output, nil
}

// screenshotDisplayToFile captures a screenshot and saves it
// to the evidence directory, returning the local file path.
func (d *DualDisplayDetector) screenshotDisplayToFile(
	ctx context.Context,
	displayID int,
	label string,
) (string, error) {
	remotePath := fmt.Sprintf(
		"/sdcard/helixqa-%s-%d.png", label, displayID,
	)

	captureArgs := d.adbArgs(
		"shell", "screencap",
		"-d", strconv.Itoa(displayID),
		"-p", remotePath,
	)
	_, err := d.cmdRunner.Run(ctx, "adb", captureArgs...)
	if err != nil {
		return "", fmt.Errorf(
			"screencap display %d: %w", displayID, err,
		)
	}

	localName := fmt.Sprintf(
		"%s-display%d-%d.png",
		label, displayID, time.Now().UnixMilli(),
	)
	localPath := filepath.Join(d.evidenceDir, localName)

	pullArgs := d.adbArgs("pull", remotePath, localPath)
	_, err = d.cmdRunner.Run(ctx, "adb", pullArgs...)
	if err != nil {
		return "", fmt.Errorf(
			"pull screenshot display %d: %w",
			displayID, err,
		)
	}

	return localPath, nil
}

// CheckVideoRouting checks if video is actively being routed
// to the secondary display. It inspects media sessions and the
// Presenter/VideoOutputManager services.
func (d *DualDisplayDetector) CheckVideoRouting(
	ctx context.Context,
) (*VideoRoutingResult, error) {
	result := &VideoRoutingResult{}

	// 1. Check media session for PLAYING state.
	mediaInfo, err := d.CheckMediaSession(ctx)
	if err == nil && mediaInfo != nil &&
		mediaInfo.State == PlaybackStatePlaying {
		result.VideoPlaying = true
	}

	// 2. Check Presenter services for video output state.
	servicesArgs := d.adbArgs(
		"shell", "dumpsys", "activity", "services",
		presenterPackage,
	)
	servicesOutput, err := d.cmdRunner.Run(
		ctx, "adb", servicesArgs...,
	)
	if err == nil {
		svcOut := string(servicesOutput)
		result.ActiveDecoder = parseActiveDecoder(svcOut)
		result.SurfaceValid = strings.Contains(
			svcOut, "surfaceValid=true",
		) || strings.Contains(
			svcOut, "mSurface",
		)

		// Extract secondary display ID if present.
		displayIDRe := regexp.MustCompile(
			`secondaryDisplayId=(\d+)`,
		)
		if match := displayIDRe.FindStringSubmatch(
			svcOut,
		); match != nil {
			if id, parseErr := strconv.Atoi(
				match[1],
			); parseErr == nil {
				result.SecondaryDisplayID = id
			}
		}
	}

	// 3. Check logcat for C2BqBuffer stall (indicates frozen
	// frame, not healthy routing).
	frozen, _, frozenErr := d.CheckFrozenFrame(ctx)
	if frozenErr == nil && frozen {
		// Video is technically routed but stalled.
		result.VideoPlaying = false
	}

	return result, nil
}

// parseActiveDecoder extracts the active decoder name from
// Presenter service dumpsys output.
func parseActiveDecoder(output string) string {
	re := regexp.MustCompile(
		`activeDecoder=([^\s,}]+)`,
	)
	if match := re.FindStringSubmatch(output); match != nil {
		return match[1]
	}

	// Alternative pattern from VideoOutputManagerService.
	re2 := regexp.MustCompile(
		`mActiveDecoder.*?name=([^\s,}]+)`,
	)
	if match := re2.FindStringSubmatch(output); match != nil {
		return match[1]
	}

	return ""
}

// CheckFrozenFrame detects frozen frames by scanning logcat
// for C2BqBuffer stall entries. A stall indicates the codec
// cannot dequeue a buffer from the BufferQueue, causing the
// last rendered frame to remain on screen indefinitely.
func (d *DualDisplayDetector) CheckFrozenFrame(
	ctx context.Context,
) (bool, time.Duration, error) {
	args := d.adbArgs(
		"logcat", "-d", "-s", "C2BqBuffer:W",
	)
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		return false, 0, fmt.Errorf(
			"logcat C2BqBuffer: %w", err,
		)
	}

	lines := strings.Split(string(output), "\n")
	var maxStallUS int64

	stallRe := regexp.MustCompile(
		`last successful dequeue was (\d+) us ago`,
	)

	for _, line := range lines {
		match := stallRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		us, parseErr := strconv.ParseInt(match[1], 10, 64)
		if parseErr != nil {
			continue
		}
		if us > maxStallUS {
			maxStallUS = us
		}
	}

	if maxStallUS == 0 {
		return false, 0, nil
	}

	stallDuration := time.Duration(maxStallUS) *
		time.Microsecond

	// Consider it a frozen frame if stall exceeds 2 seconds.
	frozen := stallDuration > 2*time.Second

	return frozen, stallDuration, nil
}

// CheckPresenter checks the Presenter service status,
// including whether it is alive and which mode it is in
// (video mode vs album cover mode).
func (d *DualDisplayDetector) CheckPresenter(
	ctx context.Context,
) (*PresenterStatus, error) {
	status := &PresenterStatus{}

	// Check if Presenter process is alive.
	pidArgs := d.adbArgs(
		"shell", "pidof", presenterPackage,
	)
	pidOutput, err := d.cmdRunner.Run(
		ctx, "adb", pidArgs...,
	)
	if err == nil &&
		strings.TrimSpace(string(pidOutput)) != "" {
		status.ServiceAlive = true
	}

	if !status.ServiceAlive {
		return status, nil
	}

	// Check service state via dumpsys.
	svcArgs := d.adbArgs(
		"shell", "dumpsys", "activity", "services",
		presenterPackage,
	)
	svcOutput, err := d.cmdRunner.Run(
		ctx, "adb", svcArgs...,
	)
	if err != nil {
		return status, nil
	}

	svcOut := string(svcOutput)

	status.VideoMode = strings.Contains(
		svcOut, "videoMode=true",
	) || strings.Contains(
		svcOut, "isVideoMode=true",
	)

	status.AlbumCoverMode = strings.Contains(
		svcOut, "albumCoverMode=true",
	) || strings.Contains(
		svcOut, "isAlbumCoverMode=true",
	)

	// Parse secondary display ID.
	displayRe := regexp.MustCompile(
		`secondaryDisplayId=(\d+)`,
	)
	if match := displayRe.FindStringSubmatch(
		svcOut,
	); match != nil {
		if id, parseErr := strconv.Atoi(
			match[1],
		); parseErr == nil {
			status.SecondaryDisplayID = id
		}
	}

	return status, nil
}

// CheckMediaSession parses dumpsys media_session to find the
// active media session and its playback state, metadata, and
// owning package.
func (d *DualDisplayDetector) CheckMediaSession(
	ctx context.Context,
) (*MediaSessionInfo, error) {
	args := d.adbArgs(
		"shell", "dumpsys", "media_session",
	)
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		return nil, fmt.Errorf(
			"dumpsys media_session: %w", err,
		)
	}

	return parseMediaSession(string(output)), nil
}

// parseMediaSession extracts MediaSessionInfo from dumpsys
// media_session output. Playback state codes:
//
//	0=NONE, 1=STOPPED, 2=PAUSED, 3=PLAYING
func parseMediaSession(output string) *MediaSessionInfo {
	info := &MediaSessionInfo{}

	// Find active session package.
	pkgRe := regexp.MustCompile(
		`package=([a-zA-Z0-9_.]+)`,
	)
	if match := pkgRe.FindStringSubmatch(
		output,
	); match != nil {
		info.PackageName = match[1]
	}

	// Parse playback state.
	stateRe := regexp.MustCompile(
		`state=PlaybackState\s*\{[^}]*state=(\d+)`,
	)
	if match := stateRe.FindStringSubmatch(
		output,
	); match != nil {
		switch match[1] {
		case "1":
			info.State = PlaybackStateStopped
		case "2":
			info.State = PlaybackStatePaused
		case "3":
			info.State = PlaybackStatePlaying
		default:
			info.State = PlaybackStateStopped
		}
	}

	// Parse metadata title.
	titleRe := regexp.MustCompile(
		`description=.*?title=([^,}]+)`,
	)
	if match := titleRe.FindStringSubmatch(
		output,
	); match != nil {
		info.Title = strings.TrimSpace(match[1])
	}

	// Parse metadata artist.
	artistRe := regexp.MustCompile(
		`description=.*?subtitle=([^,}]+)`,
	)
	if match := artistRe.FindStringSubmatch(
		output,
	); match != nil {
		info.Artist = strings.TrimSpace(match[1])
	}

	// Check for album art (iconBitmap or artUri).
	info.HasAlbumArt = strings.Contains(
		output, "iconBitmap=android.graphics.Bitmap",
	) || strings.Contains(
		output, "artUri=",
	)

	return info
}

// CheckAll runs all dual-display checks and returns the
// combined result. This is the primary entry point for
// comprehensive dual-display health assessment.
func (d *DualDisplayDetector) CheckAll(
	ctx context.Context,
) (*DualDisplayResult, error) {
	result := &DualDisplayResult{
		DetectionResult: DetectionResult{
			Platform:  "android",
			Timestamp: time.Now(),
		},
	}

	// 1. Detect displays and discover secondary ID.
	displays, err := d.DetectDisplays(ctx)
	if err != nil {
		result.Error = fmt.Sprintf(
			"display detection failed: %v", err,
		)
		return result, nil
	}

	for _, disp := range displays {
		if disp.Type == DisplayTypeExternal &&
			disp.Connected {
			result.SecondaryDisplayConnected = true
			result.SecondaryDisplayResolution =
				disp.Resolution

			// Auto-discover secondary display ID if not
			// explicitly set.
			if d.secondaryDisplayID < 0 {
				d.secondaryDisplayID = disp.ID
			}
			break
		}
	}

	// 2. Capture screenshots.
	primaryPath, screenshotErr :=
		d.screenshotDisplayToFile(
			ctx, d.primaryDisplayID, "primary",
		)
	if screenshotErr == nil {
		result.PrimaryScreenshot = primaryPath
	}

	if d.secondaryDisplayID >= 0 {
		secondaryPath, secErr :=
			d.screenshotDisplayToFile(
				ctx, d.secondaryDisplayID, "secondary",
			)
		if secErr == nil {
			result.SecondaryScreenshot = secondaryPath
		}
	}

	// 3. Check Presenter service.
	presenter, presErr := d.CheckPresenter(ctx)
	if presErr == nil && presenter != nil {
		result.PresenterServiceAlive =
			presenter.ServiceAlive
		result.AlbumCoverVisible = presenter.AlbumCoverMode

		if presenter.VideoMode {
			result.VideoOnSecondary = true
		}
	}

	// 4. Check media session.
	media, mediaErr := d.CheckMediaSession(ctx)
	if mediaErr == nil && media != nil {
		result.MediaSessionState = media.State
	}

	// 5. Check video routing.
	routing, routeErr := d.CheckVideoRouting(ctx)
	if routeErr == nil && routing != nil {
		result.ActiveCodec = routing.ActiveDecoder

		if routing.VideoPlaying {
			result.VideoOnSecondary = true
		}
	}

	// 6. Check frozen frame.
	frozen, duration, frozenErr := d.CheckFrozenFrame(ctx)
	if frozenErr == nil {
		result.FrozenFrame = frozen
		result.FrozenFrameDuration = duration
	}

	return result, nil
}

// adbArgs prepends the -s device flag if a device is
// configured.
func (d *DualDisplayDetector) adbArgs(
	args ...string,
) []string {
	if d.device != "" {
		return append(
			[]string{"-s", d.device}, args...,
		)
	}
	return args
}
