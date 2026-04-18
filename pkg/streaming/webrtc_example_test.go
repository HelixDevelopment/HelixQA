package streaming

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pion/webrtc/v4"
)

// ExampleNewWebRTCServer demonstrates creating a WebRTC server
func ExampleNewWebRTCServer() {
	// Create configuration with custom ICE servers
	config := &WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
			{
				URLs:       []string{"turn:turn.example.com:3478"},
				Username:   "user",
				Credential: "password",
			},
		},
		EnableDataChannel: true,
		EnableVideoTrack:  true,
		EnableAudioTrack:  false,
		MaxClientsPerRoom: 10,
		ConnectionTimeout: 30 * time.Second,
		EnableSTUN:        true,
		EnableTURN:        true,
	}

	// Create server
	server := NewWebRTCServer(config)

	// Create HTTP handler
	handler := NewWebRTCHandler(server)

	// Register routes
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Start HTTP server
	fmt.Println("WebRTC server starting on :8080")
	go http.ListenAndServe(":8080", mux)

	// Graceful shutdown example
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server.Shutdown(ctx)
}

// ExampleWebRTCServer_HandleWebSocket demonstrates WebSocket handling
func ExampleWebRTCServer_HandleWebSocket() {
	server := NewWebRTCServer(nil)

	// The HandleWebSocket method can be used with any HTTP server
	http.HandleFunc("/ws/webrtc", func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers for browser access
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		server.HandleWebSocket(w, r)
	})

	fmt.Println("WebSocket endpoint: ws://localhost:8080/ws/webrtc")
}

// ExampleSignalingMessage_types demonstrates message types
func ExampleSignalingMessage_types() {
	messages := []SignalingMessage{
		{
			Type:     "join",
			RoomID:   "room-123",
			ClientID: "client-456",
		},
		{
			Type:     "offer",
			RoomID:   "room-123",
			ClientID: "client-456",
			SDP: &webrtc.SessionDescription{
				Type: webrtc.SDPTypeOffer,
				SDP:  "v=0\r\n...",
			},
		},
		{
			Type:     "ice",
			RoomID:   "room-123",
			ClientID: "client-456",
			ICE: &webrtc.ICECandidateInit{
				Candidate: "candidate:...",
			},
		},
	}

	for _, msg := range messages {
		fmt.Printf("Message type: %s\n", msg.Type)
	}
	// Output:
	// Message type: join
	// Message type: offer
	// Message type: ice
}

// ExampleRoom demonstrates room management
func ExampleRoom() {
	server := NewWebRTCServer(nil)

	// Get or create a room
	room := server.getOrCreateRoom("test-room")

	// Add clients to the room
	client1 := &Client{ID: "client-1"}
	client2 := &Client{ID: "client-2"}

	room.AddClient(client1)
	room.AddClient(client2)

	fmt.Printf("Room %s has %d clients\n", room.ID, room.ClientCount())
	// Output: Room test-room has 2 clients
}

// ExampleWebRTCConfig_custom demonstrates custom configuration
func ExampleWebRTCConfig_custom() {
	config := &WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun1.l.google.com:19302",
					"stun:stun2.l.google.com:19302",
				},
			},
		},
		EnableDataChannel: true,
		EnableVideoTrack:  true,
		EnableAudioTrack:  false,
		MaxClientsPerRoom: 50,
		ConnectionTimeout: 60 * time.Second,
		EnableSTUN:        true,
		EnableTURN:        false,
	}

	server := NewWebRTCServer(config)
	fmt.Println("Server created with custom config")

	_ = server
	// Output: Server created with custom config
}
