// Package vision provides OCR capabilities using Tesseract
package vision

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TesseractConfig configures the Tesseract OCR engine
type TesseractConfig struct {
	// Path to tesseract executable
	TesseractPath string

	// Language code(s) for OCR (e.g., "eng", "eng+deu", "chi_sim")
	Language string

	// Page segmentation mode (PSM)
	// 0 = Orientation and script detection only
	// 1 = Automatic page segmentation with OSD
	// 3 = Fully automatic page segmentation (default)
	// 4 = Assume single column of text
	// 6 = Assume uniform block of text
	// 7 = Treat as single text line
	// 8 = Treat as single word
	// 11 = Sparse text - find as much text as possible
	PageSegMode int

	// OCR engine mode (OEM)
	// 0 = Legacy engine only
	// 1 = Neural nets LSTM engine only
	// 2 = Legacy + LSTM engines
	// 3 = Default (based on what's available)
	EngineMode int

	// Timeout for OCR operations
	Timeout time.Duration

	// Preserve whitespace in output
	PreserveWhitespace bool

	// Minimum confidence threshold (0-100)
	MinConfidence int
}

// DefaultTesseractConfig returns default Tesseract configuration
func DefaultTesseractConfig() *TesseractConfig {
	return &TesseractConfig{
		TesseractPath:      "tesseract",
		Language:           "eng",
		PageSegMode:        3,
		EngineMode:         3,
		Timeout:            30 * time.Second,
		PreserveWhitespace: false,
		MinConfidence:      60,
	}
}

// TesseractOCR implements OCR using Tesseract
type TesseractOCR struct {
	config *TesseractConfig
}

// NewTesseractOCR creates a new Tesseract OCR engine
func NewTesseractOCR(config *TesseractConfig) (*TesseractOCR, error) {
	if config == nil {
		config = DefaultTesseractConfig()
	}

	// Check if tesseract is available
	if err := checkTesseract(config.TesseractPath); err != nil {
		return nil, fmt.Errorf("tesseract not available: %w", err)
	}

	return &TesseractOCR{config: config}, nil
}

// DetectText detects text in an image
func (t *TesseractOCR) DetectText(img image.Image) ([]TextBlock, error) {
	// Convert image to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	// Run tesseract
	result, err := t.runTesseract(&buf, "tsv")
	if err != nil {
		return nil, err
	}

	// Parse TSV output
	return t.parseTSV(result), nil
}

// DetectTextWithContext detects text with context cancellation
func (t *TesseractOCR) DetectTextWithContext(ctx context.Context, img image.Image) ([]TextBlock, error) {
	done := make(chan struct{})
	var blocks []TextBlock
	var err error

	go func() {
		blocks, err = t.DetectText(img)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		return blocks, err
	}
}

// DetectTextString returns plain text from image
func (t *TesseractOCR) DetectTextString(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	result, err := t.runTesseract(&buf, "")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(result)), nil
}

// GetAvailableLanguages returns list of installed languages
func (t *TesseractOCR) GetAvailableLanguages() ([]string, error) {
	cmd := exec.Command(t.config.TesseractPath, "--list-langs")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list languages: %w", err)
	}

	// Parse output
	lines := strings.Split(string(output), "\n")
	var langs []string

	// Skip first line (version info)
	for i, line := range lines {
		if i == 0 {
			continue
		}
		line = strings.TrimSpace(line)
		if line != "" {
			langs = append(langs, line)
		}
	}

	return langs, nil
}

// GetVersion returns Tesseract version
func (t *TesseractOCR) GetVersion() (string, error) {
	cmd := exec.Command(t.config.TesseractPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	// Parse first line
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}

	return "", fmt.Errorf("no version info")
}

// runTesseract executes tesseract with given image and output format
func (t *TesseractOCR) runTesseract(imgData *bytes.Buffer, outputFormat string) ([]byte, error) {
	args := []string{
		"stdin",
		"stdout",
		"-l", t.config.Language,
		"--psm", strconv.Itoa(t.config.PageSegMode),
		"--oem", strconv.Itoa(t.config.EngineMode),
	}

	if outputFormat != "" {
		args = append(args, outputFormat)
	}

	if t.config.PreserveWhitespace {
		args = append(args, "preserve_interword_spaces", "1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.config.TesseractPath, args...)
	cmd.Stdin = imgData

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("tesseract timeout after %v", t.config.Timeout)
		}
		return nil, fmt.Errorf("tesseract failed: %w\nOutput: %s", err, string(output))
	}

	return output, nil
}

