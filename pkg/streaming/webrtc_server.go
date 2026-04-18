// Package streaming provides WebRTC streaming infrastructure for HelixQA
package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// SignalingMessage represents a WebRTC signaling message
type SignalingMessage struct {
	Type      string                     `json:"type"`     // "offer", "answer", "ice", "join", "leave"
	RoomID    string                     `json:"roomId"`   // Room/session identifier
	ClientID  string                     `json:"clientId"` // Unique client identifier
	SDP       *webrtc.SessionDescription `json:"sdp,omitempty"`
	ICE       *webrtc.ICECandidateInit   `json:"ice,omitempty"`
	Error     string                     `json:"error,omitempty"`
	Timestamp time.Time                  `json:"timestamp"`
}

// Client represents a connected WebRTC client
type Client struct {
	ID             string
	RoomID         string
	Socket         *websocket.Conn
	PeerConnection *webrtc.PeerConnection
	SendChan       chan []byte
	Server         *WebRTCServer
	mu             sync.RWMutex
	closed         bool
}

// Send writes a message to the client's WebSocket
func (c *Client) Send(msg []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return fmt.Errorf("client closed")
	}
	select {
	case c.SendChan <- msg:
		return nil
	default:
		return fmt.Errorf("send channel full")
	}
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.SendChan)
	if c.PeerConnection != nil {
		c.PeerConnection.Close()
	}
	if c.Socket != nil {
		c.Socket.Close()
	}
	return nil
}

// IsClosed returns true if the client is closed
func (c *Client) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

// Room manages clients in a session
type Room struct {
	ID      string
	Clients map[string]*Client
	mu      sync.RWMutex
}

// AddClient adds a client to the room
func (r *Room) AddClient(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Clients[client.ID] = client
}

// RemoveClient removes a client from the room
func (r *Room) RemoveClient(clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Clients, clientID)
}

// GetClient returns a client by ID
func (r *Room) GetClient(clientID string) (*Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.Clients[clientID]
	return client, ok
}

// Broadcast sends a message to all clients in the room except the sender
func (r *Room) Broadcast(senderID string, msg []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, client := range r.Clients {
		if id != senderID {
			client.Send(msg)
		}
	}
}

// ClientCount returns the number of clients in the room
func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Clients)
}

// WebRTCServer manages WebRTC connections and signaling
type WebRTCServer struct {
	rooms      map[string]*Room
	clients    map[string]*Client
	upgrader   websocket.Upgrader
	mu         sync.RWMutex
	iceServers []webrtc.ICEServer
	api        *webrtc.API
}

// WebRTCConfig holds server configuration
type WebRTCConfig struct {
	ICEServers        []webrtc.ICEServer
	EnableDataChannel bool
	EnableVideoTrack  bool
	EnableAudioTrack  bool
	MaxClientsPerRoom int
	ConnectionTimeout time.Duration
	EnableSTUN        bool
	EnableTURN        bool
	TURNUsername      string
	TURNPassword      string
}

// DefaultWebRTCConfig returns default configuration
func DefaultWebRTCConfig() *WebRTCConfig {
	return &WebRTCConfig{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
		EnableDataChannel: true,
		EnableVideoTrack:  true,
		EnableAudioTrack:  false,
		MaxClientsPerRoom: 10,
		ConnectionTimeout: 30 * time.Second,
		EnableSTUN:        true,
		EnableTURN:        false,
	}
}

// NewWebRTCServer creates a new WebRTC server
func NewWebRTCServer(config *WebRTCConfig) *WebRTCServer {
	if config == nil {
		config = DefaultWebRTCConfig()
	}

	// Create setting engine for custom configuration
	settingEngine := webrtc.SettingEngine{}

	// Configure ICE timeout
	settingEngine.SetICETimeouts(
		config.ConnectionTimeout,
		config.ConnectionTimeout,
		2*time.Second, // keepAliveInterval
	)

	// Create API with custom settings
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))

	return &WebRTCServer{
		rooms:   make(map[string]*Room),
		clients: make(map[string]*Client),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins in development
				// In production, validate against allowed origins
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		iceServers: config.ICEServers,
		api:        api,
	}
}

