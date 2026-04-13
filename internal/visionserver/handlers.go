// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/png" // register PNG decoder
	"net/http"
	"strings"

	"digital.vasic.helixqa/pkg/vision/cheaper"
)

// VisionExecutor is the minimal interface the Handler requires for vision
// analysis. ResilientExecutor from pkg/vision/cheaper satisfies this
// interface directly.
type VisionExecutor interface {
	Execute(ctx context.Context, img image.Image, prompt string) (*cheaper.VisionResult, error)
}

// Handler holds the HTTP handler methods for the vision server. It is
// constructed once and shared across all HTTP requests; all methods are safe
// for concurrent use provided the executor, registry, and metrics are also
// safe for concurrent use (which they are by design).
type Handler struct {
	executor VisionExecutor
	registry *cheaper.Registry
	metrics  *cheaper.Metrics
}

// NewHandler creates a Handler that delegates vision execution to executor,
// uses registry for provider listing, and records Prometheus metrics via
// metrics. All three arguments must be non-nil.
func NewHandler(executor VisionExecutor, registry *cheaper.Registry, metrics *cheaper.Metrics) *Handler {
	return &Handler{
		executor: executor,
		registry: registry,
		metrics:  metrics,
	}
}

// analyzeRequest is the JSON body expected by HandleAnalyze.
type analyzeRequest struct {
	// Image is a base64-encoded PNG image.
	Image string `json:"image"`
	// Prompt is the natural-language question or instruction for the vision
	// model.
	Prompt string `json:"prompt"`
}

// HandleAnalyze handles POST requests for vision analysis. It decodes the
// base64 PNG from the request body, runs the VisionExecutor, and writes the
// VisionResult as JSON.
//
// Request body (JSON):
//
//	{ "image": "<base64-encoded PNG>", "prompt": "<text>" }
//
// Response (JSON): cheaper.VisionResult on success, {"error":"..."} on failure.
func (h *Handler) HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Image == "" {
		writeError(w, http.StatusBadRequest, "image field is required")
		return
	}

	imgBytes, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil {
		writeError(w, http.StatusBadRequest, "base64 decode failed: "+err.Error())
		return
	}

	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "image decode failed: "+err.Error())
		return
	}

	result, err := h.executor.Execute(r.Context(), img, req.Prompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "execution failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleListProviders handles GET requests that return the names of all
// registered vision providers.
//
// Response (JSON): { "providers": ["name1", "name2", ...] }
func (h *Handler) HandleListProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": h.registry.List(),
	})
}

// HandleHealth handles GET requests for the server liveness probe.
//
// Response (JSON): { "status": "healthy" }
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// learningStatsResponse is the JSON shape returned by HandleLearningStats.
type learningStatsResponse struct {
	// RequestsTotal is the sum of all vision analysis requests recorded by
	// the Metrics instance. Derived from the Prometheus counter family.
	RequestsTotal int64 `json:"requests_total"`
	// CacheHitsTotal is the sum of all cache hits across all layers.
	CacheHitsTotal int64 `json:"cache_hits_total"`
}

// HandleLearningStats handles GET requests that return basic learning /
// metrics statistics.
//
// Response (JSON): { "requests_total": N, "cache_hits_total": N }
func (h *Handler) HandleLearningStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Gather metrics from the Prometheus registry to compute totals.
	var reqTotal, cacheTotal int64
	mfs, err := h.metrics.Registry().Gather()
	if err == nil {
		for _, mf := range mfs {
			name := mf.GetName()
			switch {
			case strings.HasSuffix(name, "_requests_total"):
				for _, m := range mf.GetMetric() {
					reqTotal += int64(m.GetCounter().GetValue())
				}
			case strings.HasSuffix(name, "_cache_hits_total"):
				for _, m := range mf.GetMetric() {
					cacheTotal += int64(m.GetCounter().GetValue())
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, learningStatsResponse{
		RequestsTotal:  reqTotal,
		CacheHitsTotal: cacheTotal,
	})
}

// HandleClearLearning handles POST requests that clear accumulated learning
// state. In this implementation the in-memory Prometheus counters cannot be
// reset, so this endpoint simply acknowledges the request.
//
// Response (JSON): { "cleared": true }
func (h *Handler) HandleClearLearning(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"cleared": true})
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
// The Content-Type header is set to application/json. Any encoding error is
// silently dropped because the status code and partial body have already been
// sent.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response of the form {"error":"..."}.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