// parseTSV parses Tesseract TSV output
func (t *TesseractOCR) parseTSV(data []byte) []TextBlock {
	var blocks []TextBlock

	lines := strings.Split(string(data), "\n")

	// Skip header line
	for i, line := range lines {
		if i == 0 {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 12 {
			continue
		}

		// Parse confidence
		conf, err := strconv.ParseFloat(fields[10], 64)
		if err != nil {
			continue
		}

		// Skip low confidence
		if conf < float64(t.config.MinConfidence) {
			continue
		}

		// Parse bounding box
		x, _ := strconv.Atoi(fields[6])
		y, _ := strconv.Atoi(fields[7])
		w, _ := strconv.Atoi(fields[8])
		h, _ := strconv.Atoi(fields[9])

		text := strings.TrimSpace(fields[11])
		if text == "" {
			continue
		}

		block := TextBlock{
			Text:       text,
			Bounds:     image.Rect(x, y, x+w, y+h),
			Confidence: conf / 100.0,
			Language:   t.config.Language,
		}

		blocks = append(blocks, block)
	}

	return blocks
}

// checkTesseract verifies tesseract is available
func checkTesseract(path string) error {
	cmd := exec.Command(path, "--version")
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// CheckTesseractAvailable checks if tesseract is installed
func CheckTesseractAvailable() bool {
	return checkTesseract("tesseract") == nil
}

// TesseractVersionInfo holds version information
type TesseractVersionInfo struct {
	Version   string
	Languages []string
	Leptonica string
}

// GetDetailedVersion returns detailed version info
func GetDetailedVersion() (*TesseractVersionInfo, error) {
	cmd := exec.Command("tesseract", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	info := &TesseractVersionInfo{}
	lines := strings.Split(string(output), "\n")

	versionRegex := regexp.MustCompile(`tesseract (\d+\.\d+\.?\d*)`)
	leptonicaRegex := regexp.MustCompile(`leptonica-(\d+\.\d+\.?\d*)`)

	for _, line := range lines {
		if matches := versionRegex.FindStringSubmatch(line); matches != nil {
			info.Version = matches[1]
		}
		if matches := leptonicaRegex.FindStringSubmatch(line); matches != nil {
			info.Leptonica = matches[1]
		}
	}

	return info, nil
}

// InstallLanguage installs a language pack (requires sudo)
func InstallLanguage(language string) error {
	cmd := exec.Command("apt-get", "install", "-y", fmt.Sprintf("tesseract-ocr-%s", language))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install language %s: %w", language, err)
	}
	return nil
}

// SupportedLanguages lists commonly supported languages
var SupportedLanguages = map[string]string{
	"eng":     "English",
	"deu":     "German",
	"fra":     "French",
	"spa":     "Spanish",
	"ita":     "Italian",
	"por":     "Portuguese",
	"rus":     "Russian",
	"chi_sim": "Chinese (Simplified)",
	"chi_tra": "Chinese (Traditional)",
	"jpn":     "Japanese",
	"kor":     "Korean",
	"ara":     "Arabic",
	"hin":     "Hindi",
	"tha":     "Thai",
	"vie":     "Vietnamese",
	"pol":     "Polish",
	"tur":     "Turkish",
	"nld":     "Dutch",
	"ces":     "Czech",
	"swe":     "Swedish",
}

// TesseractStats holds OCR statistics
type TesseractStats struct {
	ImagesProcessed uint64
	TextsFound      uint64
	TotalTimeMs     uint64
	Errors          uint64
}

// TesseractProcessor wraps Tesseract with batch processing
type TesseractProcessor struct {
	ocr    *TesseractOCR
	stats  TesseractStats
	buffer chan *ocrTask
}

type ocrTask struct {
	img     image.Image
	result  chan<- []TextBlock
	errChan chan<- error
}

// NewTesseractProcessor creates a processor with worker pool
func NewTesseractProcessor(config *TesseractConfig, workers int) (*TesseractProcessor, error) {
	ocr, err := NewTesseractOCR(config)
	if err != nil {
		return nil, err
	}

	return &TesseractProcessor{
		ocr:    ocr,
		buffer: make(chan *ocrTask, workers*2),
	}, nil
}

// Process submits an image for OCR processing
func (tp *TesseractProcessor) Process(img image.Image) ([]TextBlock, error) {
	resultChan := make(chan []TextBlock, 1)
	errChan := make(chan error, 1)

	task := &ocrTask{
		img:     img,
		result:  resultChan,
		errChan: errChan,
	}

	tp.buffer <- task

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return nil, err
	}
}

// GetStats returns processor statistics
func (tp *TesseractProcessor) GetStats() TesseractStats {
	return tp.stats
}
