// Package vision provides LLM-based vision analysis using Ollama
package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os/exec"
	"time"
)

// OllamaConfig configures the Ollama client
type OllamaConfig struct {
	// Ollama server endpoint
	Endpoint string
	
	// Model to use (llava, llava:13b, llava:34b, etc.)
	Model string
	
	// Timeout for requests
	Timeout time.Duration
	
	// Temperature for generation (0-1)
	Temperature float64
	
	// Maximum tokens to generate
	MaxTokens int
	
	// System prompt
	SystemPrompt string
}

// DefaultOllamaConfig returns default Ollama configuration
func DefaultOllamaConfig() *OllamaConfig {
	return &OllamaConfig{
		Endpoint:     "http://localhost:11434",
		Model:        "llava",
		Timeout:      60 * time.Second,
		Temperature:  0.7,
		MaxTokens:    2048,
		SystemPrompt: "You are a UI analysis assistant. Describe what you see in the image, including UI elements, text, layout, and any interactive components.",
	}
}

// OllamaClient provides LLM vision analysis
type OllamaClient struct {
	config *OllamaConfig
	client *http.Client
}

// OllamaRequest represents the API request
type OllamaRequest struct {
	Model    string    `json:"model"`
	Prompt   string    `json:"prompt"`
	Images   []string  `json:"images,omitempty"`
	Stream   bool      `json:"stream"`
	System   string    `json:"system,omitempty"`
	Options  Options   `json:"options,omitempty"`
}

// Options represents generation options
type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// OllamaResponse represents the API response
type OllamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	Context   []int  `json:"context,omitempty"`
}

// UIAnalysisResult contains structured UI analysis
type UIAnalysisResult struct {
	Description string       `json:"description"`
	Elements    []UIElement  `json:"elements"`
	Layout      LayoutInfo   `json:"layout"`
	Actions     []ActionInfo `json:"actions,omitempty"`
	RawResponse string       `json:"raw_response"`
	LatencyMs   float64      `json:"latency_ms"`
}