// HandleWebSocket upgrades HTTP connection to WebSocket and handles signaling
func (s *WebRTCServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	socket, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Create client
	client := &Client{
		ID:       generateClientID(),
		Socket:   socket,
		SendChan: make(chan []byte, 256),
		Server:   s,
	}

	// Register client
	s.registerClient(client)

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()

	log.Printf("WebRTC client connected: %s", client.ID)
}

// readPump reads messages from the WebSocket
func (c *Client) readPump() {
	defer func() {
		c.Server.unregisterClient(c)
		c.Close()
	}()

	c.Socket.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Socket.SetPongHandler(func(string) error {
		c.Socket.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		if err := c.handleMessage(message); err != nil {
			log.Printf("Message handling error: %v", err)
			c.sendError(err.Error())
		}
	}
}

// writePump writes messages to the WebSocket
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Socket.Close()
	}()

	for {
		select {
		case message, ok := <-c.SendChan:
			c.Socket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.Socket.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.Socket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming signaling messages
func (c *Client) handleMessage(data []byte) error {
	var msg SignalingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("invalid message format: %w", err)
	}

	msg.ClientID = c.ID
	msg.Timestamp = time.Now()

	switch msg.Type {
	case "join":
		return c.handleJoin(&msg)
	case "leave":
		return c.handleLeave(&msg)
	case "offer":
		return c.handleOffer(&msg)
	case "answer":
		return c.handleAnswer(&msg)
	case "ice":
		return c.handleICE(&msg)
	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleJoin processes room join request
func (c *Client) handleJoin(msg *SignalingMessage) error {
	if msg.RoomID == "" {
		return fmt.Errorf("room ID required")
	}

	c.RoomID = msg.RoomID
	room := c.Server.getOrCreateRoom(msg.RoomID)
	room.AddClient(c)

	// Send join confirmation
	response := SignalingMessage{
		Type:      "joined",
		RoomID:    msg.RoomID,
		ClientID:  c.ID,
		Timestamp: time.Now(),
	}
	return c.sendMessage(&response)
}

// handleLeave processes room leave request
func (c *Client) handleLeave(msg *SignalingMessage) error {
	if c.RoomID != "" {
		room := c.Server.getRoom(c.RoomID)
		if room != nil {
			room.RemoveClient(c.ID)
		}
		c.RoomID = ""
	}

	// Clean up peer connection
	if c.PeerConnection != nil {
		c.PeerConnection.Close()
		c.PeerConnection = nil
	}

	return nil
}

// handleOffer processes WebRTC offer
func (c *Client) handleOffer(msg *SignalingMessage) error {
	if msg.SDP == nil {
		return fmt.Errorf("SDP offer required")
	}

	// Create peer connection if not exists
	if c.PeerConnection == nil {
		if err := c.createPeerConnection(); err != nil {
			return err
		}
	}

	// Set remote description
	if err := c.PeerConnection.SetRemoteDescription(*msg.SDP); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	answer, err := c.PeerConnection.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Set local description
	if err := c.PeerConnection.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("failed to set local description: %w", err)
	}

	// Send answer back
	response := SignalingMessage{
		Type:      "answer",
		RoomID:    c.RoomID,
		ClientID:  c.ID,
		SDP:       &answer,
		Timestamp: time.Now(),
	}
	return c.sendMessage(&response)
}

// handleAnswer processes WebRTC answer
func (c *Client) handleAnswer(msg *SignalingMessage) error {
	if msg.SDP == nil {
		return fmt.Errorf("SDP answer required")
	}

	if c.PeerConnection == nil {
		return fmt.Errorf("no peer connection")
	}

	// Find the target client in the room
	room := c.Server.getRoom(c.RoomID)
	if room == nil {
		return fmt.Errorf("room not found")
	}

	// Broadcast answer to other clients in the room
	data, _ := json.Marshal(msg)
	room.Broadcast(c.ID, data)

	return nil
}

// handleICE processes ICE candidate
func (c *Client) handleICE(msg *SignalingMessage) error {
	if msg.ICE == nil {
		return fmt.Errorf("ICE candidate required")
	}

	if c.PeerConnection == nil {
		// Queue ICE candidate for later
		return nil
	}

	if err := c.PeerConnection.AddICECandidate(*msg.ICE); err != nil {
		return fmt.Errorf("failed to add ICE candidate: %w", err)
	}

	return nil
}

// createPeerConnection creates a new WebRTC peer connection
func (c *Client) createPeerConnection() error {
	config := webrtc.Configuration{
		ICEServers: c.Server.iceServers,
	}

	pc, err := c.Server.api.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Handle ICE candidates
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateInit := candidate.ToJSON()
		msg := SignalingMessage{
			Type:      "ice",
			RoomID:    c.RoomID,
			ClientID:  c.ID,
			ICE:       &candidateInit,
			Timestamp: time.Now(),
		}
		c.sendMessage(&msg)
	})

	// Handle connection state changes
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer connection state changed: %s (client: %s)", state.String(), c.ID)

		switch state {
		case webrtc.PeerConnectionStateConnected:
			log.Printf("Client %s connected", c.ID)
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed:
			log.Printf("Client %s disconnected", c.ID)
			c.Close()
		case webrtc.PeerConnectionStateClosed:
			log.Printf("Client %s connection closed", c.ID)
		}
	})

	// Handle incoming tracks
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("New track received: %s (kind: %s) from client %s",
			track.ID(), track.Kind().String(), c.ID)

		// Process the track (e.g., forward to other clients, record, etc.)
		go c.handleTrack(track)
	})

	// Handle data channel
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("New data channel: %s", dc.Label())

		dc.OnOpen(func() {
			log.Printf("Data channel opened: %s", dc.Label())
		})

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			// Handle data channel messages
			log.Printf("Data channel message: %d bytes", len(msg.Data))
		})
	})

	c.PeerConnection = pc
	return nil
}

