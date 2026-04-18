// Package vision provides OCR capabilities using PaddleOCR
package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os/exec"
	"time"
)

// PaddleOCRConfig configures the PaddleOCR engine
type PaddleOCRConfig struct {
	// Service endpoint (local or remote)
	Endpoint string

	// Language (ch, en, ch_sim, etc.)
	Language string

	// Detection model (DB)
	UseAngleCls bool

	// Recognition model (CRNN)
	RecModel string

	// Timeout for OCR operations
	Timeout time.Duration

	// Minimum confidence threshold (0-1)
	MinConfidence float64

	// Use GPU acceleration
	UseGPU bool
}

// DefaultPaddleOCRConfig returns default PaddleOCR configuration
func DefaultPaddleOCRConfig() *PaddleOCRConfig {
	return &PaddleOCRConfig{
		Endpoint:      "http://localhost:8866/predict/ocr_system",
		Language:      "en",
		UseAngleCls:   true,
		RecModel:      "ch_PP-OCRv4_rec",
		Timeout:       30 * time.Second,
		MinConfidence: 0.7,
		UseGPU:        false,
	}
}

// PaddleOCR implements OCR using PaddleOCR
type PaddleOCR struct {
	config *PaddleOCRConfig
	client *http.Client
}

// PaddleOCRResponse represents the API response
type PaddleOCRResponse struct {
	Status string          `json:"status"`
	Msg    string          `json:"msg"`
	Result [][]interface{} `json:"result"`
}

// PaddleTextBox represents a detected text box
type PaddleTextBox struct {
	Box        [4][2]float64 `json:"box"`
	Text       string        `json:"text"`
	Confidence float64       `json:"confidence"`
}

