// Package core provides configuration management for the vision system.
//
//go:build vision
// +build vision

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all vision system configuration.
type Config struct {
	// Core OpenCV settings
	OpenCV OpenCVConfig `json:"opencv" yaml:"opencv"`

	// OCR engine configurations
	OCR OCRConfig `json:"ocr" yaml:"ocr"`

	// Model configurations
	Models ModelsConfig `json:"models" yaml:"models"`

	// External framework configurations
	Frameworks FrameworksConfig `json:"frameworks" yaml:"frameworks"`

	// Visual regression configuration
	Regression RegressionConfig `json:"regression" yaml:"regression"`

	// Performance settings
	Performance PerformanceConfig `json:"performance" yaml:"performance"`

	// Logging settings
	Logging LoggingConfig `json:"logging" yaml:"logging"`
}

// OpenCVConfig configures core OpenCV settings.
type OpenCVConfig struct {
	// Enabled enables OpenCV processing
	Enabled bool `json:"enabled" yaml:"enabled"`

	// GPUEnabled enables GPU acceleration
	GPUEnabled bool `json:"gpu_enabled" yaml:"gpu_enabled"`

	// GPUBackend selects GPU backend (cuda, opencl)
	GPUBackend string `json:"gpu_backend" yaml:"gpu_backend"`

	// CacheSize sets the LRU cache size
	CacheSize int `json:"cache_size" yaml:"cache_size"`

	// FeatureDetection configures feature detection
	FeatureDetection FeatureDetectionConfig `json:"feature_detection" yaml:"feature_detection"`

	// TextDetection configures text detection
	TextDetection TextDetectionConfig `json:"text_detection" yaml:"text_detection"`
}

// FeatureDetectionConfig configures feature detection algorithms.
type FeatureDetectionConfig struct {
	// Algorithm selects the detector (orb, sift, akaze)
	Algorithm string `json:"algorithm" yaml:"algorithm"`

	// MaxFeatures limits the number of features
	MaxFeatures int `json:"max_features" yaml:"max_features"`

	// ScaleFactor for pyramid construction
	ScaleFactor float64 `json:"scale_factor" yaml:"scale_factor"`

	// NLevels is the number of pyramid levels
	NLevels int `json:"n_levels" yaml:"n_levels"`

	// EdgeThreshold filters out edge features
	EdgeThreshold int `json:"edge_threshold" yaml:"edge_threshold"`

	// PatchSize for descriptor computation
	PatchSize int `json:"patch_size" yaml:"patch_size"`
}

