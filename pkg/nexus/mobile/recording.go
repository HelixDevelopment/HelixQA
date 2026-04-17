package mobile

import (
	"context"
	"encoding/base64"
	"fmt"
)

// StartRecording begins a screen recording on the device. Android uses
// `mobile: startScreenRecording`, iOS uses `mobile: startPerfRecord`.
// The method returns nil on success; the actual bytes are retrieved by
// StopRecording.
func (c *AppiumClient) StartRecording(ctx context.Context, options RecordingOptions) error {
	args := map[string]any{}
	if options.BitRate > 0 {
		args["bitRate"] = options.BitRate
	}
	if options.TimeLimitSec > 0 {
		args["timeLimit"] = options.TimeLimitSec
	}
	if options.VideoSize != "" {
		args["videoSize"] = options.VideoSize
	}
	_, err := c.ExecuteScript(ctx, "mobile: startScreenRecording", args)
	if err != nil {
		return fmt.Errorf("start recording: %w", err)
	}
	return nil
}

// StopRecording stops the active recording and returns the raw bytes
// (already base64-decoded from Appium's payload).
func (c *AppiumClient) StopRecording(ctx context.Context) ([]byte, error) {
	raw, err := c.ExecuteScript(ctx, "mobile: stopScreenRecording", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("stop recording: %w", err)
	}
	encoded, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("stop recording: unexpected response type %T", raw)
	}
	if encoded == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode recording: %w", err)
	}
	return decoded, nil
}

// RecordingOptions tunes the start-recording call. Zero values apply
// platform defaults.
type RecordingOptions struct {
	BitRate      int    // bits per second; android default 4_000_000
	TimeLimitSec int    // max recording length; android 180, ios 600
	VideoSize    string // "1920x1080" form; android only
}
