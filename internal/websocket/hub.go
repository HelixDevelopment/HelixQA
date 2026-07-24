package websocket

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Client struct {
	ID     uuid.UUID
	Conn   *websocket.Conn
	Send   chan []byte
	RoomID string
}

type Hub struct {
	clients    map[*Client]bool
	rooms      map[string]map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	logger     *zap.Logger
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if _, ok := h.rooms[client.RoomID]; !ok {
				h.rooms[client.RoomID] = make(map[*Client]bool)
			}
			h.rooms[client.RoomID][client] = true
			h.mu.Unlock()
			h.logger.Info("client connected", zap.String("room", client.RoomID))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if room, ok := h.rooms[client.RoomID]; ok {
					delete(room, client)
					if len(room) == 0 {
						delete(h.rooms, client.RoomID)
					}
				}
				close(client.Send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastToRoom(roomID string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal message", zap.Error(err))
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if room, ok := h.rooms[roomID]; ok {
		for client := range room {
			select {
			case client.Send <- data:
			default:
				close(client.Send)
				delete(h.clients, client)
			}
		}
	}
}

func (h *Hub) BroadcastToAll(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal message", zap.Error(err))
		return
	}
	h.broadcast <- data
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}
