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
	"go.mongodb.org/mongo-driver/v2/bson"
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

	switch wsMsg.Type {
	case "room.enter":
		h.handleEnterRoom(client, payload)
	case "room.exit":
		h.handleExitRoom(client, payload)
	case "room.join":
		h.handleJoinRoom(client, payload)
	case "room.leave":
		h.handleLeaveRoom(client, payload)
	case "message.send":
		h.handleSendMessage(client, payload)
	case "message.edit":
		h.handleEditMessage(client, payload)
	case "message.delete":
		h.handleDeleteMessage(client, payload)
	case "message.typing":
		h.handleTyping(client, payload)
	default:
		h.sendError(client, "Unknown action")
	}
}

func (h *ChatHandler) handleEnterRoom(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
		return
	}

	err := h.ChatService.EnterRoom(client, roomID)
	if err != nil {
		h.sendError(client, "Failed to enter room. "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "room.entered",
		Success: true,
		Message: "Entered room successfully",
		Data: map[string]any{
			"roomId": roomID,
		},
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleExitRoom(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
		return
	}

	h.ChatService.ExitRoom(client, roomID)
}

func (h *ChatHandler) handleJoinRoom(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
		return
	}

	err := h.ChatService.JoinRoom(client, roomID)
	if err != nil {
		h.sendError(client, "Failed to join room. "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "room.joined",
		Success: true,
		Message: "Successfully joined room",
		Data: map[string]any{
			"roomId": roomID,
		},
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleLeaveRoom(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
		return
	}

	h.ChatService.LeaveRoom(client, roomID)

	response := models.WSResponse{
		Type:    "room.left",
		Success: true,
		Message: "Successfully left room",
		Data: map[string]any{
			"roomId": roomID,
		},
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleSendMessage(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
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

	fileUrl := ""
	if fileUrlStr, exists := payload["fileUrl"].(string); exists {
		fileUrl = fileUrlStr
	}

	var replyTo *models.MessagePreview = nil
	if replyToMap, exists := payload["replyTo"].(map[string]any); exists {
		idStr, ok := replyToMap["id"].(string)
		if !ok {
			h.sendError(client, "Missing reply_to id")
			return
		}
		replyToObjectID, err := bson.ObjectIDFromHex(idStr)
		if err != nil {
			h.sendError(client, "Invalid reply_to id")
			return
		}

		content, ok := replyToMap["content"].(string)
		if !ok {
			h.sendError(client, "Missing reply_to content")
			return
		}

		authorMap, ok := replyToMap["author"].(map[string]any)
		if !ok {
			h.sendError(client, "Missing reply_to author")
			return
		}

		authorIDStr, ok := authorMap["id"].(string)
		if !ok {
			h.sendError(client, "Missing reply_to author id")
			return
		}
		authorID, err := bson.ObjectIDFromHex(authorIDStr)
		if err != nil {
			h.sendError(client, "Invalid reply_to author id")
			return
		}

		authorUsername, ok := authorMap["username"].(string)
		if !ok {
			h.sendError(client, "Missing reply_to author username")
			return
		}

		authorPicture, _ := authorMap["picture"].(string)

		createdAtStr, ok := replyToMap["createdAt"].(string)
		if !ok {
			h.sendError(client, "Missing reply_to createdAt")
			return
		}
		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			h.sendError(client, "Invalid reply_to createdAt format")
			return
		}

		replyTo = &models.MessagePreview{
			ID:      replyToObjectID,
			Content: content,
			Author: models.UserSummary{
				ID:       authorID,
				Username: authorUsername,
				Picture:  authorPicture,
			},
			CreatedAt: createdAt,
		}
	}

	err := h.ChatService.SendMessage(client, roomID, content, fileUrl, messageType, replyTo)
	if err != nil {
		h.sendError(client, "Failed to send message: "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "message.sent",
		Success: true,
		Message: "Message sent successfully",
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleEditMessage(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
		return
	}

	currentRoom := h.ChatService.GetUserCurrentRoom(client)
	if currentRoom != roomID {
		h.sendError(client, "You are not currently in this room")
		return
	}

	messageId, ok := payload["messageId"].(string)
	if !ok {
		h.sendError(client, "Missing message to edit")
		return
	}

	content, ok := payload["content"].(string)
	if !ok {
		h.sendError(client, "Missing content")
		return
	}

	err := h.ChatService.EditMessage(client, messageId, content)
	if err != nil {
		h.sendError(client, "Failed to edit message: "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "message.editted",
		Success: true,
		Message: "Message editted successfully",
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleDeleteMessage(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
		return
	}

	currentRoom := h.ChatService.GetUserCurrentRoom(client)
	if currentRoom != roomID {
		h.sendError(client, "You are not currently in this room")
		return
	}

	messageId, ok := payload["messageId"].(string)
	if !ok {
		h.sendError(client, "Missing message to delete")
		return
	}

	err := h.ChatService.DeleteMessage(client, messageId)
	if err != nil {
		h.sendError(client, "Failed to delete message: "+err.Error())
		return
	}

	response := models.WSResponse{
		Type:    "message.deleted",
		Success: true,
		Message: "Message deleted successfully",
	}
	h.sendToClient(client, response)
}

func (h *ChatHandler) handleTyping(client *services.Client, payload map[string]any) {
	roomID, ok := payload["roomId"].(string)
	if !ok {
		h.sendError(client, "Missing roomId")
		return
	}

	currentRoom := h.ChatService.GetUserCurrentRoom(client)
	if currentRoom != roomID {
		h.sendError(client, "You are not currently in this room")
		return
	}

	response := models.WSResponse{
		Type:    "user.typing",
		Success: true,
		Data: map[string]any{
			"roomId": roomID,
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
				log.Printf("Recovered in broadcast user typing: %v\n%s", r, debug.Stack())
			}
		}()
		select {
		case h.ChatService.Hub.Broadcast <- data:
		default:
			log.Printf("Broadcast channel full, skipping user typing message")
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
