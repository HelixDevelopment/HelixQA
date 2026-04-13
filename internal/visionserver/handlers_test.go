// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/vision/cheaper"
)

// --- test doubles -----------------------------------------------------------

// stubExecutor is a VisionExecutor test double whose behaviour is controlled
// by the result/err fields.
type stubExecutor struct {
	result *cheaper.VisionResult
	err    error
}

func (s *stubExecutor) Execute(_ context.Context, _ image.Image, _ string) (*cheaper.VisionResult, error) {
	return s.result, s.err
}

// --- helpers ----------------------------------------------------------------

// makePNGBase64 encodes a 1×1 white PNG image to a base64 string.
func makePNGBase64(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// newTestHandler builds a Handler with a stub executor and empty registry /
// metrics — sufficient for unit tests.
func newTestHandler(t *testing.T, exec VisionExecutor) *Handler {
	t.Helper()
	reg := cheaper.NewRegistry()
	met := cheaper.NewMetrics("test_cheaper_vision")
	return NewHandler(exec, reg, met)
}

// --- TestHandleAnalyze_Success ----------------------------------------------

func TestHandleAnalyze_Success(t *testing.T) {
	want := &cheaper.VisionResult{
		Text:      "a white square",
		Provider:  "stub",
		Model:     "stub-v1",
		Timestamp: time.Now(),
	}
	exec := &stubExecutor{result: want}
	h := newTestHandler(t, exec)

	body, err := json.Marshal(analyzeRequest{
		Image:  makePNGBase64(t),
		Prompt: "describe this image",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	h.HandleAnalyze(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got cheaper.VisionResult
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, want.Text, got.Text)
	assert.Equal(t, want.Provider, got.Provider)
	assert.Equal(t, want.Model, got.Model)
}

// --- TestHandleAnalyze_InvalidJSON ------------------------------------------

func TestHandleAnalyze_InvalidJSON(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")

	h.HandleAnalyze(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "invalid JSON")
}

// --- TestHandleAnalyze_MissingImage -----------------------------------------

func TestHandleAnalyze_MissingImage(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	body, _ := json.Marshal(analyzeRequest{Image: "", Prompt: "describe"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze", bytes.NewReader(body))

	h.HandleAnalyze(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "image field is required")
}

// --- TestHandleAnalyze_InvalidBase64 ----------------------------------------

func TestHandleAnalyze_InvalidBase64(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	body, _ := json.Marshal(analyzeRequest{Image: "!!!not-base64!!!", Prompt: "describe"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze", bytes.NewReader(body))

	h.HandleAnalyze(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "base64 decode failed")
}

// --- TestHandleAnalyze_InvalidImageBytes ------------------------------------

func TestHandleAnalyze_InvalidImageBytes(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	// Valid base64 but not a valid PNG.
	badImg := base64.StdEncoding.EncodeToString([]byte("this is not a png"))
	body, _ := json.Marshal(analyzeRequest{Image: badImg, Prompt: "describe"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze", bytes.NewReader(body))

	h.HandleAnalyze(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "image decode failed")
}

// --- TestHandleAnalyze_ExecutorError ----------------------------------------

func TestHandleAnalyze_ExecutorError(t *testing.T) {
	exec := &stubExecutor{err: errors.New("provider unavailable")}
	h := newTestHandler(t, exec)

	body, _ := json.Marshal(analyzeRequest{
		Image:  makePNGBase64(t),
		Prompt: "describe",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/analyze", bytes.NewReader(body))

	h.HandleAnalyze(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "execution failed")
}

// --- TestHandleAnalyze_WrongMethod ------------------------------------------

func TestHandleAnalyze_WrongMethod(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/analyze", nil)

	h.HandleAnalyze(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// --- TestHandleListProviders ------------------------------------------------

func TestHandleListProviders(t *testing.T) {
	reg := cheaper.NewRegistry()
	reg.Register("alpha", func(_ map[string]interface{}) (cheaper.VisionProvider, error) {
		return nil, nil
	})
	reg.Register("beta", func(_ map[string]interface{}) (cheaper.VisionProvider, error) {
		return nil, nil
	})
	met := cheaper.NewMetrics("test_list_providers")
	h := NewHandler(&stubExecutor{}, reg, met)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/providers", nil)

	h.HandleListProviders(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string][]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.ElementsMatch(t, []string{"alpha", "beta"}, resp["providers"])
}

func TestHandleListProviders_WrongMethod(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/providers", nil)

	h.HandleListProviders(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// --- TestHandleHealth -------------------------------------------------------

func TestHandleHealth(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	h.HandleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "healthy", resp["status"])
}

func TestHandleHealth_WrongMethod(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/health", nil)

	h.HandleHealth(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// --- TestHandleLearningStats ------------------------------------------------

func TestHandleLearningStats(t *testing.T) {
	met := cheaper.NewMetrics("test_learning_stats")
	// Record a couple of requests so the counters are non-zero.
	met.RecordRequest("stub", 50*time.Millisecond)
	met.RecordRequest("stub", 80*time.Millisecond)
	met.RecordCacheHit("exact")

	h := NewHandler(&stubExecutor{}, cheaper.NewRegistry(), met)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/learning/stats", nil)

	h.HandleLearningStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp learningStatsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, int64(2), resp.RequestsTotal)
	assert.Equal(t, int64(1), resp.CacheHitsTotal)
}

func TestHandleLearningStats_WrongMethod(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/learning/stats", nil)

	h.HandleLearningStats(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// --- TestHandleClearLearning ------------------------------------------------

func TestHandleClearLearning(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/learning/clear", nil)

	h.HandleClearLearning(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]bool
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.True(t, resp["cleared"])
}

func TestHandleClearLearning_WrongMethod(t *testing.T) {
	h := newTestHandler(t, &stubExecutor{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/learning/clear", nil)

	h.HandleClearLearning(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
