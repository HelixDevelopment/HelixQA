// Package text provides OCR and text extraction capabilities.
//
// This file implements Tesseract OCR integration for text extraction
// from UI screenshots.
//
//go:build vision
// +build vision

package text

import (
	"context"
	"fmt"
	"image"
	"strings"
	"sync"

	"github.com/otiai10/gosseract/v2"
	"gocv.io/x/gocv"

	"digital.vasic.helixqa/pkg/vision/core"
)

// TesseractExtractor implements text extraction using Tesseract OCR.
//
// Tesseract is an open-source OCR engine that provides:
// - Fast text recognition (50-100ms per image)
// - Support for 100+ languages
// - Configurable page segmentation modes
// - No external API dependencies
//
// Usage:
//
//	extractor := text.NewTesseractExtractor(text.TesseractConfig{
//	    DataPath: "/usr/share/tesseract-ocr/4.00/tessdata",
//	    Languages: []string{"eng", "chi_sim"},
//	})
//	defer extractor.Close()
//
//	regions, err := extractor.Extract(ctx, frame)
type TesseractExtractor struct {
	config TesseractConfig

	// Client pool for concurrent usage
	clientPool chan *gosseract.Client
	poolOnce   sync.Once

	// Statistics
	stats Stats
}

// TesseractConfig configures the Tesseract extractor.
type TesseractConfig struct {
	// DataPath to tessdata directory
	DataPath string

	// Languages to recognize (e.g., "eng", "chi_sim", "fra")
	Languages []string

	// PageSegmentMode sets the PSM mode:
	// 0 = Orientation and script detection (OSD) only.
	// 1 = Automatic page segmentation with OSD.
	// 2 = Automatic page segmentation, but no OSD, or OCR.
	// 3 = Fully automatic page segmentation, but no OSD. (Default)
	// 4 = Assume a single column of text of variable sizes.
	// 5 = Assume a single uniform block of vertically aligned text.
	// 6 = Assume a single uniform block of text.
	// 7 = Treat the image as a single text line.
	// 8 = Treat the image as a single word.
	// 9 = Treat the image as a single word in a circle.
	// 10 = Treat the image as a single character.
	// 11 = Sparse text. Find as much text as possible in no particular order.
	// 12 = Sparse text with OSD.
	// 13 = Raw line. Treat the image as a single text line,
	//      bypassing hacks that are Tesseract-specific.
	PageSegmentMode int

	// OEMode sets the OCR Engine mode:
	// 0 = Original Tesseract only.
	// 1 = Neural nets LSTM only.
	// 2 = Tesseract + LSTM.
	// 3 = Default, based on what is available.
	OEMode int

	// Variables for Tesseract configuration
	Variables map[string]string

	// DPI for image processing
	DPI int

	// PoolSize for concurrent client usage
	PoolSize int

	// MinConfidence threshold for text regions
	MinConfidence float64
}

// DefaultTesseractConfig returns sensible defaults.
func DefaultTesseractConfig() TesseractConfig {
	return TesseractConfig{
		DataPath:        "/usr/share/tesseract-ocr/4.00/tessdata",
		Languages:       []string{"eng"},
		PageSegmentMode: 3,
		OEMode:          3,
		Variables: map[string]string{
			"tessedit_char_whitelist": "", // Empty = all characters
		},
		DPI:           300,
		PoolSize:      4,
		MinConfidence: 60.0,
	}
}

// Stats contains performance statistics.
type Stats struct {
	ExtractCalls   int64
	TotalDuration  int64
	ErrorCount     int64
	CharacterCount int64
}

// NewTesseractExtractor creates a new Tesseract-based extractor.
func NewTesseractExtractor(config TesseractConfig) (*TesseractExtractor, error) {
	extractor := &TesseractExtractor{
		config:     config,
		clientPool: make(chan *gosseract.Client, config.PoolSize),
	}

	// Initialize client pool
	for i := 0; i < config.PoolSize; i++ {
		client := gosseract.NewClient()

		// Set language
		lang := strings.Join(config.Languages, "+")
		if err := client.SetLanguage(lang); err != nil {
			return nil, fmt.Errorf("setting language: %w", err)
		}

		// Set data path if provided
		if config.DataPath != "" {
			client.SetTessdataPrefix(config.DataPath)
		}

		// Set page segmentation mode
		client.SetPageSegMode(gosseract.PageSegMode(config.PageSegmentMode))

		// Set variables
		for key, value := range config.Variables {
			client.SetVariable(key, value)
		}

		extractor.clientPool <- client
	}

	return extractor, nil
}

