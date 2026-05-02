package screenshot

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// HTTPHandler provides HTTP endpoints for screenshot capture using standard library handlers.
type HTTPHandler struct {
	manager *Manager
}

// NewHTTPHandler creates a new screenshot HTTP handler.
func NewHTTPHandler(manager *Manager) *HTTPHandler {
	return &HTTPHandler{manager: manager}
}

// RegisterRoutes registers screenshot routes on the given mux.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/capture", h.handleCapture)
	mux.HandleFunc(prefix+"/capture/", h.handleCaptureByPlatform)
	mux.HandleFunc(prefix+"/engines", h.handleListEngines)
}

// CaptureRequest is the request body for screenshot capture.
type CaptureRequest struct {
	Platform   string `json:"platform"`
	Format     string `json:"format,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	FullPage   bool   `json:"full_page,omitempty"`
	MaxRetries int    `json:"max_retries,omitempty"`
}

// CaptureResponse is the response for screenshot capture.
type CaptureResponse struct {
	Success   bool   `json:"success"`
	Data      []byte `json:"data"`
	Format    string `json:"format"`
	Platform  string `json:"platform"`
	Engine    string `json:"engine"`
	Timestamp string `json:"timestamp"`
	Duration  string `json:"duration"`
	Error     string `json:"error,omitempty"`
}

func (h *HTTPHandler) handleCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	platform := config.Platform(req.Platform)
	opts := CaptureOptions{
		Format:     req.Format,
		Width:      req.Width,
		Height:     req.Height,
		FullPage:   req.FullPage,
		MaxRetries: req.MaxRetries,
	}

	result, err := h.manager.Capture(r.Context(), platform, opts)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CaptureResponse{
			Success:  false,
			Platform: req.Platform,
			Error:    err.Error(),
		})
		return
	}

	resp := CaptureResponse{
		Success:   true,
		Data:      result.Data,
		Format:    result.Format,
		Platform:  string(result.Platform),
		Engine:    result.Engine,
		Timestamp: result.Timestamp.Format(time.RFC3339),
		Duration:  result.Duration.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HTTPHandler) handleCaptureByPlatform(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract platform from path: /prefix/capture/{platform}
	path := r.URL.Path
	var platform string
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			platform = path[i+1:]
			break
		}
	}
	if platform == "" {
		http.Error(w, "platform required", http.StatusBadRequest)
		return
	}

	opts := CaptureOptions{
		Width:  1280,
		Height: 720,
	}

	result, err := h.manager.Capture(r.Context(), config.Platform(platform), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	encode := r.URL.Query().Get("encode")
	if encode == "base64" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(base64.StdEncoding.EncodeToString(result.Data)))
		return
	}

	contentType := "image/png"
	if result.Format == "txt" {
		contentType = "text/plain"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(result.Data)))
	w.Header().Set("X-Screenshot-Engine", result.Engine)
	w.Header().Set("X-Screenshot-Platform", string(result.Platform))
	w.Header().Set("X-Screenshot-Timestamp", result.Timestamp.Format(time.RFC3339))
	w.Header().Set("X-Screenshot-Duration", result.Duration.String())
	w.WriteHeader(http.StatusOK)
	w.Write(result.Data)
}

func (h *HTTPHandler) handleListEngines(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	platforms := h.manager.SupportedPlatforms(r.Context())
	var engines []string
	for _, p := range platforms {
		engines = append(engines, string(p))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"engines": engines})
}
