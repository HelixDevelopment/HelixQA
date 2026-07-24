package websocket

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewHub(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)

	if hub.clients == nil {
		t.Error("clients map not initialized")
	}
	if hub.rooms == nil {
		t.Error("rooms map not initialized")
	}
	if hub.broadcast == nil {
		t.Error("broadcast channel not initialized")
	}
}

func TestHub_Register(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{},
		Send:   make(chan []byte, 256),
		RoomID: "test-room",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(hub.clients))
	}
	if len(hub.rooms["test-room"]) != 1 {
		t.Errorf("expected 1 client in room, got %d", len(hub.rooms["test-room"]))
	}
	hub.mu.RUnlock()
}

func TestHub_Unregister(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{},
		Send:   make(chan []byte, 256),
		RoomID: "test-room",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 0 {
		t.Errorf("expected 0 clients after unregister, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()
}

func TestHub_BroadcastToAll(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{},
		Send:   make(chan []byte, 256),
		RoomID: "default",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	msg := Message{Type: "test", Payload: "hello"}
	hub.BroadcastToAll(msg)

	select {
	case received := <-client.Send:
		if len(received) == 0 {
			t.Error("expected non-empty message")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for broadcast")
	}
}

func TestHub_BroadcastToRoom(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client1 := &Client{
		ID:     [16]byte{1},
		Send:   make(chan []byte, 256),
		RoomID: "room-a",
	}
	client2 := &Client{
		ID:     [16]byte{2},
		Send:   make(chan []byte, 256),
		RoomID: "room-b",
	}

	hub.Register(client1)
	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)

	msg := Message{Type: "update", Payload: "data"}
	hub.BroadcastToRoom("room-a", msg)

	select {
	case <-client1.Send:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for room-a broadcast")
	}

	select {
	case <-client2.Send:
		t.Error("room-b client should not receive room-a message")
	case <-time.After(50 * time.Millisecond):
		// expected - no message
	}
}

func TestHub_EmptyRoomCleanup(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{},
		Send:   make(chan []byte, 256),
		RoomID: "temp-room",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if _, exists := hub.rooms["temp-room"]; exists {
		t.Error("expected empty room to be cleaned up")
	}
	hub.mu.RUnlock()
}

func TestHub_UnregisterNonExistent(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{99},
		Send:   make(chan []byte, 256),
		RoomID: "ghost",
	}

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(hub.clients))
	}
	if _, exists := hub.rooms["ghost"]; exists {
		t.Error("expected no room for unregistered client")
	}
	hub.mu.RUnlock()
}

func TestHub_BroadcastToAllFullSend(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{1},
		Send:   make(chan []byte, 1),
		RoomID: "full-test",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	client.Send <- []byte("occupy")
	msg := Message{Type: "test", Payload: "overflow"}
	hub.BroadcastToAll(msg)

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if _, exists := hub.clients[client]; exists {
		t.Error("expected client to be removed after full send buffer")
	}
	hub.mu.RUnlock()
}

func TestHub_BroadcastToRoomFullSend(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{1},
		Send:   make(chan []byte, 1),
		RoomID: "room-full",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	client.Send <- []byte("occupy")
	msg := Message{Type: "test", Payload: "overflow"}
	hub.BroadcastToRoom("room-full", msg)

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if _, exists := hub.clients[client]; exists {
		t.Error("expected client to be removed after full send buffer in BroadcastToRoom")
	}
	hub.mu.RUnlock()
}