// TextDetectionConfig configures text detection.
type TextDetectionConfig struct {
	// EASTModel path to EAST model file
	EASTModel string `json:"east_model" yaml:"east_model"`

	// ConfidenceThreshold for text detection
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`

	// NMSThreshold for non-maximum suppression
	NMSThreshold float64 `json:"nms_threshold" yaml:"nms_threshold"`

	// InputWidth for EAST model
	InputWidth int `json:"input_width" yaml:"input_width"`

	// InputHeight for EAST model
	InputHeight int `json:"input_height" yaml:"input_height"`
}

// OCRConfig configures OCR engines.
type OCRConfig struct {
	// Primary OCR engine
	Primary string `json:"primary" yaml:"primary"`

	// Fallback OCR engine
	Fallback string `json:"fallback" yaml:"fallback"`

	// Tesseract configuration
	Tesseract TesseractConfig `json:"tesseract" yaml:"tesseract"`

	// PaddleOCR configuration
	Paddle PaddleConfig `json:"paddle" yaml:"paddle"`

	// Chandra OCR configuration
	Chandra ChandraConfig `json:"chandra" yaml:"chandra"`

	// RapidOCR configuration
	Rapid RapidConfig `json:"rapid" yaml:"rapid"`
}

// TesseractConfig configures Tesseract OCR.
type TesseractConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// DataPath to tessdata directory
	DataPath string `json:"data_path" yaml:"data_path"`

	// Languages to recognize
	Languages []string `json:"languages" yaml:"languages"`

	// PageSegmentMode sets PSM mode
	PageSegmentMode int `json:"page_segment_mode" yaml:"page_segment_mode"`

	// OEMode sets OCR engine mode
	OEMode int `json:"o_e_mode" yaml:"o_e_mode"`

	// Variables for Tesseract configuration
	Variables map[string]string `json:"variables" yaml:"variables"`
}

// PaddleConfig configures PaddleOCR.
type PaddleConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint for PaddleOCR service
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// UseGPU for inference
	UseGPU bool `json:"use_gpu" yaml:"use_gpu"`

	// Language for recognition
	Language string `json:"language" yaml:"language"`

	// UseAngleClassification for rotated text
	UseAngleClassification bool `json:"use_angle_classification" yaml:"use_angle_classification"`

	// DetAlgorithm (DB, EAST)
	DetAlgorithm string `json:"det_algorithm" yaml:"det_algorithm"`

	// RecAlgorithm (CRNN, SVTR)
	RecAlgorithm string `json:"rec_algorithm" yaml:"rec_algorithm"`
}

// ChandraConfig configures Chandra OCR.
type ChandraConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint for Chandra service
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Model name
	Model string `json:"model" yaml:"model"`

	// MaxOutputTokens limits response length
	MaxOutputTokens int `json:"max_output_tokens" yaml:"max_output_tokens"`

	// IncludeImages extracts images
	IncludeImages bool `json:"include_images" yaml:"include_images"`

	// IncludeHeadersFooters includes headers/footers
	IncludeHeadersFooters bool `json:"include_headers_footers" yaml:"include_headers_footers"`

	// BatchSize for processing
	BatchSize int `json:"batch_size" yaml:"batch_size"`
}

// RapidConfig configures RapidOCR.
type RapidConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ModelPath to RapidOCR models
	ModelPath string `json:"model_path" yaml:"model_path"`

	// ThreadCount for inference
	ThreadCount int `json:"thread_count" yaml:"thread_count"`
}

// ModelsConfig configures ML model integrations.
type ModelsConfig struct {
	// OmniParser configuration
	OmniParser OmniParserConfig `json:"omniparser" yaml:"omniparser"`

	// UGround configuration
	UGround UGroundConfig `json:"uground" yaml:"uground"`

	// UIDETR configuration
	UIDETR UIDETRConfig `json:"uidetr" yaml:"uidetr"`

	// RFDETR configuration
	RFDETR RFDETRConfig `json:"rfdetr" yaml:"rfdetr"`

	// YOLO configuration
	YOLO YOLOConfig `json:"yolo" yaml:"yolo"`
}

// OmniParserConfig configures OmniParser V2.
type OmniParserConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint for OmniParser service
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Timeout for requests
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// BoxThreshold for detection
	BoxThreshold float64 `json:"box_threshold" yaml:"box_threshold"`

	// IOUThreshold for NMS
	IOUThreshold float64 `json:"iou_threshold" yaml:"iou_threshold"`

	// UsePaddleOCR enables PaddleOCR in OmniParser
	UsePaddleOCR bool `json:"use_paddleocr" yaml:"use_paddleocr"`
}

// UGroundConfig configures UGround visual grounding.
type UGroundConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint for vLLM service
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Model name
	Model string `json:"model" yaml:"model"`

	// Timeout for requests
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Temperature for generation (0 for deterministic)
	Temperature float64 `json:"temperature" yaml:"temperature"`

	// MaxTokens for response
	MaxTokens int `json:"max_tokens" yaml:"max_tokens"`
}

// UIDETRConfig configures UI-DETR-1.
type UIDETRConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ModelPath to ONNX model
	ModelPath string `json:"model_path" yaml:"model_path"`

	// ConfidenceThreshold for detections
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`

	// InputSize for model (width, height)
	InputSize [2]int `json:"input_size" yaml:"input_size"`

	// UseGPU for inference
	UseGPU bool `json:"use_gpu" yaml:"use_gpu"`
}

