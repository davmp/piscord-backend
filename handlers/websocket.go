package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"piscord-backend/config"
	"piscord-backend/models"
	"piscord-backend/services"
	"piscord-backend/utils"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatHandler struct {
	ChatService *services.ChatService
	Config      *config.Config
}

func NewChatHandler(chatService *services.ChatService, config *config.Config) *ChatHandler {
	return &ChatHandler{
		ChatService: chatService,
		Config:      config,
	}
}

func (h *ChatHandler) HandleWebsocket(c *gin.Context) {
	token := c.Query("at")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token query parameter required"})
		return
	}

	claims, err := utils.ValidateJWT(token, h.Config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
		return
	}

	userID := claims.Subject
	username := claims.Username
	picture := claims.Picture

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	client := &services.Client{
		ID:       uuid.New().String(),
		UserID:   userID,
		Username: username,
		Picture:  picture,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Rooms:    make(map[string]bool),
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered in Register client: %v\n%s", r, debug.Stack())
			}
		}()
		h.ChatService.Hub.Register <- client
	}()

	go h.writePump(client)
	go h.readPump(client)
}

func (h *ChatHandler) readPump(client *services.Client) {
	defer func() {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered in Unregister client: %v\n%s", r, debug.Stack())
				}
			}()
			h.ChatService.Hub.Unregister <- client
		}()
		client.Conn.Close()
	}()

	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		h.handleMessage(client, message)
	}
}

func (h *ChatHandler) writePump(client *services.Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *ChatHandler) handleMessage(client *services.Client, message []byte) {
	var wsMsg models.WSMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		h.sendError(client, "Invalid message format")
		return
	}

	payload, ok := wsMsg.Payload.(map[string]any)
	if !ok {
		h.sendError(client, "Invalid payload format")
		return
	}

	action, ok := payload["action"].(string)
	if !ok {
		h.sendError(client, "Missing action")
		return
	}

	switch action {
	case "enter_room":
		h.handleEnterRoom(client, payload)
	case "join_room":
		h.handleJoinRoom(client, payload)
	case "leave_room":
		h.handleLeaveRoom(client, payload)
	case "send_message":
		h.handleSendMessage(client, payload)
	case "notify_message":
		// This action can be handled if needed
	case "fetch_messages":
		// This action can be handled if needed
	case "typing":
		h.handleTyping(client, payload)
	default:
		h.sendError(client, "Unknown action")
	}
}

func (h *ChatHandler) handleEnterRoom(client *services.Client, payload map[string]any) {
	roomID, ok := payload["room_id"].(string)
	if !ok {
		h.sendError(client, "Missing room_id")
		return
	}

	err := h.ChatService.EnterRoom(client, roomID)
	if err != nil {
		h.sendError(client, "Failed to enter room: "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "room_entered",
		Success: true,
		Message: "Entered room successfully",
		Data: map[string]any{
			"room_id": roomID,
		},
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleJoinRoom(client *services.Client, payload map[string]any) {
	roomID, ok := payload["room_id"].(string)
	if !ok {
		h.sendError(client, "Missing room_id")
		return
	}

	err := h.ChatService.JoinRoom(client, roomID)
	if err != nil {
		h.sendError(client, "Failed to join room: "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "room_joined",
		Success: true,
		Message: "Successfully joined room",
		Data: map[string]any{
			"room_id": roomID,
		},
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleLeaveRoom(client *services.Client, payload map[string]any) {
	roomID, ok := payload["room_id"].(string)
	if !ok {
		h.sendError(client, "Missing room_id")
		return
	}

	h.ChatService.LeaveRoom(client, roomID)

	response := models.WSResponse{
		Type:    "room_left",
		Success: true,
		Message: "Successfully left room",
		Data: map[string]any{
			"room_id": roomID,
		},
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleSendMessage(client *services.Client, payload map[string]any) {
	roomID, ok := payload["room_id"].(string)
	if !ok {
		h.sendError(client, "Missing room_id")
		return
	}

	currentRoom := h.ChatService.GetUserCurrentRoom(client)
	if currentRoom != roomID {
		h.sendError(client, "You are not currently in this room")
		return
	}

	content, ok := payload["content"].(string)
	if !ok {
		h.sendError(client, "Missing content")
		return
	}

	messageType := "text"
	if msgType, exists := payload["type"].(string); exists {
		messageType = msgType
	}

	err := h.ChatService.SendMessage(client, roomID, content, messageType)
	if err != nil {
		h.sendError(client, "Failed to send message: "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "message_sent",
		Success: true,
		Message: "Message sent successfully",
		Data: map[string]any{
			"is_own_message": true,
		},
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleTyping(client *services.Client, payload map[string]any) {
	roomID, ok := payload["room_id"].(string)
	if !ok {
		h.sendError(client, "Missing room_id")
		return
	}

	currentRoom := h.ChatService.GetUserCurrentRoom(client)
	if currentRoom != roomID {
		h.sendError(client, "You are not currently in this room")
		return
	}

	response := models.WSResponse{
		Type:    "user_typing",
		Success: true,
		Data: map[string]any{
			"room_id": roomID,
			"user": map[string]any{
				"id":       client.UserID,
				"username": client.Username,
				"picture":  client.Picture,
			},
		},
	}

	data, _ := json.Marshal(response)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered in broadcast user_typing: %v\n%s", r, debug.Stack())
			}
		}()
		select {
		case h.ChatService.Hub.Broadcast <- data:
		default:
			log.Printf("Broadcast channel full, skipping user_typing message")
		}
	}()
}

func (h *ChatHandler) sendError(client *services.Client, message string) {
	response := models.WSResponse{
		Type:    "error",
		Success: false,
		Message: message,
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) sendToClient(client *services.Client, response models.WSResponse) {
	data, _ := json.Marshal(response)
	select {
	case client.Send <- data:
	default:
		log.Printf("Send buffer full, closing connection for client %s", client.UserID)
		// Close client connection to trigger cleanup
		go func() {
			client.Conn.Close()
		}()
	}
}