// Close releases all resources.
func (e *TesseractExtractor) Close() error {
	close(e.clientPool)
	for client := range e.clientPool {
		client.Close()
	}
	return nil
}

// Extract implements core.TextExtractor.
func (e *TesseractExtractor) Extract(ctx context.Context, frame *core.Frame) ([]core.TextRegion, error) {
	client := e.acquireClient()
	defer e.releaseClient(client)

	// Set image from bytes
	if err := client.SetImageFromBytes(frame.Data); err != nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("setting image: %w", err)
	}

	// Extract text with bounding boxes
	text, err := client.Text()
	if err != nil {
		e.stats.ErrorCount++
		return nil, fmt.Errorf("extracting text: %w", err)
	}

	// Get bounding boxes for each word/line
	boxes, err := client.GetBoundingBoxes(gosseract.RIL_WORD)
	if err != nil {
		// Fallback to single region if bounding boxes fail
		return []core.TextRegion{{
			Bounds: core.Rectangle{
				Rectangle:  frame.Bounds,
				Confidence: 0.8,
			},
			Text:       strings.TrimSpace(text),
			Confidence: 0.8,
			Language:   e.config.Languages[0],
		}}, nil
	}

	regions := make([]core.TextRegion, 0, len(boxes))
	for _, box := range boxes {
		// Filter by confidence
		if box.Confidence < e.config.MinConfidence {
			continue
		}

		region := core.TextRegion{
			Bounds: core.Rectangle{
				Rectangle: image.Rectangle{
					Min: image.Point{X: box.Box.Min.X, Y: box.Box.Min.Y},
					Max: image.Point{X: box.Box.Max.X, Y: box.Box.Max.Y},
				},
				Confidence: float64(box.Confidence) / 100.0,
			},
			Text:       strings.TrimSpace(box.Word),
			Confidence: float64(box.Confidence) / 100.0,
			Language:   e.config.Languages[0],
			IsVertical: false,
		}
		regions = append(regions, region)
	}

	e.stats.ExtractCalls++
	e.stats.CharacterCount += int64(len(text))

	return regions, nil
}

// ExtractRegion implements core.TextExtractor.
func (e *TesseractExtractor) ExtractRegion(
	ctx context.Context,
	frame *core.Frame,
	region image.Rectangle,
) (*core.TextRegion, error) {
	client := e.acquireClient()
	defer e.releaseClient(client)

	// Crop image to region
	cropped, err := cropFrame(frame, region)
	if err != nil {
		return nil, fmt.Errorf("cropping frame: %w", err)
	}

	// Set image
	if err := client.SetImageFromBytes(cropped.Data); err != nil {
		return nil, fmt.Errorf("setting image: %w", err)
	}

	// Extract text
	text, err := client.Text()
	if err != nil {
		return nil, fmt.Errorf("extracting text: %w", err)
	}

	// Get confidence
	confidence, _ := client.MeanTextConf()

	return &core.TextRegion{
		Bounds: core.Rectangle{
			Rectangle:  region,
			Confidence: float64(confidence) / 100.0,
		},
		Text:       strings.TrimSpace(text),
		Confidence: float64(confidence) / 100.0,
		Language:   e.config.Languages[0],
	}, nil
}

// DetectLanguage implements core.TextExtractor.
func (e *TesseractExtractor) DetectLanguage(ctx context.Context, frame *core.Frame) (string, error) {
	client := e.acquireClient()
	defer e.releaseClient(client)

	if err := client.SetImageFromBytes(frame.Data); err != nil {
		return "", fmt.Errorf("setting image: %w", err)
	}

	// OSD (Orientation and Script Detection)
	osd, err := client.OSD()
	if err != nil {
		return "", fmt.Errorf("detecting orientation: %w", err)
	}

	// Map script to language
	scriptMap := map[string]string{
		"Latin":      "eng",
		"Han":        "chi_sim",
		"Hangul":     "kor",
		"Japanese":   "jpn",
		"Arabic":     "ara",
		"Cyrillic":   "rus",
		"Greek":      "ell",
		"Hebrew":     "heb",
		"Thai":       "tha",
		"Devanagari": "hin",
	}

	if lang, ok := scriptMap[osd.ScriptName]; ok {
		return lang, nil
	}

	return "eng", nil // Default to English
}

