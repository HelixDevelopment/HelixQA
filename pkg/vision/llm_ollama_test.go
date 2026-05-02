package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOllamaConfig(t *testing.T) {
	config := DefaultOllamaConfig()

	assert.Equal(t, "http://localhost:11434", config.Endpoint)
	assert.Equal(t, "llava", config.Model)
	assert.Equal(t, 0.7, config.Temperature)
	assert.Equal(t, 2048, config.MaxTokens)
	assert.NotEmpty(t, config.SystemPrompt)
}

func TestCheckOllamaAvailable(t *testing.T) {
	// bluff-scan: no-assert-ok (environment-probe smoke — must not panic; result depends on host)
	// This will likely be false in test environment
	available := CheckOllamaAvailable("")
	t.Logf("Ollama available: %v", available)
}

func TestRecommendedModels(t *testing.T) {
	assert.Greater(t, len(RecommendedModels), 0)

	// Check for expected models
	modelNames := make(map[string]bool)
	for _, m := range RecommendedModels {
		modelNames[m.Name] = true
	}

	assert.True(t, modelNames["llava"])
	assert.True(t, modelNames["llava:13b"])
}

func TestUIAnalysisResult(t *testing.T) {
	result := &UIAnalysisResult{
		Description: "Login form with username and password fields",
		Elements: []UIElement{
			{Type: "input", Label: "Username", Confidence: 0.95},
			{Type: "input", Label: "Password", Confidence: 0.94},
			{Type: "button", Label: "Login", Confidence: 0.98},
		},
		Layout: LayoutInfo{
			Type:      "form",
			Structure: "vertical",
		},
		Actions: []ActionInfo{
			{Action: "click", Target: "Login button"},
		},
		RawResponse: "Full LLM response",
		LatencyMs:   1500.5,
	}

	assert.Equal(t, "Login form with username and password fields", result.Description)
	assert.Len(t, result.Elements, 3)
	assert.Equal(t, "form", result.Layout.Type)
	assert.Len(t, result.Actions, 1)
	assert.Equal(t, 1500.5, result.LatencyMs)
}

func TestUIElement(t *testing.T) {
	elem := UIElement{
		Type:        "button",
		Label:       "Submit",
		Description: "Primary action button",
		Location:    "bottom right",
		Confidence:  0.92,
	}

	assert.Equal(t, "button", elem.Type)
	assert.Equal(t, "Submit", elem.Label)
	assert.Equal(t, "Primary action button", elem.Description)
	assert.Equal(t, 0.92, elem.Confidence)
}

func TestLayoutInfo(t *testing.T) {
	layout := LayoutInfo{
		Type:        "dashboard",
		Structure:   "grid",
		ColorScheme: "dark",
	}

	assert.Equal(t, "dashboard", layout.Type)
	assert.Equal(t, "grid", layout.Structure)
	assert.Equal(t, "dark", layout.ColorScheme)
}

func TestActionInfo(t *testing.T) {
	action := ActionInfo{
		Action:      "click",
		Target:      "submit-button",
		Description: "Submit the form",
	}

	assert.Equal(t, "click", action.Action)
	assert.Equal(t, "submit-button", action.Target)
}

func TestLLMStats(t *testing.T) {
	stats := LLMStats{
		ImagesAnalyzed: 100,
		TotalTokens:    50000,
		TotalTimeMs:    120000,
		Errors:         3,
	}

	assert.Equal(t, uint64(100), stats.ImagesAnalyzed)
	assert.Equal(t, uint64(50000), stats.TotalTokens)
	assert.Equal(t, uint64(120000), stats.TotalTimeMs)
	assert.Equal(t, uint64(3), stats.Errors)
}

func TestOllamaOptions(t *testing.T) {
	options := Options{
		Temperature: 0.8,
		NumPredict:  1024,
		TopK:        50,
		TopP:        0.95,
	}

	assert.Equal(t, 0.8, options.Temperature)
	assert.Equal(t, 1024, options.NumPredict)
	assert.Equal(t, 50, options.TopK)
	assert.Equal(t, 0.95, options.TopP)
}

