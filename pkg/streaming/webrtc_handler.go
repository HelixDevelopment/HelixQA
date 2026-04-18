// Package streaming provides HTTP handlers for WebRTC streaming
package streaming

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	webrtcConnectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "helixqa_webrtc_connections_total",
		Help: "Total WebRTC connections",
	}, []string{"room"})

	webrtcConnectionsActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "helixqa_webrtc_connections_active",
		Help: "Active WebRTC connections",
	}, []string{"room"})

	webrtcSignalingMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "helixqa_webrtc_signaling_messages_total",
		Help: "Total signaling messages",
	}, []string{"type"})
)

// WebRTCHandler provides HTTP endpoints for WebRTC
type WebRTCHandler struct {
	server *WebRTCServer
	config *WebRTCConfig
}

// NewWebRTCHandler creates a new WebRTC HTTP handler
func NewWebRTCHandler(server *WebRTCServer) *WebRTCHandler {
	return &WebRTCHandler{
		server: server,
		config: DefaultWebRTCConfig(),
	}
}

// RegisterRoutes registers WebRTC routes on the given mux
func (h *WebRTCHandler) RegisterRoutes(mux *http.ServeMux) {
	// WebSocket endpoint for signaling
	mux.HandleFunc("/ws/webrtc", h.HandleWebSocket)

	// Configuration endpoint
	mux.HandleFunc("/api/webrtc/config", h.HandleConfig)

	// Stats endpoint
	mux.HandleFunc("/api/webrtc/stats", h.HandleStats)

	// Room stats endpoint
	mux.HandleFunc("/api/webrtc/rooms/", h.HandleRoomStats)
}

// HandleWebSocket upgrades HTTP connection to WebSocket
func (h *WebRTCHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get room ID from query parameter
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}

	// Update metrics
	webrtcConnectionsTotal.WithLabelValues(roomID).Inc()
	webrtcConnectionsActive.WithLabelValues(roomID).Inc()
	defer webrtcConnectionsActive.WithLabelValues(roomID).Dec()

	// Delegate to server
	h.server.HandleWebSocket(w, r)
}

// HandleConfig returns WebRTC configuration for clients
func (h *WebRTCHandler) HandleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return ICE servers and other config (excluding sensitive data)
	config := map[string]interface{}{
		"iceServers": h.config.ICEServers,
		"features": map[string]bool{
			"dataChannel": h.config.EnableDataChannel,
			"videoTrack":  h.config.EnableVideoTrack,
			"audioTrack":  h.config.EnableAudioTrack,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// HandleStats returns server statistics
func (h *WebRTCHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.server.GetServerStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleRoomStats returns statistics for a specific room
func (h *WebRTCHandler) HandleRoomStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract room ID from URL path
	// URL pattern: /api/webrtc/rooms/{roomId}
	roomID := r.URL.Path[len("/api/webrtc/rooms/"):]
	if roomID == "" {
		http.Error(w, "Room ID required", http.StatusBadRequest)
		return
	}

	stats := h.server.GetRoomStats(roomID)
	if stats == nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// RecordSignalingMessage records a signaling message metric
func RecordSignalingMessage(msgType string) {
	webrtcSignalingMessages.WithLabelValues(msgType).Inc()
}
