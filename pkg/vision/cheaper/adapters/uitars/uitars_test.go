// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package uitars

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestImage returns a small RGBA image suitable for encoding in tests.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	return img
}

// validCompletionsResponse returns a minimal OpenAI-compatible JSON body with
// one choice.
func validCompletionsResponse(content string) []byte {
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{
				"message": map[string]string{
					"content": content,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

// TestNewUITARSProvider_MissingKey verifies that omitting "api_key" returns an
// error and no provider.
func TestNewUITARSProvider_MissingKey(t *testing.T) {
	provider, err := NewUITARSProvider(map[string]interface{}{})

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "api_key")
}

// TestNewUITARSProvider_Defaults verifies that optional fields fall back to
// their documented defaults when absent from the config map.
func TestNewUITARSProvider_Defaults(t *testing.T) {
	provider, err := NewUITARSProvider(map[string]interface{}{
		"api_key": "hf_test_token",
	})

	require.NoError(t, err)
	require.NotNil(t, provider)

	p := provider.(*UITARSProvider)
	assert.Equal(t, defaultBaseURL, p.baseURL)
	assert.Equal(t, defaultModel, p.model)
	assert.Equal(t, defaultTimeout, p.timeout)
}

// TestUITARS_Analyze_Success verifies that a valid API response is parsed into
// a correctly populated VisionResult.
func TestUITARS_Analyze_Success(t *testing.T) {
	expectedText := "Tap the Login button at the top right corner."

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer hf_test", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(validCompletionsResponse(expectedText))
	}))
	defer srv.Close()

	provider, err := NewUITARSProvider(map[string]interface{}{
		"api_key":  "hf_test",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	img := newTestImage(4, 4)
	result, err := provider.Analyze(context.Background(), img, "What should I tap?")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expectedText, result.Text)
	assert.Equal(t, providerName, result.Provider)
	assert.Equal(t, defaultModel, result.Model)
	assert.False(t, result.Timestamp.IsZero())
	assert.Greater(t, result.Duration, time.Duration(0))
}

// TestUITARS_Analyze_APIError verifies that a non-200 response from the API
// surfaces as an error.
func TestUITARS_Analyze_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	provider, err := NewUITARSProvider(map[string]interface{}{
		"api_key":  "hf_test",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(2, 2), "prompt")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "500")
}

// TestUITARS_Analyze_EmptyChoices verifies that an API response with an empty
// "choices" array surfaces as an error.
func TestUITARS_Analyze_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	provider, err := NewUITARSProvider(map[string]interface{}{
		"api_key":  "hf_test",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(2, 2), "prompt")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no choices")
}

// TestUITARS_HealthCheck_Healthy verifies that a 200 response from /health
// is treated as healthy (nil error).
func TestUITARS_HealthCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, "Bearer hf_healthy", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	provider, err := NewUITARSProvider(map[string]interface{}{
		"api_key":  "hf_healthy",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())
	assert.NoError(t, err)
}

// TestUITARS_HealthCheck_Unhealthy verifies that a non-2xx response from
// /health surfaces as an error.
func TestUITARS_HealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	provider, err := NewUITARSProvider(map[string]interface{}{
		"api_key":  "hf_test",
		"base_url": srv.URL,
	})
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

// TestUITARS_Name verifies the provider returns the expected name constant.
func TestUITARS_Name(t *testing.T) {
	provider, err := NewUITARSProvider(map[string]interface{}{"api_key": "tok"})
	require.NoError(t, err)
	assert.Equal(t, "ui-tars-1.5", provider.Name())
}

// TestUITARS_GetCapabilities verifies that the capability values match the
// specification (20 MB max, png/jpg/jpeg/webp, 2s latency, cost 0).
func TestUITARS_GetCapabilities(t *testing.T) {
	provider, err := NewUITARSProvider(map[string]interface{}{"api_key": "tok"})
	require.NoError(t, err)

	caps := provider.GetCapabilities()

	assert.Equal(t, 20*1024*1024, caps.MaxImageSize)
	assert.ElementsMatch(t, []string{"png", "jpg", "jpeg", "webp"}, caps.SupportedFormats)
	assert.Equal(t, 2*time.Second, caps.AverageLatency)
	assert.Equal(t, float64(0), caps.CostPer1MTokens)
}

// TestUITARS_GetCostEstimate verifies the nominal cost estimate value.
func TestUITARS_GetCostEstimate(t *testing.T) {
	provider, err := NewUITARSProvider(map[string]interface{}{"api_key": "tok"})
	require.NoError(t, err)

	cost := provider.GetCostEstimate(1024*1024, 100)
	assert.Equal(t, 0.0001, cost)
}