// RFDETRConfig configures RF-DETR.
type RFDETRConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Model variant (nano, small, medium, large, xl)
	Variant string `json:"variant" yaml:"variant"`

	// ModelPath to ONNX model
	ModelPath string `json:"model_path" yaml:"model_path"`

	// ConfidenceThreshold
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`

	// InputSize
	InputSize [2]int `json:"input_size" yaml:"input_size"`
}

// YOLOConfig configures YOLOv8.
type YOLOConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ModelPath to ONNX model
	ModelPath string `json:"model_path" yaml:"model_path"`

	// ConfidenceThreshold
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`

	// IOUThreshold for NMS
	IOUThreshold float64 `json:"iou_threshold" yaml:"iou_threshold"`

	// InputSize
	InputSize [2]int `json:"input_size" yaml:"input_size"`
}

// FrameworksConfig configures external framework integrations.
type FrameworksConfig struct {
	// Midscene.js configuration
	Midscene MidsceneConfig `json:"midscene" yaml:"midscene"`

	// Aguvis configuration
	Aguvis AguvisConfig `json:"aguvis" yaml:"aguvis"`

	// Optics configuration
	Optics OpticsConfig `json:"optics" yaml:"optics"`

	// Maestro configuration
	Maestro MaestroConfig `json:"maestro" yaml:"maestro"`
}

// MidsceneConfig configures Midscene.js bridge.
type MidsceneConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint for Midscene service
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Timeout for operations
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// Model for AI operations
	Model string `json:"model" yaml:"model"`

	// Provider (openai, anthropic, etc.)
	Provider string `json:"provider" yaml:"provider"`
}

// AguvisConfig configures Aguvis integration.
type AguvisConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Endpoint for Aguvis service
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// Model name
	Model string `json:"model" yaml:"model"`

	// Timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// OpticsConfig configures Optics framework adapter.
type OpticsConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ConfigPath to Optics configuration
	ConfigPath string `json:"config_path" yaml:"config_path"`

	// Timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// MaestroConfig configures Maestro integration.
type MaestroConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// BinaryPath to maestro executable
	BinaryPath string `json:"binary_path" yaml:"binary_path"`

	// FlowsDir for Maestro flows
	FlowsDir string `json:"flows_dir" yaml:"flows_dir"`

	// Timeout
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// RegressionConfig configures visual regression.
type RegressionConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Tool to use (ssim, lost-pixel, pixelmatch)
	Tool string `json:"tool" yaml:"tool"`

	// SimilarityThreshold (0-1, where 1 is identical)
	SimilarityThreshold float64 `json:"similarity_threshold" yaml:"similarity_threshold"`

	// PixelThreshold for pixel diff
	PixelThreshold float64 `json:"pixel_threshold" yaml:"pixel_threshold"`

	// BaselineDir for storing baselines
	BaselineDir string `json:"baseline_dir" yaml:"baseline_dir"`

	// DiffDir for storing diffs
	DiffDir string `json:"diff_dir" yaml:"diff_dir"`

	// LostPixel configuration
	LostPixel LostPixelConfig `json:"lost_pixel" yaml:"lost_pixel"`
}

// LostPixelConfig configures Lost Pixel integration.
type LostPixelConfig struct {
	// Enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// APIKey for Lost Pixel Platform
	APIKey string `json:"api_key" yaml:"api_key"`

	// ProjectID for Lost Pixel Platform
	ProjectID string `json:"project_id" yaml:"project_id"`

	// UseOSS uses open-source version
	UseOSS bool `json:"use_oss" yaml:"use_oss"`
}

