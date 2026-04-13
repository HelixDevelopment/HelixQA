// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package showui

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestImage returns a small RGBA image suitable for encode/decode tests.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	return img
}

// TestNewShowUIProvider_Defaults verifies that omitting all optional keys
// results in the expected default values.
func TestNewShowUIProvider_Defaults(t *testing.T) {
	provider, err := NewShowUIProvider(map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, provider)

	p, ok := provider.(*ShowUIProvider)
	require.True(t, ok, "provider should be *ShowUIProvider")

	assert.Equal(t, defaultAPIURL, p.apiURL)
	assert.Equal(t, defaultTimeout, p.timeout)
	assert.NotNil(t, p.client)
}

// TestShowUI_Analyze_Success verifies that a well-formed Gradio response is
// parsed and returned as a populated VisionResult.
func TestShowUI_Analyze_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/predict", r.URL.Path)

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		data, ok := body["data"].([]interface{})
		require.True(t, ok, "request body should contain a 'data' array")
		assert.Len(t, data, 2, "data array should have image and prompt")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []string{"Login button found"},
		})
	}))
	defer srv.Close()

	provider, err := NewShowUIProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(4, 4), "find login button")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Login button found", result.Text)
	assert.Equal(t, "showui-2b", result.Provider)
	assert.Equal(t, "ShowUI-2B", result.Model)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
	assert.False(t, result.Timestamp.IsZero())
}

// TestShowUI_Analyze_APIError verifies that a non-2xx HTTP response is
// reported as an error.
func TestShowUI_Analyze_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	provider, err := NewShowUIProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(4, 4), "some prompt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestShowUI_Analyze_EmptyData verifies that a response with an empty data
// array is treated as an error.
func TestShowUI_Analyze_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []string{},
		})
	}))
	defer srv.Close()

	provider, err := NewShowUIProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(4, 4), "some prompt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestShowUI_HealthCheck_Healthy verifies that a 200 OK from the base URL
// results in a nil error.
func TestShowUI_HealthCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	provider, err := NewShowUIProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())
	assert.NoError(t, err)
}

// TestShowUI_HealthCheck_Unhealthy verifies that a 503 response is reported
// as an error.
func TestShowUI_HealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	provider, err := NewShowUIProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())
	assert.Error(t, err)
}

// TestShowUI_Name verifies the registered provider name.
func TestShowUI_Name(t *testing.T) {
	provider, err := NewShowUIProvider(map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "showui-2b", provider.Name())
}

// TestShowUI_GetCapabilities verifies that the capability values match the
// specification.
func TestShowUI_GetCapabilities(t *testing.T) {
	provider, err := NewShowUIProvider(map[string]interface{}{})
	require.NoError(t, err)

	caps := provider.GetCapabilities()

	assert.Equal(t, 10*1024*1024, caps.MaxImageSize)
	assert.ElementsMatch(t, []string{"png", "jpg", "jpeg"}, caps.SupportedFormats)
	assert.Equal(t, int64(500), caps.AverageLatency.Milliseconds())
	assert.Equal(t, 0.0, caps.CostPer1MTokens)
}

// TestShowUI_GetCostEstimate verifies that the provider always returns 0.0
// (free local inference).
func TestShowUI_GetCostEstimate(t *testing.T) {
	provider, err := NewShowUIProvider(map[string]interface{}{})
	require.NoError(t, err)

	assert.Equal(t, 0.0, provider.GetCostEstimate(1024*1024, 200))
	assert.Equal(t, 0.0, provider.GetCostEstimate(0, 0))
}
