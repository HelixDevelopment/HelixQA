package streaming

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultWebRTCConfig(t *testing.T) {
	config := DefaultWebRTCConfig()

	assert.NotNil(t, config)
	assert.True(t, config.EnableSTUN)
	assert.False(t, config.EnableTURN)
	assert.True(t, config.EnableDataChannel)
	assert.True(t, config.EnableVideoTrack)
	assert.False(t, config.EnableAudioTrack)
	assert.Equal(t, 10, config.MaxClientsPerRoom)
	assert.Equal(t, 30*time.Second, config.ConnectionTimeout)
	assert.NotEmpty(t, config.ICEServers)
}

func TestNewWebRTCServer(t *testing.T) {
	config := DefaultWebRTCConfig()
	server := NewWebRTCServer(config)

	assert.NotNil(t, server)
	assert.NotNil(t, server.rooms)
	assert.NotNil(t, server.clients)
	assert.NotNil(t, server.api)
	assert.Equal(t, config.ICEServers, server.iceServers)
}

func TestWebRTCServer_HandleWebSocket(t *testing.T) {
	server := NewWebRTCServer(nil)

	// Create test HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Wait a bit for server to register client
	time.Sleep(100 * time.Millisecond)

	// Check server has registered the client
	server.mu.RLock()
	clientCount := len(server.clients)
	server.mu.RUnlock()

	assert.Equal(t, 1, clientCount)
}

func TestSignalingMessage_JoinRoom(t *testing.T) {
	server := NewWebRTCServer(nil)

	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	// Connect first client
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws1.Close()

	// Send join message
	joinMsg := SignalingMessage{
		Type:   "join",
		RoomID: "test-room",
	}
	err = ws1.WriteJSON(joinMsg)
	require.NoError(t, err)

	// Wait for response
	ws1.SetReadDeadline(time.Now().Add(2 * time.Second))
	var response SignalingMessage
	err = ws1.ReadJSON(&response)
	require.NoError(t, err)

	assert.Equal(t, "joined", response.Type)
	assert.Equal(t, "test-room", response.RoomID)

	// Verify room was created
	room := server.getRoom("test-room")
	require.NotNil(t, room)
	assert.Equal(t, 1, room.ClientCount())
}

func TestRoom_AddRemoveClient(t *testing.T) {
	room := &Room{
		ID:      "test-room",
		Clients: make(map[string]*Client),
	}

	client1 := &Client{ID: "client-1"}
	client2 := &Client{ID: "client-2"}

	// Add clients
	room.AddClient(client1)
	assert.Equal(t, 1, room.ClientCount())

	room.AddClient(client2)
	assert.Equal(t, 2, room.ClientCount())

	// Get client
	c, ok := room.GetClient("client-1")
	assert.True(t, ok)
	assert.Equal(t, "client-1", c.ID)

	// Remove client
	room.RemoveClient("client-1")
	assert.Equal(t, 1, room.ClientCount())

	_, ok = room.GetClient("client-1")
	assert.False(t, ok)
}

func TestRoom_Broadcast(t *testing.T) {
	room := &Room{
		ID:      "test-room",
		Clients: make(map[string]*Client),
	}

	// Create clients with buffered channels
	client1 := &Client{
		ID:       "client-1",
		SendChan: make(chan []byte, 10),
	}
	client2 := &Client{
		ID:       "client-2",
		SendChan: make(chan []byte, 10),
	}

	room.AddClient(client1)
	room.AddClient(client2)

	// Broadcast from client-1
	msg := []byte("test message")
	room.Broadcast("client-1", msg)

	// client-2 should receive the message
	select {
	case received := <-client2.SendChan:
		assert.Equal(t, msg, received)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for broadcast")
	}

	// client-1 should NOT receive its own message
	select {
	case <-client1.SendChan:
		t.Fatal("Sender should not receive broadcast")
	case <-time.After(100 * time.Millisecond):
		// Expected - no message
	}
}