// NewPaddleOCR creates a new PaddleOCR engine
func NewPaddleOCR(config *PaddleOCRConfig) (*PaddleOCR, error) {
	if config == nil {
		config = DefaultPaddleOCRConfig()
	}

	return &PaddleOCR{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// DetectText detects text in an image using PaddleOCR service
func (p *PaddleOCR) DetectText(img image.Image) ([]TextBlock, error) {
	// Convert image to base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return p.detectWithAPI(buf.Bytes())
}

// DetectTextWithContext detects text with context cancellation
func (p *PaddleOCR) DetectTextWithContext(ctx context.Context, img image.Image) ([]TextBlock, error) {
	done := make(chan struct{})
	var blocks []TextBlock
	var err error

	go func() {
		blocks, err = p.DetectText(img)
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		return blocks, err
	}
}

// detectWithAPI sends image to PaddleOCR service
func (p *PaddleOCR) detectWithAPI(imageData []byte) ([]TextBlock, error) {
	// Create request
	reqBody := map[string]interface{}{
		"images": []string{encodeBase64(imageData)},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", p.config.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paddleocr service returned status %d", resp.StatusCode)
	}

	// Parse response
	var result PaddleOCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "0" {
		return nil, fmt.Errorf("paddleocr error: %s", result.Msg)
	}

	return p.parseResult(result)
}

// parseResult converts PaddleOCR response to TextBlock format
func (p *PaddleOCR) parseResult(result PaddleOCRResponse) ([]TextBlock, error) {
	var blocks []TextBlock

	for _, item := range result.Result {
		if len(item) < 2 {
			continue
		}

		// Parse bounding box
		boxData, ok := item[0].([]interface{})
		if !ok {
			continue
		}

		// Get text
		text, ok := item[1].(string)
		if !ok {
			continue
		}

		// Get confidence
		confidence := 1.0
		if len(item) > 2 {
			if conf, ok := item[2].(float64); ok {
				confidence = conf
			}
		}

		// Skip low confidence
		if confidence < p.config.MinConfidence {
			continue
		}

		// Calculate bounding rectangle
		bounds := p.calculateBounds(boxData)

		block := TextBlock{
			Text:       text,
			Bounds:     bounds,
			Confidence: confidence,
			Language:   p.config.Language,
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

// calculateBounds calculates image.Rectangle from box coordinates
func (p *PaddleOCR) calculateBounds(boxData []interface{}) image.Rectangle {
	if len(boxData) < 4 {
		return image.Rect(0, 0, 0, 0)
	}

	var minX, minY, maxX, maxY float64

	for i, point := range boxData {
		coords, ok := point.([]interface{})
		if !ok || len(coords) < 2 {
			continue
		}

		x, _ := coords[0].(float64)
		y, _ := coords[1].(float64)

		if i == 0 {
			minX, minY, maxX, maxY = x, y, x, y
		} else {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}

	return image.Rect(int(minX), int(minY), int(maxX), int(maxY))
}

// encodeBase64 encodes bytes to base64 string
func encodeBase64(data []byte) string {
	// Simple base64 encoding
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	result := make([]byte, 0, len(data)*4/3+4)

	for i := 0; i < len(data); i += 3 {
		b := []int{int(data[i]), 0, 0}
		if i+1 < len(data) {
			b[1] = int(data[i+1])
		}
		if i+2 < len(data) {
			b[2] = int(data[i+2])
		}

		// Encode 3 bytes to 4 characters
		result = append(result, base64Chars[(b[0]>>2)&0x3F])
		result = append(result, base64Chars[((b[0]&0x03)<<4)|((b[1]>>4)&0x0F)])

		if i+1 < len(data) {
			result = append(result, base64Chars[((b[1]&0x0F)<<2)|((b[2]>>6)&0x03)])
		} else {
			result = append(result, '=')
		}

		if i+2 < len(data) {
			result = append(result, base64Chars[b[2]&0x3F])
		} else {
			result = append(result, '=')
		}
	}

	return string(result)
}

// CheckPaddleOCRAvailable checks if PaddleOCR service is available
func CheckPaddleOCRAvailable(endpoint string) bool {
	if endpoint == "" {
		endpoint = DefaultPaddleOCRConfig().Endpoint
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// StartPaddleOCRService starts the PaddleOCR service
func StartPaddleOCRService(gpu bool) (*exec.Cmd, error) {
	args := []string{
		"-m", "paddleocr",
		"--use_gpu", fmt.Sprintf("%v", gpu),
		"--enable_mkldnn", "true",
		"--use_tensorrt", "false",
		"--use_angle_cls", "true",
		"--lang", "en",
		"--use_space_char", "true",
	}

	cmd := exec.Command("python3", args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start paddleocr: %w", err)
	}

	// Wait for service to be ready
	time.Sleep(3 * time.Second)

	return cmd, nil
}

// PaddleOCRService manages a local PaddleOCR service
type PaddleOCRService struct {
	cmd    *exec.Cmd
	config *PaddleOCRConfig
}

// NewPaddleOCRService creates and starts a local PaddleOCR service
func NewPaddleOCRService(config *PaddleOCRConfig) (*PaddleOCRService, error) {
	if config == nil {
		config = DefaultPaddleOCRConfig()
	}

	cmd, err := StartPaddleOCRService(config.UseGPU)
	if err != nil {
		return nil, err
	}

	return &PaddleOCRService{
		cmd:    cmd,
		config: config,
	}, nil
}

// Stop stops the PaddleOCR service
func (s *PaddleOCRService) Stop() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// IsRunning checks if the service is running
func (s *PaddleOCRService) IsRunning() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}

	// Check if process is still alive
	return s.cmd.Process.Signal(nil) == nil
}

// PaddleStats holds PaddleOCR statistics
type PaddleStats struct {
	ImagesProcessed uint64
	TextsFound      uint64
	TotalTimeMs     uint64
	Errors          uint64
}

// GetPaddleVersion returns PaddleOCR version info
func GetPaddleVersion() (string, error) {
	cmd := exec.Command("python3", "-m", "paddleocr", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}
	return string(output), nil
}

// SupportedPaddleLanguages lists supported languages
var SupportedPaddleLanguages = map[string]string{
	"en":         "English",
	"ch":         "Chinese (Traditional)",
	"ch_sim":     "Chinese (Simplified)",
	"korean":     "Korean",
	"japan":      "Japanese",
	"latin":      "Latin",
	"arabic":     "Arabic",
	"cyrillic":   "Cyrillic",
	"devanagari": "Devanagari",
}

// CompareOCREngines compares Tesseract and PaddleOCR results
func CompareOCREngines(img image.Image) (map[string][]TextBlock, error) {
	results := make(map[string][]TextBlock)

	// Try Tesseract
	if CheckTesseractAvailable() {
		tessConfig := DefaultTesseractConfig()
		tess, err := NewTesseractOCR(tessConfig)
		if err == nil {
			if blocks, err := tess.DetectText(img); err == nil {
				results["tesseract"] = blocks
			}
		}
	}

	// Try PaddleOCR
	if CheckPaddleOCRAvailable("") {
		paddleConfig := DefaultPaddleOCRConfig()
		paddle, err := NewPaddleOCR(paddleConfig)
		if err == nil {
			if blocks, err := paddle.DetectText(img); err == nil {
				results["paddle"] = blocks
			}
		}
	}

	return results, nil
}