// handleTrack processes incoming media tracks
func (c *Client) handleTrack(track *webrtc.TrackRemote) {
	// Read RTP packets from the track
	buf := make([]byte, 1500)
	for {
		n, _, err := track.Read(buf)
		if err != nil {
			log.Printf("Track read error: %v", err)
			return
		}

		// Process the RTP packet
		// This is where you would forward to other clients, decode, etc.
		_ = n
	}
}

// sendMessage sends a signaling message to the client
func (c *Client) sendMessage(msg *SignalingMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Send(data)
}

// sendError sends an error message to the client
func (c *Client) sendError(errMsg string) {
	msg := SignalingMessage{
		Type:      "error",
		ClientID:  c.ID,
		Error:     errMsg,
		Timestamp: time.Now(),
	}
	c.sendMessage(&msg)
}

// registerClient adds a client to the server
func (s *WebRTCServer) registerClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ID] = client
}

// unregisterClient removes a client from the server
func (s *WebRTCServer) unregisterClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from room
	if client.RoomID != "" {
		if room, ok := s.rooms[client.RoomID]; ok {
			room.RemoveClient(client.ID)
		}
	}

	delete(s.clients, client.ID)
	log.Printf("Client unregistered: %s", client.ID)
}

// getOrCreateRoom returns existing room or creates new one
func (s *WebRTCServer) getOrCreateRoom(roomID string) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room, ok := s.rooms[roomID]; ok {
		return room
	}

	room := &Room{
		ID:      roomID,
		Clients: make(map[string]*Client),
	}
	s.rooms[roomID] = room
	return room
}

// getRoom returns a room by ID
func (s *WebRTCServer) getRoom(roomID string) *Room {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rooms[roomID]
}

// GetRoomStats returns statistics for a room
func (s *WebRTCServer) GetRoomStats(roomID string) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return nil
	}

	return map[string]interface{}{
		"roomId":      roomID,
		"clientCount": room.ClientCount(),
	}
}

// GetServerStats returns overall server statistics
func (s *WebRTCServer) GetServerStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"totalClients": len(s.clients),
		"totalRooms":   len(s.rooms),
	}
}

// Shutdown gracefully shuts down the server
func (s *WebRTCServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	clients := make([]*Client, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	// Close all clients
	for _, client := range clients {
		client.Close()
	}

	return nil
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}