func TestClient_Send(t *testing.T) {
	client := &Client{
		ID:       "test-client",
		SendChan: make(chan []byte, 5),
	}

	// Send message
	msg := []byte("test message")
	err := client.Send(msg)
	require.NoError(t, err)

	// Verify message in channel
	select {
	case received := <-client.SendChan:
		assert.Equal(t, msg, received)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Close client
	err = client.Close()
	require.NoError(t, err)

	// Send after close should fail
	err = client.Send(msg)
	assert.Error(t, err)
}

func TestClient_IsClosed(t *testing.T) {
	client := &Client{
		ID:       "test-client",
		SendChan: make(chan []byte),
	}

	assert.False(t, client.IsClosed())

	client.Close()

	assert.True(t, client.IsClosed())
}

func TestWebRTCServer_GetServerStats(t *testing.T) {
	server := NewWebRTCServer(nil)

	stats := server.GetServerStats()
	require.NotNil(t, stats)
	assert.Equal(t, 0, stats["totalClients"])
	assert.Equal(t, 0, stats["totalRooms"])
}

func TestWebRTCServer_GetRoomStats(t *testing.T) {
	server := NewWebRTCServer(nil)

	// Create a room
	room := server.getOrCreateRoom("test-room")
	room.AddClient(&Client{ID: "client-1"})
	room.AddClient(&Client{ID: "client-2"})

	stats := server.GetRoomStats("test-room")
	require.NotNil(t, stats)
	assert.Equal(t, "test-room", stats["roomId"])
	assert.Equal(t, 2, stats["clientCount"])

	// Non-existent room
	stats = server.GetRoomStats("non-existent")
	assert.Nil(t, stats)
}

func TestWebRTCServer_Shutdown(t *testing.T) {
	server := NewWebRTCServer(nil)

	// Create some clients
	client1 := &Client{
		ID:       "client-1",
		SendChan: make(chan []byte),
	}
	client2 := &Client{
		ID:       "client-2",
		SendChan: make(chan []byte),
	}

	server.registerClient(client1)
	server.registerClient(client2)

	assert.Equal(t, 2, len(server.clients))

	// Shutdown
	ctx := context.Background()
	err := server.Shutdown(ctx)
	require.NoError(t, err)

	// Verify clients are closed
	assert.True(t, client1.IsClosed())
	assert.True(t, client2.IsClosed())
}

func TestGenerateClientID(t *testing.T) {
	id1 := generateClientID()
	id2 := generateClientID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.True(t, strings.HasPrefix(id1, "client_"))
}

func TestSignalingMessageTypes(t *testing.T) {
	tests := []struct {
		name     string
		msgType  string
		expected string
	}{
		{"join", "join", "join"},
		{"leave", "leave", "leave"},
		{"offer", "offer", "offer"},
		{"answer", "answer", "answer"},
		{"ice", "ice", "ice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := SignalingMessage{
				Type:      tt.msgType,
				RoomID:    "test-room",
				ClientID:  "test-client",
				Timestamp: time.Now(),
			}
			assert.Equal(t, tt.expected, msg.Type)
		})
	}
}

func TestPeerConnectionConfiguration(t *testing.T) {
	config := DefaultWebRTCConfig()

	// Verify STUN server configuration
	require.NotEmpty(t, config.ICEServers)
	assert.Contains(t, config.ICEServers[0].URLs, "stun:stun.l.google.com:19302")
}

// Integration test - requires network
func TestWebRTCServer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	config := &WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
		EnableDataChannel: true,
		EnableVideoTrack:  true,
		ConnectionTimeout: 10 * time.Second,
	}

	server := NewWebRTCServer(config)

	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	// Test WebSocket connection
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Test join room
	joinMsg := SignalingMessage{
		Type:   "join",
		RoomID: "integration-test",
	}
	err = ws.WriteJSON(joinMsg)
	require.NoError(t, err)

	// Read joined response
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	var response SignalingMessage
	err = ws.ReadJSON(&response)
	require.NoError(t, err)
	assert.Equal(t, "joined", response.Type)
}