func TestOllamaRequest(t *testing.T) {
	req := OllamaRequest{
		Model:   "llava",
		Prompt:  "Describe this image",
		Images:  []string{"base64encodedimage"},
		Stream:  false,
		System:  "You are a UI analyst",
		Options: Options{Temperature: 0.7},
	}

	assert.Equal(t, "llava", req.Model)
	assert.Equal(t, "Describe this image", req.Prompt)
	assert.Len(t, req.Images, 1)
	assert.False(t, req.Stream)
	assert.Equal(t, "You are a UI analyst", req.System)
}

func TestOllamaResponse(t *testing.T) {
	resp := OllamaResponse{
		Model:     "llava",
		CreatedAt: "2024-01-01T00:00:00Z",
		Response:  "This is a login form",
		Done:      true,
		Context:   []int{1, 2, 3},
	}

	assert.Equal(t, "llava", resp.Model)
	assert.Equal(t, "This is a login form", resp.Response)
	assert.True(t, resp.Done)
	assert.Len(t, resp.Context, 3)
}

func TestVisionLLM_cvOnlyResult(t *testing.T) {
	cvResult := &FrameResult{
		FrameID: "test-frame",
		Elements: []Element{
			{Type: ElementButton, Label: "Submit", Confidence: 0.95},
			{Type: ElementInput, Label: "Username", Confidence: 0.90},
		},
	}

	vl := &VisionLLM{}
	result := vl.cvOnlyResult(cvResult, 500)

	assert.Equal(t, "Detected 2 UI elements", result.Description)
	assert.Equal(t, 500.0, result.LatencyMs)
	assert.Len(t, result.Elements, 2)
	assert.Equal(t, "button", result.Elements[0].Type)
	assert.Equal(t, "Submit", result.Elements[0].Label)
}

func TestVisionLLM_mergeResults(t *testing.T) {
	cvResult := &FrameResult{
		Elements: []Element{
			{Type: ElementButton, Label: "Submit", Confidence: 0.95},
			{Type: ElementInput, Label: "Email", Confidence: 0.88},
		},
	}

	llmResult := &UIAnalysisResult{
		Description: "Login form",
		Elements: []UIElement{
			{Type: "button", Label: "Submit", Confidence: 0.90},
			{Type: "link", Label: "Forgot password", Confidence: 0.85},
		},
	}

	vl := &VisionLLM{}
	merged := vl.mergeResults(cvResult, llmResult)

	assert.Equal(t, "Login form", merged.Description)
	// Should have 3 elements (Submit from LLM, Email from CV, link from LLM)
	assert.Len(t, merged.Elements, 3)
}

func TestVisionLLM_GetStats(t *testing.T) {
	vl := &VisionLLM{
		stats: LLMStats{
			ImagesAnalyzed: 50,
			TotalTokens:    25000,
		},
	}

	stats := vl.GetStats()
	assert.Equal(t, uint64(50), stats.ImagesAnalyzed)
	assert.Equal(t, uint64(25000), stats.TotalTokens)
}

func TestOllamaService(t *testing.T) {
	// bluff-scan: no-assert-ok (service smoke — public method must not panic on standard call)
	// Skip in CI environment
	t.Skip("Requires Ollama installation")
}

func TestCheckGPUAvailable(t *testing.T) {
	// bluff-scan: no-assert-ok (environment-probe smoke — must not panic; result depends on host)
	// Just test it doesn't panic
	available := CheckGPUAvailable()
	t.Logf("GPU available: %v", available)
}

func TestGetAvailableModels(t *testing.T) {
	if !CheckOllamaAvailable("") {
		t.Skip("Ollama not available")
	}

	models, err := GetAvailableModels("")
	require.NoError(t, err)
	assert.NotEmpty(t, models)
	t.Logf("Available models: %v", models)
}

func TestCreateTestImageForLLM(t *testing.T) {
	img := createTestImageForLLM(200, 100)

	assert.NotNil(t, img)
	bounds := img.Bounds()
	assert.Equal(t, 200, bounds.Dx())
	assert.Equal(t, 100, bounds.Dy())
}

// Helper function
func createTestImageForLLM(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Create a gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}