// UIElement represents a detected UI element from LLM
type UIElement struct {
	Type        string  `json:"type"`
	Label       string  `json:"label,omitempty"`
	Description string  `json:"description,omitempty"`
	Location    string  `json:"location,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}

// LayoutInfo describes the overall layout
type LayoutInfo struct {
	Type        string `json:"type"`
	Structure   string `json:"structure,omitempty"`
	ColorScheme string `json:"color_scheme,omitempty"`
}

// ActionInfo describes possible actions
type ActionInfo struct {
	Action      string `json:"action"`
	Target      string `json:"target"`
	Description string `json:"description,omitempty"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(config *OllamaConfig) (*OllamaClient, error) {
	if config == nil {
		config = DefaultOllamaConfig()
	}
	
	return &OllamaClient{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// AnalyzeImage analyzes an image and returns UI description
func (o *OllamaClient) AnalyzeImage(img image.Image, prompt string) (*UIAnalysisResult, error) {
	start := time.Now()
	
	if prompt == "" {
		prompt = "Describe this UI screenshot. What elements do you see? What is the layout? What actions can be performed?"
	}
	
	// Convert image to base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}
	imgBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	
	// Create request
	reqBody := OllamaRequest{
		Model:   o.config.Model,
		Prompt:  prompt,
		Images:  []string{imgBase64},
		Stream:  false,
		System:  o.config.SystemPrompt,
		Options: Options{
			Temperature: o.config.Temperature,
			NumPredict:  o.config.MaxTokens,
			TopK:        40,
			TopP:        0.9,
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Send request
	req, err := http.NewRequest("POST", o.config.Endpoint+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	
	// Parse response
	var result OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	latency := time.Since(start).Milliseconds()
	
	// Parse structured result
	return o.parseAnalysis(result.Response, float64(latency)), nil
}

// AnalyzeImageWithContext analyzes image with context cancellation
func (o *OllamaClient) AnalyzeImageWithContext(ctx context.Context, img image.Image, prompt string) (*UIAnalysisResult, error) {
	done := make(chan struct{})
	var result *UIAnalysisResult
	var err error
	
	go func() {
		result, err = o.AnalyzeImage(img, prompt)
		close(done)
	}()
	
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		return result, err
	}
}

// parseAnalysis extracts structured data from LLM response
func (o *OllamaClient) parseAnalysis(response string, latency float64) *UIAnalysisResult {
	result := &UIAnalysisResult{
		Description: response,
		RawResponse: response,
		LatencyMs:   latency,
		Elements:    make([]UIElement, 0),
	}
	
	// Try to extract structured elements from response
	// This is a simplified parser - in production, use more sophisticated parsing
	
	return result
}

// CheckOllamaAvailable checks if Ollama server is available
func CheckOllamaAvailable(endpoint string) bool {
	if endpoint == "" {
		endpoint = DefaultOllamaConfig().Endpoint
	}
	
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}

// GetAvailableModels returns list of available models
func GetAvailableModels(endpoint string) ([]string, error) {
	if endpoint == "" {
		endpoint = DefaultOllamaConfig().Endpoint
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}
	
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}
	
	return models, nil
}

// PullModel pulls a model from Ollama registry
func PullModel(model, endpoint string) error {
	if endpoint == "" {
		endpoint = DefaultOllamaConfig().Endpoint
	}
	
	reqBody := map[string]string{
		"name": model,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	
	resp, err := http.Post(
		endpoint+"/api/pull",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to pull model: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull failed with status %d", resp.StatusCode)
	}
	
	return nil
}

// OllamaService manages a local Ollama service
type OllamaService struct {
	cmd    *exec.Cmd
	config *OllamaConfig
}

// NewOllamaService creates and starts a local Ollama service
func NewOllamaService() (*OllamaService, error) {
	cmd := exec.Command("ollama", "serve")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ollama: %w", err)
	}
	
	// Wait for service to be ready
	time.Sleep(2 * time.Second)
	
	return &OllamaService{
		cmd:    cmd,
		config: DefaultOllamaConfig(),
	}, nil
}

// Stop stops the Ollama service
func (s *OllamaService) Stop() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// IsRunning checks if the service is running
func (s *OllamaService) IsRunning() bool {
	return CheckOllamaAvailable(s.config.Endpoint)
}

// LLMStats holds LLM usage statistics
type LLMStats struct {
	ImagesAnalyzed uint64
	TotalTokens    uint64
	TotalTimeMs    uint64
	Errors         uint64
}

// VisionLLM combines multiple vision analysis methods
type VisionLLM struct {
	ollama   *OllamaClient
	detector *ElementDetector
	stats    LLMStats
}

// NewVisionLLM creates a comprehensive vision analysis pipeline
func NewVisionLLM(ollamaConfig *OllamaConfig, detectorConfig DetectorConfig) (*VisionLLM, error) {
	ollama, err := NewOllamaClient(ollamaConfig)
	if err != nil {
		return nil, err
	}
	
	detector := NewElementDetector(detectorConfig)
	
	return &VisionLLM{
		ollama:   ollama,
		detector: detector,
	}, nil
}

// Analyze performs comprehensive UI analysis
func (v *VisionLLM) Analyze(img image.Image) (*UIAnalysisResult, error) {
	start := time.Now()
	
	// First, do traditional CV detection
	cvResult, err := v.detector.Detect(img)
	if err != nil {
		return nil, err
	}
	
	// Then, do LLM analysis
	llmResult, err := v.ollama.AnalyzeImage(img, "")
	if err != nil {
		// Return CV-only result if LLM fails
		return v.cvOnlyResult(cvResult, time.Since(start).Milliseconds()), nil
	}
	
	// Merge results
	merged := v.mergeResults(cvResult, llmResult)
	merged.LatencyMs = float64(time.Since(start).Milliseconds())
	
	// Update stats
	v.stats.ImagesAnalyzed++
	v.stats.TotalTimeMs += uint64(merged.LatencyMs)
	
	return merged, nil
}

// cvOnlyResult converts CV result to UIAnalysisResult
func (v *VisionLLM) cvOnlyResult(cvResult *FrameResult, latency int64) *UIAnalysisResult {
	result := &UIAnalysisResult{
		Description: fmt.Sprintf("Detected %d UI elements", len(cvResult.Elements)),
		LatencyMs:   float64(latency),
		Elements:    make([]UIElement, 0, len(cvResult.Elements)),
	}
	
	for _, elem := range cvResult.Elements {
		uiElem := UIElement{
			Type:       string(elem.Type),
			Label:      elem.Label,
			Confidence: elem.Confidence,
		}
		result.Elements = append(result.Elements, uiElem)
	}
	
	return result
}

// mergeResults combines CV and LLM results
func (v *VisionLLM) mergeResults(cvResult *FrameResult, llmResult *UIAnalysisResult) *UIAnalysisResult {
	// Start with LLM result
	merged := *llmResult
	
	// Add CV elements not already detected
	existingTypes := make(map[string]bool)
	for _, e := range merged.Elements {
		existingTypes[e.Type+"_"+e.Label] = true
	}
	
	for _, elem := range cvResult.Elements {
		key := string(elem.Type) + "_" + elem.Label
		if !existingTypes[key] {
			uiElem := UIElement{
				Type:       string(elem.Type),
				Label:      elem.Label,
				Confidence: elem.Confidence,
			}
			merged.Elements = append(merged.Elements, uiElem)
		}
	}
	
	return &merged
}

// GetStats returns usage statistics
func (v *VisionLLM) GetStats() LLMStats {
	return v.stats
}

// RecommendedModels lists recommended vision models
var RecommendedModels = []struct {
	Name        string
	Description string
	Size        string
}{
	{"llava", "Base LLaVA model", "4GB"},
	{"llava:13b", "LLaVA 13B parameter model", "8GB"},
	{"llava:34b", "LLaVA 34B parameter model", "20GB"},
	{"bakllava", "BakLLaVA model", "4GB"},
	{"moondream", "Tiny vision model", "2GB"},
}

// CheckGPUAvailable checks if GPU is available for Ollama
func CheckGPUAvailable() bool {
	cmd := exec.Command("ollama", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// Check output for GPU info
	return len(output) > 0
}
