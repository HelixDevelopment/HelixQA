package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSHandler struct {
	hub    *Hub
	logger *zap.Logger
}

func NewWSHandler(hub *Hub, logger *zap.Logger) *WSHandler {
	return &WSHandler{hub: hub, logger: logger}
}

// GET /ws
func (h *WSHandler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket", zap.Error(err))
		return
	}

	roomID := c.Query("room")
	if roomID == "" {
		roomID = "default"
	}

	client := &Client{
		ID:     uuid.New(),
		Conn:   conn,
		Send:   make(chan []byte, 256),
		RoomID: roomID,
	}

	h.hub.Register(client)
	go h.writePump(client)
	go h.readPump(client)
}

func (h *WSHandler) readPump(client *Client) {
	defer func() {
		h.hub.Unregister(client)
		client.Conn.Close()
	}()
	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (h *WSHandler) writePump(client *Client) {
	defer client.Conn.Close()
	for message := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