func TestHub_BroadcastToRoomNonExistent(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	msg := Message{Type: "test", Payload: "no-room"}
	hub.BroadcastToRoom("non-existent", msg)

	hub.mu.RLock()
	if len(hub.clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()
}

func TestHub_BroadcastToAllMarshalError(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{1},
		Send:   make(chan []byte, 256),
		RoomID: "error-test",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	msg := Message{Type: "test", Payload: math.NaN()}
	hub.BroadcastToAll(msg)

	time.Sleep(50 * time.Millisecond)

	select {
	case <-client.Send:
		t.Error("client should not receive message on marshal error")
	case <-time.After(50 * time.Millisecond):
		// expected - no message sent
	}
}

func TestHub_BroadcastToRoomMarshalError(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	client := &Client{
		ID:     [16]byte{1},
		Send:   make(chan []byte, 256),
		RoomID: "error-room",
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	msg := Message{Type: "test", Payload: math.NaN()}
	hub.BroadcastToRoom("error-room", msg)

	time.Sleep(50 * time.Millisecond)

	select {
	case <-client.Send:
		t.Error("client should not receive message on marshal error")
	case <-time.After(50 * time.Millisecond):
		// expected - no message sent
	}
}

func TestHub_MultipleClientsSameRoom(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	c1 := &Client{ID: [16]byte{1}, Send: make(chan []byte, 256), RoomID: "multi"}
	c2 := &Client{ID: [16]byte{2}, Send: make(chan []byte, 256), RoomID: "multi"}
	c3 := &Client{ID: [16]byte{3}, Send: make(chan []byte, 256), RoomID: "multi"}

	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.rooms["multi"]) != 3 {
		t.Errorf("expected 3 clients in room, got %d", len(hub.rooms["multi"]))
	}
	hub.mu.RUnlock()

	msg := Message{Type: "update", Payload: "shared"}
	hub.BroadcastToRoom("multi", msg)

	for i, c := range []*Client{c1, c2, c3} {
		select {
		case <-c.Send:
		case <-time.After(200 * time.Millisecond):
			t.Errorf("client %d timeout waiting for message", i+1)
		}
	}
}

func TestHub_UnregisterCleansRoom(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	c1 := &Client{ID: [16]byte{1}, Send: make(chan []byte, 256), RoomID: "clean-room"}
	c2 := &Client{ID: [16]byte{2}, Send: make(chan []byte, 256), RoomID: "clean-room"}

	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(c1)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(hub.clients))
	}
	if len(hub.rooms["clean-room"]) != 1 {
		t.Errorf("expected 1 client in room, got %d", len(hub.rooms["clean-room"]))
	}
	hub.mu.RUnlock()

	hub.Unregister(c2)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	if _, exists := hub.rooms["clean-room"]; exists {
		t.Error("expected room to be cleaned up after last client leaves")
	}
	hub.mu.RUnlock()
}

func TestHub_BroadcastToAllMultipleClients(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	c1 := &Client{ID: [16]byte{1}, Send: make(chan []byte, 256), RoomID: "a"}
	c2 := &Client{ID: [16]byte{2}, Send: make(chan []byte, 256), RoomID: "b"}

	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(10 * time.Millisecond)

	msg := Message{Type: "global", Payload: "everyone"}
	hub.BroadcastToAll(msg)

	for i, c := range []*Client{c1, c2} {
		select {
		case received := <-c.Send:
			if len(received) == 0 {
				t.Errorf("client %d received empty message", i+1)
			}
		case <-time.After(200 * time.Millisecond):
			t.Errorf("client %d timeout", i+1)
		}
	}
}

func TestHub_BroadcastToRoomNoRoom(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	c := &Client{ID: [16]byte{1}, Send: make(chan []byte, 256), RoomID: "exists"}
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	msg := Message{Type: "test", Payload: "nope"}
	hub.BroadcastToRoom("does-not-exist", msg)

	select {
	case <-c.Send:
		t.Error("client in different room should not receive message")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_RunBroadcastChannel(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	c := &Client{ID: [16]byte{1}, Send: make(chan []byte, 256), RoomID: "chan-test"}
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	data, _ := json.Marshal(Message{Type: "chan", Payload: "via-channel"})
	hub.broadcast <- data

	select {
	case received := <-c.Send:
		if string(received) != string(data) {
			t.Errorf("expected %q, got %q", string(data), string(received))
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for broadcast via channel")
	}
}

func TestHub_RunBroadcastChannelFullClient(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	c := &Client{ID: [16]byte{1}, Send: make(chan []byte, 1), RoomID: "chan-full"}
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	c.Send <- []byte("occupy")

	data, _ := json.Marshal(Message{Type: "test", Payload: "overflow"})
	hub.broadcast <- data

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if _, exists := hub.clients[c]; exists {
		t.Error("expected client removed after full send during broadcast")
	}
	hub.mu.RUnlock()
}

func TestHub_NewHubFields(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)

	if cap(hub.broadcast) != 256 {
		t.Errorf("expected broadcast channel capacity 256, got %d", cap(hub.broadcast))
	}
	if hub.register == nil {
		t.Error("register channel not initialized")
	}
	if hub.unregister == nil {
		t.Error("unregister channel not initialized")
	}
	if hub.logger != logger {
		t.Error("logger not set correctly")
	}
}
