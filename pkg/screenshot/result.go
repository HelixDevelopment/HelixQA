package screenshot

import (
	"encoding/json"
	"fmt"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// Result carries the image and all metadata required for retrieval, presentation, and timeline correlation.
type Result struct {
	Data        []byte
	Format      string
	Width       int
	Height      int
	Size        int
	Platform    config.Platform
	Timestamp   time.Time
	Duration    time.Duration
	SessionID   string
	StepName    string
	StepIndex   int
	Path        string
	VideoOffset time.Duration
	Thumbnail   []byte
	Breakpoint  string
	Engine      string
}

// Validate checks that the result has valid screenshot data.
func (r *Result) Validate() error {
	if r == nil {
		return fmt.Errorf("result is nil")
	}
	if len(r.Data) == 0 {
		return fmt.Errorf("screenshot data is empty")
	}
	if r.Format != "png" && r.Format != "jpg" && r.Format != "txt" {
		return fmt.Errorf("unsupported screenshot format: %s", r.Format)
	}
	if r.Format == "png" && len(r.Data) < 8 {
		return fmt.Errorf("PNG data too small (%d bytes)", len(r.Data))
	}
	return nil
}

// MetadataJSON returns the metadata as a JSON string (data excluded).
func (r *Result) MetadataJSON() (string, error) {
	if r == nil {
		return "", fmt.Errorf("result is nil")
	}
	m := map[string]interface{}{
		"format":    r.Format,
		"width":     r.Width,
		"height":    r.Height,
		"size":      len(r.Data),
		"platform":  string(r.Platform),
		"timestamp": r.Timestamp.Format(time.RFC3339),
		"duration":  r.Duration.String(),
		"sessionID": r.SessionID,
		"stepName":  r.StepName,
		"stepIndex": r.StepIndex,
		"engine":    r.Engine,
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