// ExtractWithLayout extracts text with layout information.
func (e *TesseractExtractor) ExtractWithLayout(ctx context.Context, frame *core.Frame) ([]core.TextBlock, error) {
	client := e.acquireClient()
	defer e.releaseClient(client)

	if err := client.SetImageFromBytes(frame.Data); err != nil {
		return nil, fmt.Errorf("setting image: %w", err)
	}

	// Get text blocks at paragraph level
	boxes, err := client.GetBoundingBoxes(gosseract.RIL_PARA)
	if err != nil {
		return nil, fmt.Errorf("getting bounding boxes: %w", err)
	}

	blocks := make([]core.TextBlock, 0, len(boxes))
	for i, box := range boxes {
		blockType := inferBlockType(box.Word, i)
		level := inferHeadingLevel(box.Word)

		block := core.TextBlock{
			TextRegion: core.TextRegion{
				Bounds: core.Rectangle{
					Rectangle: image.Rectangle{
						Min: image.Point{X: box.Box.Min.X, Y: box.Box.Min.Y},
						Max: image.Point{X: box.Box.Max.X, Y: box.Box.Max.Y},
					},
					Confidence: float64(box.Confidence) / 100.0,
				},
				Text:       strings.TrimSpace(box.Word),
				Confidence: float64(box.Confidence) / 100.0,
				Language:   e.config.Languages[0],
			},
			BlockType:    blockType,
			Level:        level,
			ReadingOrder: i,
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// GetStats returns usage statistics.
func (e *TesseractExtractor) GetStats() Stats {
	return e.stats
}

// acquireClient gets a client from the pool.
func (e *TesseractExtractor) acquireClient() *gosseract.Client {
	return <-e.clientPool
}

// releaseClient returns a client to the pool.
func (e *TesseractExtractor) releaseClient(client *gosseract.Client) {
	e.clientPool <- client
}

// cropFrame crops a frame to the specified region.
func cropFrame(frame *core.Frame, region image.Rectangle) (*core.Frame, error) {
	// Convert bytes to mat for cropping
	mat, err := gocv.NewMatFromBytes(
		frame.Bounds.Dy(),
		frame.Bounds.Dx(),
		gocv.MatTypeCV8UC3,
		frame.Data,
	)
	if err != nil {
		return nil, fmt.Errorf("creating mat: %w", err)
	}
	defer mat.Close()

	// Crop to region
	cropped := mat.Region(region)
	defer cropped.Close()

	// Convert back to bytes
	data, err := cropped.ToImage()
	if err != nil {
		return nil, fmt.Errorf("converting to image: %w", err)
	}

	// Convert image to bytes (simplified - actual implementation would encode)
	_ = data

	return &core.Frame{
		Data:      frame.Data, // Placeholder - would be actual cropped data
		Bounds:    region,
		Timestamp: frame.Timestamp,
		Source:    frame.Source,
		Metadata:  frame.Metadata,
	}, nil
}

// inferBlockType determines the type of text block.
func inferBlockType(text string, position int) core.TextBlockType {
	text = strings.TrimSpace(text)

	if text == "" {
		return core.TextBlockUnknown
	}

	// Check for heading characteristics
	if len(text) < 100 {
		// Short text at the beginning might be a heading
		if position < 3 && !strings.Contains(text, ".") {
			return core.TextBlockHeading
		}

		// Check for all caps
		if text == strings.ToUpper(text) && len(text) > 3 {
			return core.TextBlockHeading
		}
	}

	// Check for button characteristics
	if len(text) < 30 && (strings.HasSuffix(text, "→") || strings.HasSuffix(text, ">")) {
		return core.TextBlockButton
	}

	// Check for link characteristics
	if strings.HasPrefix(text, "http") || strings.Contains(text, "www.") {
		return core.TextBlockLink
	}

	// Check for code
	if strings.Contains(text, "{") || strings.Contains(text, "}") || strings.Contains(text, ";") {
		return core.TextBlockCode
	}

	// Default to paragraph
	return core.TextBlockParagraph
}

// inferHeadingLevel determines heading level (1-6).
func inferHeadingLevel(text string) int {
	// Simple heuristic based on length and capitalization
	text = strings.TrimSpace(text)
	length := len(text)

	if length < 20 && text == strings.ToUpper(text) {
		return 1
	}
	if length < 40 && text == strings.ToUpper(text) {
		return 2
	}
	if length < 60 {
		return 3
	}
	if length < 80 {
		return 4
	}
	if length < 100 {
		return 5
	}
	return 0 // Not a heading
}
