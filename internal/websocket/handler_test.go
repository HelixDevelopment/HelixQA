package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewWSHandler(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	h := NewWSHandler(hub, logger)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.hub != hub {
		t.Error("hub not set correctly")
	}
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestHandleWebSocket_UpgradeAndRegister(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=test-room"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(hub.clients))
	}
	if len(hub.rooms["test-room"]) != 1 {
		t.Errorf("expected 1 client in test-room, got %d", len(hub.rooms["test-room"]))
	}
	hub.mu.RUnlock()
}

func TestHandleWebSocket_DefaultRoom(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 1 {
		t.Errorf("expected 1 client, got %d", len(hub.clients))
	}
	if _, ok := hub.rooms["default"]; !ok {
		t.Error("expected client in default room")
	}
	hub.mu.RUnlock()
}

func TestHandleWebSocket_WritePump(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=write-test"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	var client *Client
	for c := range hub.clients {
		client = c
		break
	}
	hub.mu.RUnlock()

	if client == nil {
		t.Fatal("no client found in hub")
	}

	testData := []byte(`{"type":"test","payload":"hello"}`)
	client.Send <- testData

	_, message, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	if string(message) != string(testData) {
		t.Errorf("expected %q, got %q", string(testData), string(message))
	}
}

func TestHandleWebSocket_ReadPump(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=read-test"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	initialCount := len(hub.clients)
	hub.mu.RUnlock()

	if initialCount != 1 {
		t.Fatalf("expected 1 client, got %d", initialCount)
	}

	ws.Close()
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	afterCount := len(hub.clients)
	hub.mu.RUnlock()

	if afterCount != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", afterCount)
	}
}

func TestHandleWebSocket_MultipleClientsSameRoom(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=shared"

	ws1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect ws1: %v", err)
	}
	defer ws1.Close()

	ws2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect ws2: %v", err)
	}
	defer ws2.Close()

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 2 {
		t.Errorf("expected 2 clients, got %d", len(hub.clients))
	}
	if len(hub.rooms["shared"]) != 2 {
		t.Errorf("expected 2 clients in shared room, got %d", len(hub.rooms["shared"]))
	}
	hub.mu.RUnlock()

	msg := Message{Type: "broadcast", Payload: "to-all"}
	hub.BroadcastToRoom("shared", msg)

	for _, ws := range []*websocket.Conn{ws1, ws2} {
		ws.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _, err := ws.ReadMessage()
		if err != nil {
			t.Errorf("client failed to receive room broadcast: %v", err)
		}
	}
}

func TestHandleWebSocket_BroadcastBetweenRooms(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL1 := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=room-1"
	ws1, _, err := websocket.DefaultDialer.Dial(wsURL1, nil)
	if err != nil {
		t.Fatalf("failed to connect ws1: %v", err)
	}
	defer ws1.Close()

	wsURL2 := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=room-2"
	ws2, _, err := websocket.DefaultDialer.Dial(wsURL2, nil)
	if err != nil {
		t.Fatalf("failed to connect ws2: %v", err)
	}
	defer ws2.Close()

	time.Sleep(50 * time.Millisecond)

	msg := Message{Type: "update", Payload: "data"}
	hub.BroadcastToRoom("room-1", msg)

	ws1.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = ws1.ReadMessage()
	if err != nil {
		t.Errorf("room-1 client should receive message: %v", err)
	}

	ws2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err = ws2.ReadMessage()
	if err == nil {
		t.Error("room-2 client should NOT receive room-1 message")
	}
}

func TestHandleWebSocket_BroadcastToAllViaHandler(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=broadcast-all"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer ws.Close()

	time.Sleep(50 * time.Millisecond)

	msg := Message{Type: "ping", Payload: map[string]string{"status": "ok"}}
	hub.BroadcastToAll(msg)

	ws.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, received, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read broadcast message: %v", err)
	}
	if len(received) == 0 {
		t.Error("expected non-empty broadcast message")
	}
}

func TestHandleWebSocket_WritePumpError(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=write-error"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	ws.Close()
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 0 {
		t.Errorf("expected 0 clients after write error, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()
}

func TestHandleWebSocket_ReadPumpError(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=read-error"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()

	ws.Close()
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 0 {
		t.Errorf("expected 0 clients after read error, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()
}

func TestHandleWebSocket_WritePumpSendError(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=write-pump-error"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()

	ws.Close()
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	afterCount := len(hub.clients)
	hub.mu.RUnlock()

	if clientCount != 1 {
		t.Errorf("expected 1 client before close, got %d", clientCount)
	}
	if afterCount != 0 {
		t.Errorf("expected 0 clients after close, got %d", afterCount)
	}
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 0 {
		t.Errorf("expected 0 clients after write pump error, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()
}

func TestHandleWebSocket_UpgradeFailure(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()

	h := NewWSHandler(hub, logger)

	router := gin.New()
	router.GET("/ws", h.HandleWebSocket)

	server := httptest.NewServer(router)
	defer server.Close()

	resp, err := http.Get(server.URL + "/ws")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusSwitchingProtocols {
		t.Error("expected non-upgrade response for plain HTTP request")
	}

	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	if len(hub.clients) != 0 {
		t.Errorf("expected 0 clients after failed upgrade, got %d", len(hub.clients))
	}
	hub.mu.RUnlock()
}
