// Package analysis provides AI-driven screen state recognition for QA
package analysis

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/png"
	"os"
)

// ScreenState represents the recognized state of the app screen
type ScreenState string

const (
	StateSplashScreen     ScreenState = "splash_screen"
	StateLoading          ScreenState = "loading"
	StateHomeContent      ScreenState = "home_with_content"
	StateHomeEmpty        ScreenState = "home_empty"
	StateMovieGrid        ScreenState = "movie_grid"
	StateMovieDetails     ScreenState = "movie_details"
	StateTVShows          ScreenState = "tv_shows"
	StateSearch           ScreenState = "search"
	StateSettings         ScreenState = "settings"
	StateSystemSettings   ScreenState = "system_settings" // Wrong state!
	StateError            ScreenState = "error"
	StateUnknown          ScreenState = "unknown"
)

// StateDetector uses AI/ML to recognize screen states
type StateDetector struct {
	visionClient VisionClient
}

// VisionClient interface for vision model integration
type VisionClient interface {
	AnalyzeImage(ctx context.Context, imageBase64 string, prompt string) (*VisionResult, error)
}

// VisionResult contains the vision model analysis
type VisionResult struct {
	State           ScreenState            `json:"state"`
	Confidence      float64                `json:"confidence"`
	ContentItems    int                    `json:"content_items,omitempty"`
	HasError        bool                   `json:"has_error"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	UIElements      []UIElement            `json:"ui_elements,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// UIElement represents a detected UI element
type UIElement struct {
	Type        string  `json:"type"`
	Label       string  `json:"label,omitempty"`
	Confidence  float64 `json:"confidence"`
	BoundingBox struct {
		X, Y, Width, Height float64
	} `json:"bounding_box,omitempty"`
}

// NewStateDetector creates a new state detector
func NewStateDetector(visionClient VisionClient) *StateDetector {
	return &StateDetector{visionClient: visionClient}
}

// DetectState analyzes a screenshot and returns the recognized state
func (d *StateDetector) DetectState(ctx context.Context, screenshotPath string) (*VisionResult, error) {
	// Read and encode image
	imageBase64, err := encodeImageToBase64(screenshotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	prompt := `Analyze this Android TV app screenshot and identify:
1. Current screen state (splash, loading, home, movie grid, details, settings, error, etc.)
2. Number of content items visible (movies, shows, etc.)
3. Any error messages or loading indicators
4. Whether this is the Catalogizer app or system UI
5. Confidence level (0.0-1.0)

Respond in JSON format with fields: state, confidence, content_items, has_error, error_message`

	return d.visionClient.AnalyzeImage(ctx, imageBase64, prompt)
}

// WaitForState waits for a specific state with timeout
func (d *StateDetector) WaitForState(ctx context.Context, screenshotFunc func() (string, error), targetState ScreenState, timeoutSec int) (*VisionResult, error) {
	for i := 0; i < timeoutSec; i++ {
		screenshotPath, err := screenshotFunc()
		if err != nil {
			return nil, err
		}

		result, err := d.DetectState(ctx, screenshotPath)
		if err != nil {
			return nil, err
		}

		if result.State == targetState && result.Confidence > 0.7 {
			return result, nil
		}

		// Check for error states
		if result.State == StateError || result.State == StateSystemSettings {
			return result, fmt.Errorf("unexpected state detected: %s", result.State)
		}

		// Wait before next check
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-waitChan(1):
		}
	}

	return nil, fmt.Errorf("timeout waiting for state %s", targetState)
}

// VerifyAppInFocus checks if Catalogizer is still the active app
func (d *StateDetector) VerifyAppInFocus(ctx context.Context, screenshotPath string) (bool, *VisionResult, error) {
	result, err := d.DetectState(ctx, screenshotPath)
	if err != nil {
		return false, nil, err
	}

	// System settings or other apps indicate we lost focus
	if result.State == StateSystemSettings {
		return false, result, nil
	}

	return true, result, nil
}

// encodeImageToBase64 encodes an image file to base64
func encodeImageToBase64(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// waitChan creates a channel that closes after duration
func waitChan(seconds int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		// In real implementation, use time.Sleep
		close(ch)
	}()
	return ch
}

// ScreenshotValidator validates screenshot quality
type ScreenshotValidator struct {
	minFileSize int64
	minWidth    int
	minHeight   int
}

// NewScreenshotValidator creates a validator with defaults
func NewScreenshotValidator() *ScreenshotValidator {
	return &ScreenshotValidator{
		minFileSize: 500000, // 500KB
		minWidth:    1920,
		minHeight:   1080,
	}
}

// Validate checks if a screenshot is valid
func (v *ScreenshotValidator) Validate(path string) (*ValidationReport, error) {
	report := &ValidationReport{
		Path:     path,
		IsValid:  true,
		Issues:   []string{},
	}

	// Check file exists and size
	info, err := os.Stat(path)
	if err != nil {
		report.IsValid = false
		report.Issues = append(report.Issues, fmt.Sprintf("file error: %v", err))
		return report, err
	}

	report.FileSize = info.Size()

	if info.Size() < v.minFileSize {
		report.IsValid = false
		report.Issues = append(report.Issues, 
			fmt.Sprintf("file too small: %d bytes (min: %d)", info.Size(), v.minFileSize))
	}

	// Try to decode image
	file, err := os.Open(path)
	if err != nil {
		report.IsValid = false
		report.Issues = append(report.Issues, fmt.Sprintf("cannot open: %v", err))
		return report, err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		report.IsValid = false
		report.Issues = append(report.Issues, fmt.Sprintf("decode error: %v", err))
		return report, err
	}

	report.Format = format
	bounds := img.Bounds()
	report.Width = bounds.Dx()
	report.Height = bounds.Dy()

	if report.Width < v.minWidth || report.Height < v.minHeight {
		report.IsValid = false
		report.Issues = append(report.Issues,
			fmt.Sprintf("dimensions too small: %dx%d (min: %dx%d)",
				report.Width, report.Height, v.minWidth, v.minHeight))
	}

	return report, nil
}

// ValidationReport contains validation results
type ValidationReport struct {
	Path     string   `json:"path"`
	IsValid  bool     `json:"is_valid"`
	FileSize int64    `json:"file_size"`
	Format   string   `json:"format"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Issues   []string `json:"issues,omitempty"`
}
