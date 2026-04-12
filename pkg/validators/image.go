// Package validators provides asset validation for QA session results
package validators

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

// ImageValidator validates image files
type ImageValidator struct{}

// NewImageValidator creates a new image validator
func NewImageValidator() *ImageValidator {
	return &ImageValidator{}
}

// Validate validates an image file
func (v *ImageValidator) Validate(path string) (*ValidationResult, error) {
	result := &ValidationResult{
		AssetPath: path,
		AssetType: AssetTypeImage,
		IsValid:   true,
		Metadata:  make(map[string]interface{}),
	}

	// Get file info
	stat, err := os.Stat(path)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to stat file: %v", err))
		return result, nil
	}

	if stat.Size() == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "image file is empty (0 bytes)")
		return result, nil
	}

	result.Metadata["size_bytes"] = stat.Size()
	result.Metadata["size_human"] = formatBytes(stat.Size())

	// Open and decode image
	file, err := os.Open(path)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to open image: %v", err))
		return result, nil
	}
	defer file.Close()

	// Try to decode image
	img, format, err := image.Decode(file)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to decode image: %v", err))
		return result, nil
	}

	// Basic metadata
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	result.Metadata["format"] = format
	result.Metadata["width"] = width
	result.Metadata["height"] = height
	result.Metadata["aspect_ratio"] = float64(width) / float64(height)

	// Validate dimensions
	if width == 0 || height == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "image has invalid dimensions (0x0)")
	}

	// Check for suspicious dimensions
	if width > 16384 || height > 16384 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("image dimensions are very large (%dx%d)", width, height))
	}

	// Format-specific validation
	switch format {
	case "jpeg":
		v.validateJPEG(file, result)
	case "png":
		v.validatePNG(file, result)
	case "gif":
		v.validateGIF(file, result)
	}

	// Check for common screenshot sizes
	result.Metadata["is_screenshot_size"] = isScreenshotSize(width, height)
	result.Metadata["is_hd"] = width >= 1280 && height >= 720
	result.Metadata["is_full_hd"] = width >= 1920 && height >= 1080
	result.Metadata["is_4k"] = width >= 3840 && height >= 2160

	return result, nil
}

// Supports checks if this validator supports the given file
func (v *ImageValidator) Supports(path string) bool {
	type_ := DetectAssetType(path)
	return type_ == AssetTypeImage
}

func (v *ImageValidator) validateJPEG(file *os.File, result *ValidationResult) {
	// Reset file position
	file.Seek(0, 0)

	config, err := jpeg.DecodeConfig(file)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("JPEG config decode error: %v", err))
		return
	}

	result.Metadata["color_model"] = fmt.Sprintf("%v", config.ColorModel)
}

func (v *ImageValidator) validatePNG(file *os.File, result *ValidationResult) {
	// Reset file position
	file.Seek(0, 0)

	config, err := png.DecodeConfig(file)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("PNG config decode error: %v", err))
		return
	}

	result.Metadata["color_model"] = fmt.Sprintf("%v", config.ColorModel)
	result.Metadata["has_alpha"] = true // PNG always supports alpha
}

func (v *ImageValidator) validateGIF(file *os.File, result *ValidationResult) {
	// Reset file position
	file.Seek(0, 0)

	config, err := gif.DecodeConfig(file)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("GIF config decode error: %v", err))
		return
	}

	result.Metadata["color_model"] = fmt.Sprintf("%v", config.ColorModel)
	// gif.DecodeConfig returns image.Config which has no Delay field;
	// animated GIF detection requires gif.DecodeAll which is expensive.
	result.Metadata["is_animated"] = false
}

func isScreenshotSize(width, height int) bool {
	// Common Android TV screenshot resolutions
	commonSizes := []struct{ w, h int }{
		{1920, 1080}, // Full HD
		{1280, 720},  // HD
		{3840, 2160}, // 4K
		{2560, 1440}, // QHD
	}

	for _, size := range commonSizes {
		if (width == size.w && height == size.h) || (width == size.h && height == size.w) {
			return true
		}
	}
	return false
}

// ValidateImageDirectory validates all image files in a directory
func ValidateImageDirectory(dirPath string) ([]*ValidationResult, error) {
	validator := NewImageValidator()
	
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var results []*ValidationResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		path := filepath.Join(dirPath, entry.Name())
		if validator.Supports(path) {
			result, err := validator.Validate(path)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// ScreenshotValidator specifically validates screenshot images
type ScreenshotValidator struct {
	ImageValidator
	expectedWidth  int
	expectedHeight int
}

// NewScreenshotValidator creates a screenshot validator with expected dimensions
func NewScreenshotValidator(expectedWidth, expectedHeight int) *ScreenshotValidator {
	return &ScreenshotValidator{
		expectedWidth:  expectedWidth,
		expectedHeight: expectedHeight,
	}
}

// Validate validates a screenshot against expected dimensions
func (v *ScreenshotValidator) Validate(path string) (*ValidationResult, error) {
	result, err := v.ImageValidator.Validate(path)
	if err != nil {
		return nil, err
	}

	// Additional screenshot validation
	if v.expectedWidth > 0 && v.expectedHeight > 0 {
		width, _ := result.Metadata["width"].(int)
		height, _ := result.Metadata["height"].(int)

		if width != v.expectedWidth || height != v.expectedHeight {
			result.Warnings = append(result.Warnings, 
				fmt.Sprintf("screenshot dimensions %dx%d don't match expected %dx%d", 
					width, height, v.expectedWidth, v.expectedHeight))
		}
	}

	// Screenshots should be reasonable size
	sizeBytes, _ := result.Metadata["size_bytes"].(int64)
	if sizeBytes < 10000 { // Less than 10KB
		result.Warnings = append(result.Warnings, "screenshot is unusually small (< 10KB)")
	}
	if sizeBytes > 5000000 { // More than 5MB
		result.Warnings = append(result.Warnings, "screenshot is unusually large (> 5MB)")
	}

	return result, nil
}
