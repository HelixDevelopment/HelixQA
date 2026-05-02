// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package omniparser

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

// newTestImage returns a small RGBA image suitable for encoding tests.
func newTestImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	return img
}

// TestNewOmniParserProvider_Defaults verifies that omitting all optional keys
// results in the expected default values.

// TestOmniParser_Analyze_Success verifies that a well-formed Gradio response is
// parsed and returned as a populated VisionResult with the correct fields.
func TestOmniParser_Analyze_Success(t *testing.T) {
	const parsedElements = `[{"type":"button","caption":"Login","bbox":[10,20,80,40]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/predict", r.URL.Path)

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		data, ok := body["data"].([]interface{})
		require.True(t, ok, "request body should contain a 'data' array")
		assert.Len(t, data, 2, "data array should have image (base64) and prompt")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []string{parsedElements},
		})
	}))
	defer srv.Close()

	provider, err := NewOmniParserProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(4, 4), "find all buttons")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, parsedElements, result.Text)
	assert.Equal(t, "omniparser-v2", result.Provider)
	assert.Equal(t, "OmniParser-V2", result.Model)
	assert.Greater(t, result.Duration.Nanoseconds(), int64(0))
	assert.False(t, result.Timestamp.IsZero())
}

// TestOmniParser_Analyze_APIError verifies that a non-2xx HTTP response is
// reported as an error with a nil result.
func TestOmniParser_Analyze_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	provider, err := NewOmniParserProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(4, 4), "some prompt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestOmniParser_Analyze_EmptyData verifies that a response with an empty data
// array is treated as an error.
func TestOmniParser_Analyze_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []string{},
		})
	}))
	defer srv.Close()

	provider, err := NewOmniParserProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	result, err := provider.Analyze(context.Background(), newTestImage(4, 4), "some prompt")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestOmniParser_HealthCheck_Healthy verifies that a 200 OK from the base URL
// (apiURL with "/api/predict" stripped) results in a nil error.
func TestOmniParser_HealthCheck_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	provider, err := NewOmniParserProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())
	assert.NoError(t, err)
}

// TestOmniParser_HealthCheck_Unhealthy verifies that a 503 response from the
// base URL is reported as an error.
func TestOmniParser_HealthCheck_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	provider, err := NewOmniParserProvider(map[string]interface{}{
		"api_url": srv.URL + "/api/predict",
	})
	require.NoError(t, err)

	err = provider.HealthCheck(context.Background())
	assert.Error(t, err)
}

// TestOmniParser_Name verifies the registered provider name.
func TestOmniParser_Name(t *testing.T) {
	provider, err := NewOmniParserProvider(map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "omniparser-v2", provider.Name())
}

// TestOmniParser_GetCapabilities verifies that the capability values match the
// specification: 15 MB max image size, png/jpg/jpeg formats, 800 ms latency,
// zero cost.
func TestOmniParser_GetCapabilities(t *testing.T) {
	provider, err := NewOmniParserProvider(map[string]interface{}{})
	require.NoError(t, err)

	caps := provider.GetCapabilities()

	assert.Equal(t, 15*1024*1024, caps.MaxImageSize)
	assert.ElementsMatch(t, []string{"png", "jpg", "jpeg"}, caps.SupportedFormats)
	assert.Equal(t, int64(800), caps.AverageLatency.Milliseconds())
	assert.Equal(t, 0.0, caps.CostPer1MTokens)
}

// TestOmniParser_GetCostEstimate verifies that the provider returns the
// expected nominal cost value (0.0019 USD) regardless of input sizes.
func TestOmniParser_GetCostEstimate(t *testing.T) {
	provider, err := NewOmniParserProvider(map[string]interface{}{})
	require.NoError(t, err)

	assert.Equal(t, 0.0019, provider.GetCostEstimate(1024*1024, 200))
	assert.Equal(t, 0.0019, provider.GetCostEstimate(0, 0))
}

// TestNewOmniParserProvider_CustomConfig verifies that custom api_url and
// timeout values are honoured by the constructor.
func TestNewOmniParserProvider_CustomConfig(t *testing.T) {
	customURL := "http://192.168.1.10:7861/api/predict"
	customTimeout := 30 * time.Second

	provider, err := NewOmniParserProvider(map[string]interface{}{
		"api_url": customURL,
		"timeout": customTimeout,
	})
	require.NoError(t, err)

	p, ok := provider.(*OmniParserProvider)
	require.True(t, ok)

	assert.Equal(t, customURL, p.apiURL)
	assert.Equal(t, customTimeout, p.timeout)
}

// TestNewOmniParserProvider_TimeoutAsFloat64 verifies that a float64 timeout
// value (seconds) is correctly converted to a time.Duration.
func TestNewOmniParserProvider_TimeoutAsFloat64(t *testing.T) {
	provider, err := NewOmniParserProvider(map[string]interface{}{
		"timeout": float64(45),
	})
	require.NoError(t, err)

	p, ok := provider.(*OmniParserProvider)
	require.True(t, ok)

	assert.Equal(t, 45*time.Second, p.timeout)
}