// PerformanceConfig configures performance settings.
type PerformanceConfig struct {
	// MaxWorkers for parallel processing
	MaxWorkers int `json:"max_workers" yaml:"max_workers"`

	// FrameBufferSize for video processing
	FrameBufferSize int `json:"frame_buffer_size" yaml:"frame_buffer_size"`

	// ProcessingTimeout for operations
	ProcessingTimeout time.Duration `json:"processing_timeout" yaml:"processing_timeout"`

	// EnableProfiling enables performance profiling
	EnableProfiling bool `json:"enable_profiling" yaml:"enable_profiling"`

	// MemoryLimitMB limits memory usage
	MemoryLimitMB int `json:"memory_limit_mb" yaml:"memory_limit_mb"`
}

// LoggingConfig configures logging.
type LoggingConfig struct {
	// Level (debug, info, warn, error)
	Level string `json:"level" yaml:"level"`

	// Format (json, text)
	Format string `json:"format" yaml:"format"`

	// Output (stdout, file, both)
	Output string `json:"output" yaml:"output"`

	// FilePath for log file
	FilePath string `json:"file_path" yaml:"file_path"`

	// EnableFrameDump saves processed frames
	EnableFrameDump bool `json:"enable_frame_dump" yaml:"enable_frame_dump"`

	// FrameDumpDir for saved frames
	FrameDumpDir string `json:"frame_dump_dir" yaml:"frame_dump_dir"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		OpenCV: OpenCVConfig{
			Enabled:    true,
			GPUEnabled: false,
			GPUBackend: "cuda",
			CacheSize:  1000,
			FeatureDetection: FeatureDetectionConfig{
				Algorithm:     "orb",
				MaxFeatures:   500,
				ScaleFactor:   1.2,
				NLevels:       8,
				EdgeThreshold: 31,
				PatchSize:     31,
			},
			TextDetection: TextDetectionConfig{
				EASTModel:           "models/east/frozen_east_text_detection.pb",
				ConfidenceThreshold: 0.5,
				NMSThreshold:        0.4,
				InputWidth:          320,
				InputHeight:         320,
			},
		},
		OCR: OCRConfig{
			Primary:  "tesseract",
			Fallback: "paddle",
			Tesseract: TesseractConfig{
				Enabled:         true,
				DataPath:        "/usr/share/tesseract-ocr/4.00/tessdata",
				Languages:       []string{"eng"},
				PageSegmentMode: 3,
				OEMode:          3,
				Variables:       map[string]string{},
			},
			Paddle: PaddleConfig{
				Enabled:                true,
				Endpoint:               "http://localhost:8080",
				UseGPU:                 false,
				Language:               "en",
				UseAngleClassification: true,
				DetAlgorithm:           "DB",
				RecAlgorithm:           "CRNN",
			},
			Chandra: ChandraConfig{
				Enabled:               false,
				Endpoint:              "http://localhost:8000/v1",
				Model:                 "datalab-to/chandra-ocr-2",
				MaxOutputTokens:       8192,
				IncludeImages:         true,
				IncludeHeadersFooters: false,
				BatchSize:             1,
			},
			Rapid: RapidConfig{
				Enabled:     false,
				ModelPath:   "models/rapid/",
				ThreadCount: 4,
			},
		},
		Models: ModelsConfig{
			OmniParser: OmniParserConfig{
				Enabled:       false,
				Endpoint:      "http://localhost:8000",
				Timeout:       30 * time.Second,
				BoxThreshold:  0.5,
				IOUThreshold:  0.3,
				UsePaddleOCR:  true,
			},
			UGround: UGroundConfig{
				Enabled:     false,
				Endpoint:    "http://localhost:8000/v1",
				Model:       "osunlp/UGround-V1-7B",
				Timeout:     30 * time.Second,
				Temperature: 0,
				MaxTokens:   256,
			},
			UIDETR: UIDETRConfig{
				Enabled:             false,
				ModelPath:           "models/uidetr/ui-detr-1.onnx",
				ConfidenceThreshold: 0.7,
				InputSize:           [2]int{800, 600},
				UseGPU:              false,
			},
			RFDETR: RFDETRConfig{
				Enabled:             false,
				Variant:             "medium",
				ModelPath:           "models/rfdetr/rfdetr-medium.onnx",
				ConfidenceThreshold: 0.5,
				InputSize:           [2]int{560, 560},
			},
			YOLO: YOLOConfig{
				Enabled:             false,
				ModelPath:           "models/yolo/yolov8n.onnx",
				ConfidenceThreshold: 0.5,
				IOUThreshold:        0.45,
				InputSize:           [2]int{640, 640},
			},
		},
		Frameworks: FrameworksConfig{
			Midscene: MidsceneConfig{
				Enabled:  false,
				Endpoint: "http://localhost:3000",
				Timeout:  60 * time.Second,
				Model:    "gpt-4o",
				Provider: "openai",
			},
			Aguvis: AguvisConfig{
				Enabled:  false,
				Endpoint: "http://localhost:8000",
				Model:    "aguvis-7b",
				Timeout:  30 * time.Second,
			},
			Optics: OpticsConfig{
				Enabled:    false,
				ConfigPath: "optics-config.yaml",
				Timeout:    60 * time.Second,
			},
			Maestro: MaestroConfig{
				Enabled:    false,
				BinaryPath: "maestro",
				FlowsDir:   "maestro-flows/",
				Timeout:    300 * time.Second,
			},
		},
		Regression: RegressionConfig{
			Enabled:             true,
			Tool:                "ssim",
			SimilarityThreshold: 0.95,
			PixelThreshold:      0.1,
			BaselineDir:         "baselines/",
			DiffDir:             "diffs/",
			LostPixel: LostPixelConfig{
				Enabled:   false,
				APIKey:    "",
				ProjectID: "",
				UseOSS:    true,
			},
		},
		Performance: PerformanceConfig{
			MaxWorkers:          4,
			FrameBufferSize:     30,
			ProcessingTimeout:   30 * time.Second,
			EnableProfiling:     false,
			MemoryLimitMB:       2048,
		},
		Logging: LoggingConfig{
			Level:             "info",
			Format:            "text",
			Output:            "stdout",
			FilePath:          "",
			EnableFrameDump:   false,
			FrameDumpDir:      "frames/",
		},
	}
}

// LoadConfig loads configuration from a JSON or YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	config := DefaultConfig()

	// Try JSON first, then YAML
	if err := json.Unmarshal(data, config); err != nil {
		if yamlErr := yaml.Unmarshal(data, config); yamlErr != nil {
			return nil, fmt.Errorf("parsing config (tried JSON and YAML): %w", err)
		}
	}

	return config, nil
}

// SaveConfig saves configuration to a JSON file.
func SaveConfig(config *Config, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if !c.OpenCV.Enabled {
		return nil // All other settings ignored
	}

	// Validate OCR settings
	validOCREngines := map[string]bool{
		"tesseract": true,
		"paddle":    true,
		"chandra":   true,
		"rapid":     true,
	}

	if !validOCREngines[c.OCR.Primary] {
		return fmt.Errorf("invalid primary OCR engine: %s", c.OCR.Primary)
	}

	if c.OCR.Fallback != "" && !validOCREngines[c.OCR.Fallback] {
		return fmt.Errorf("invalid fallback OCR engine: %s", c.OCR.Fallback)
	}

	// Validate feature detection algorithm
	validAlgorithms := map[string]bool{
		"orb":   true,
		"sift":  true,
		"akaze": true,
	}

	if !validAlgorithms[c.OpenCV.FeatureDetection.Algorithm] {
		return fmt.Errorf("invalid feature detection algorithm: %s", c.OpenCV.FeatureDetection.Algorithm)
	}

	// Validate thresholds
	if c.Regression.SimilarityThreshold < 0 || c.Regression.SimilarityThreshold > 1 {
		return fmt.Errorf("similarity threshold must be between 0 and 1")
	}

	return nil
}
