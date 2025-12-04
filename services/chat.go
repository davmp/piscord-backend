package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"piscord-backend/models"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Client struct {
	ID       string
	UserID   string
	Username string
	Picture  string
	Conn     *websocket.Conn
	Send     chan []byte
	Rooms    map[string]bool
}

type Hub struct {
	Clients    map[string]*Client
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	Rooms      map[string]map[string]*Client
	mutex      sync.RWMutex
}

type ChatService struct {
	Hub                 *Hub
	RoomService         *RoomService
	MongoService        *MongoService
	NotificationService *NotificationService
	AuthService         *AuthService
	RedisService        *RedisService
}

func NewChatService(roomService *RoomService, mongoService *MongoService, notificationService *NotificationService, authService *AuthService, redisService *RedisService) *ChatService {
	hub := &Hub{
		Clients:    make(map[string]*Client),
		Broadcast:  make(chan []byte, 256),
		Register:   make(chan *Client, 256),
		Unregister: make(chan *Client, 256),
		Rooms:      make(map[string]map[string]*Client),
	}

	chatService := &ChatService{
		Hub:                 hub,
		RoomService:         roomService,
		MongoService:        mongoService,
		NotificationService: notificationService,
		AuthService:         authService,
		RedisService:        redisService,
	}

	go chatService.Run()
	return chatService
}

func (cs *ChatService) Run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ChatService.Run recovered from panic: %v", r)
			debug.PrintStack()
		}
	}()
	for {
		select {
		case client := <-cs.Hub.Register:
			cs.registerClient(client)

		case client := <-cs.Hub.Unregister:
			cs.unregisterClient(client)

		case message := <-cs.Hub.Broadcast:
			cs.broadcastMessage(message)
		}
	}
}

func (cs *ChatService) EnterRoom(client *Client, roomID string) error {
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}

	userObjectID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return err
	}

	room, err := cs.RoomService.GetRoomByID(roomObjectID)
	if err != nil {
		return err
	}

	if !slices.Contains(room.Members, userObjectID) {
		return errors.New("user is not a member of this room")
	}

	for currentRoomID := range client.Rooms {
		cs.exitRoomInternal(client, currentRoomID)
	}

	cs.Hub.mutex.Lock()
	defer cs.Hub.mutex.Unlock()

	if cs.Hub.Rooms[roomID] == nil {
		cs.Hub.Rooms[roomID] = make(map[string]*Client)
	}
	cs.Hub.Rooms[roomID][client.UserID] = client
	client.Rooms[roomID] = true

	go func() {
		if err := cs.RedisService.AddUserToRoom(client.UserID, roomID); err != nil {
			log.Printf("Failed to add user to room in Redis: %v", err)
		}
	}()

	return nil
}

func (cs *ChatService) JoinRoom(client *Client, roomID string) error {
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}

	room, err := cs.RoomService.GetRoomByID(roomObjectID)
	if err != nil {
		return err
	}

	for currentRoomID := range client.Rooms {
		cs.exitRoomInternal(client, currentRoomID)
	}

	cs.Hub.mutex.Lock()

	if cs.Hub.Rooms[roomID] == nil {
		cs.Hub.Rooms[roomID] = make(map[string]*Client)
	}
	cs.Hub.Rooms[roomID][client.UserID] = client
	client.Rooms[roomID] = true

	go func() {
		if err := cs.RedisService.AddUserToRoom(client.UserID, roomID); err != nil {
			log.Printf("Failed to add user to room in Redis: %v", err)
		}
	}()

	cs.Hub.mutex.Unlock()

	message := models.WSResponse{
		Type:    "user.joined",
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

	cs.broadcastToRoomExcept(roomID, message, client)
	cs.notifyUserJoined(room, client)

	return nil
}

func (cs *ChatService) ExitRoom(client *Client, roomID string) {
	cs.exitRoomInternal(client, roomID)
}

func (cs *ChatService) LeaveRoom(client *Client, roomID string) {
	cs.leaveRoomInternal(client, roomID)
}

func (cs *ChatService) GetUserCurrentRoom(client *Client) string {
	cs.Hub.mutex.RLock()
	defer cs.Hub.mutex.RUnlock()

	for roomID := range client.Rooms {
		return roomID
	}
	return ""
}

func (cs *ChatService) GetUserStatus(userID string) bool {
	client := cs.getClientSnapshot(userID)
	return client != nil
}

func (cs *ChatService) IsUserInRoom(userID, roomID string) bool {
	cs.Hub.mutex.RLock()
	if room, ok := cs.Hub.Rooms[roomID]; ok {
		if _, ok := room[userID]; ok {
			cs.Hub.mutex.RUnlock()
			return true
		}
	}
	cs.Hub.mutex.RUnlock()

	inRoom, err := cs.RedisService.IsUserInRoom(userID, roomID)
	if err != nil {
		log.Printf("Error checking Redis for room membership: %v", err)
		return false
	}
	return inRoom
}

func (cs *ChatService) SendMessage(client *Client, roomID, content, fileUrl, messageType string, replyTo *models.MessagePreview) error {
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}

	room, err := cs.RoomService.GetRoomByID(roomObjectID)
	if err != nil {
		return err
	}

	userObjectID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return err
	}

	if !slices.Contains(room.Members, userObjectID) {
		return errors.New("user is not a member of this room")
	}

	message := models.MessageSend{
		RoomID: roomObjectID,
		Author: models.UserSummary{
			ID:       userObjectID,
			Username: client.Username,
			Picture:  client.Picture,
		},
		Content: content,
		FileURL: fileUrl,
		ReplyTo: replyTo,
		SentAt:  time.Now(),
	}

	cs.RedisService.Publish("chat", "message.send", message)

	cs.notifyMessage(message)
	cs.notifyMessageGroupUsers(room, message)

	cs.broadcastToRoomExcept(
		roomID,
		models.WSResponse{
			Type:    "new.message",
			Success: true,
			Data:    message,
		},
		client,
	)

	cs.sendToClient(client, models.WSResponse{
		Type:    "new.message",
		Success: true,
		Data:    message,
	})

	return nil
}

func (cs *ChatService) EditMessage(client *Client, messageID, content string) error {
	userObjectID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return err
	}

	messageObjectID, err := bson.ObjectIDFromHex(messageID)
	if err != nil {
		return err
	}

	return cs.RedisService.Publish("chat", "message.update", map[string]any{
		"id":        messageObjectID,
		"userId":    userObjectID,
		"content":   content,
		"updatedAt": time.Now(),
	})
}

func (cs *ChatService) UpdateClient(userID, username, picture string) error {
	client := cs.getClientSnapshot(userID)

	if client == nil {
		return errors.New("invalid client")
	}

	cs.Hub.mutex.Lock()
	if username != "" {
		client.Username = username
	}
	if picture != "" {
		client.Picture = picture
	}
	cs.Hub.mutex.Unlock()

	return nil
}

func (cs *ChatService) registerClient(client *Client) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("registerClient panic: %v", r)
			debug.PrintStack()
		}
	}()

	cs.Hub.mutex.Lock()
	cs.Hub.Clients[client.UserID] = client
	client.Rooms = make(map[string]bool)
	cs.Hub.mutex.Unlock()

	cs.sendToClient(client, models.WSResponse{
		Type:    "connection",
		Success: true,
		Message: "Connected to chat server",
		Data: map[string]any{
			"user": map[string]any{
				"id":       client.UserID,
				"username": client.Username,
				"picture":  client.Picture,
			},
		},
	})
}

func (cs *ChatService) unregisterClient(client *Client) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("unregisterClient panic: %v", r)
			debug.PrintStack()
		}
	}()

	for roomID := range client.Rooms {
		cs.exitRoomInternal(client, roomID)
	}

	cs.Hub.mutex.Lock()
	defer cs.Hub.mutex.Unlock()

	delete(cs.Hub.Clients, client.UserID)

	close(client.Send)
	if client.Conn != nil {
		client.Conn.Close()
	}
}

func (cs *ChatService) exitRoomInternal(client *Client, roomID string) {
	cs.Hub.mutex.Lock()
	defer cs.Hub.mutex.Unlock()

	if room, exists := cs.Hub.Rooms[roomID]; exists {
		delete(room, client.UserID)
		if len(room) == 0 {
			delete(cs.Hub.Rooms, roomID)
		}
	}

	delete(client.Rooms, roomID)

	go func() {
		if err := cs.RedisService.RemoveUserFromRoom(client.UserID, roomID); err != nil {
			log.Printf("Failed to remove user from room in Redis: %v", err)
		}
	}()
}

func (cs *ChatService) leaveRoomInternal(client *Client, roomID string) {
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return
	}

	room, err := cs.RoomService.GetRoomByID(roomObjectID)
	if err != nil {
		return
	}

	cs.Hub.mutex.Lock()

	if room, exists := cs.Hub.Rooms[roomID]; exists {
		delete(room, client.UserID)
		if len(room) == 0 {
			delete(cs.Hub.Rooms, roomID)
		}
	}

	cs.Hub.mutex.Unlock()

	delete(client.Rooms, roomID)

	go func() {
		if err := cs.RedisService.RemoveUserFromRoom(client.UserID, roomID); err != nil {
			log.Printf("Failed to remove user from room in Redis: %v", err)
		}
	}()

	cs.notifyUserLeft(room, client)

	notification := models.WSResponse{
		Type:    "user.left",
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

	if len(cs.Hub.Rooms[roomID]) > 0 {
		cs.broadcastToRoom(roomID, cs.marshalResponse(notification))
	}
}

func (cs *ChatService) notifyUserLeft(room *models.Room, client *Client) error {
	cs.Hub.mutex.RLock()
	roomMap := cs.Hub.Rooms[room.ID.Hex()]
	cs.Hub.mutex.RUnlock()

	usersOutOfRoom := make([]bson.ObjectID, 0, len(room.Members))
	for _, memberID := range room.Members {
		if roomMap == nil {
			if room.Type != "public" || (room.Type == "public" && slices.Contains(room.Admins, memberID)) {
				usersOutOfRoom = append(usersOutOfRoom, memberID)
			}
			continue
		}

		if _, ok := roomMap[memberID.Hex()]; !ok {
			if room.Type != "public" || (room.Type == "public" && slices.Contains(room.Admins, memberID)) {
				usersOutOfRoom = append(usersOutOfRoom, memberID)
			}
		}
	}

	for _, userID := range usersOutOfRoom {
		if room.Type != "public" || (room.Type == "public" && slices.Contains(room.Admins, userID)) {
			notification := models.Notification{
				ID:        bson.NewObjectID(),
				UserID:    userID,
				Title:     fmt.Sprintf("%s saiu da sala", client.Username),
				Body:      fmt.Sprintf("O usuário %s saiu da sala %s", client.Username, room.Name),
				Picture:   client.Picture,
				Type:      models.NotificationTypeUserLeft,
				IsRead:    false,
				CreatedAt: time.Now(),
			}

			if err := cs.NotificationService.CreateNotification(&notification); err != nil {
				return err
			}

			memberHex := userID.Hex()
			client := cs.getClientSnapshot(memberHex)

			if client == nil {
				continue
			}

			cs.sendToClient(client, models.WSResponse{
				Type:    "notification",
				Success: true,
				Data: map[string]any{
					"id":   notification.ID.Hex(),
					"type": notification.Type,
				},
			})
		}
	}

	return nil
}

func (cs *ChatService) notifyUserJoined(room *models.Room, client *Client) error {
	cs.Hub.mutex.RLock()
	roomMap := cs.Hub.Rooms[room.ID.Hex()]
	cs.Hub.mutex.RUnlock()

	usersOutOfRoom := make([]bson.ObjectID, 0, len(room.Members))
	for _, memberID := range room.Members {
		if roomMap == nil {
			if room.Type != "public" || (room.Type == "public" && slices.Contains(room.Admins, memberID)) {
				usersOutOfRoom = append(usersOutOfRoom, memberID)
			}
			continue
		}

		if _, ok := roomMap[memberID.Hex()]; !ok {
			if room.Type != "public" || (room.Type == "public" && slices.Contains(room.Admins, memberID)) {
				usersOutOfRoom = append(usersOutOfRoom, memberID)
			}
		}
	}

	for _, userID := range usersOutOfRoom {
		if room.Type != "public" || (room.Type == "public" && slices.Contains(room.Admins, userID)) {
			notification := models.Notification{
				ID:        bson.NewObjectID(),
				UserID:    userID,
				Title:     fmt.Sprintf("%s entrou na sala", client.Username),
				Body:      fmt.Sprintf("Um novo usuário %s entrou em %s", client.Username, room.Name),
				Picture:   client.Picture,
				Type:      "user.joined",
				IsRead:    false,
				CreatedAt: time.Now(),
			}

			if err := cs.NotificationService.CreateNotification(&notification); err != nil {
				return err
			}

			memberHex := userID.Hex()
			client := cs.getClientSnapshot(memberHex)

			if client == nil {
				continue
			}

			cs.sendToClient(client, models.WSResponse{
				Type:    "notification",
				Success: true,
				Data: map[string]any{
					"id":   notification.ID.Hex(),
					"type": notification.Type,
				},
			})
		}
	}

	return nil
}

func (cs *ChatService) notifyMessageGroupUsers(room *models.Room, message models.MessageSend) error {
	if len(room.Members) == 0 {
		return nil
	}

	cs.Hub.mutex.RLock()
	roomMap := cs.Hub.Rooms[room.ID.Hex()]
	cs.Hub.mutex.RUnlock()

	usersToNotify := make(map[bson.ObjectID]struct{}, len(room.Members))

	for _, memberID := range room.Members {
		isOutOfRoom := (roomMap != nil && roomMap[memberID.Hex()] == nil) || roomMap == nil
		if isOutOfRoom && (room.Type != "public" || slices.Contains(room.Admins, memberID)) {
			usersToNotify[memberID] = struct{}{}
		}
	}

	if message.ReplyTo != nil {
		var repliedMsg models.Message
		err := cs.MongoService.GetCollection("messages").FindOne(context.Background(), bson.M{"_id": message.ReplyTo}).Decode(&repliedMsg)
		if err == nil && repliedMsg.Author.ID != message.Author.ID {
			usersToNotify[repliedMsg.Author.ID] = struct{}{}
		}
	}

	var title string
	var body string
	if room.Type == "direct" {
		title = fmt.Sprintf("Nova mensagem de %s", message.Author.Username)
		body = strings.ReplaceAll(message.Content, "\n", " ")
	} else {
		title = fmt.Sprintf("Nova mensagem em %s", room.Name)
		body = fmt.Sprintf("%s: %s", message.Author.Username, strings.ReplaceAll(message.Content, "\n", " "))
	}

	for userID := range usersToNotify {
		notification := models.Notification{
			ID:        bson.NewObjectID(),
			UserID:    userID,
			Title:     title,
			Body:      body,
			Type:      models.NotificationTypeNewMessage,
			Link:      fmt.Sprintf("/chat/%s", room.ID.Hex()),
			Picture:   message.Author.Picture,
			IsRead:    false,
			CreatedAt: time.Now(),
		}

		if err := cs.NotificationService.CreateNotification(&notification); err != nil {
			log.Printf("Failed to create notification for user %s: %v", userID.Hex(), err)
			continue
		}

		if client := cs.getClientSnapshot(userID.Hex()); client != nil {
			cs.sendToClient(client, models.WSResponse{
				Type:    "notification",
				Success: true,
				Data: map[string]any{
					"id":   notification.ID.Hex(),
					"type": notification.Type,
				},
			})
		}
	}

	return nil
}

func (cs *ChatService) notifyMessage(message models.MessageSend) error {
	room, err := cs.RoomService.GetRoomByID(message.RoomID)
	if err != nil {
		return err
	}

	if len(room.Members) == 0 {
		return nil
	}

	for _, userID := range room.Members {
		memberHex := userID.Hex()
		client := cs.getClientSnapshot(memberHex)

		if client == nil {
			continue
		}

		cs.sendToClient(client, models.WSResponse{
			Type:    "message.notification",
			Success: true,
			Data:    message,
		})
	}

	return nil
}

func (cs *ChatService) broadcastMessage(message []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("broadcastMessage panic: %v", r)
			debug.PrintStack()
		}
	}()

	var wsMsg models.WSMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		log.Printf("Error unmarshaling broadcast message: %v", err)
		return
	}

	if payload, ok := wsMsg.Payload.(map[string]any); ok {
		if roomID, exists := payload["roomId"].(string); exists {
			cs.broadcastToRoom(roomID, message)
			return
		}
	}

	cs.Hub.mutex.RLock()
	clients := make([]*Client, 0, len(cs.Hub.Clients))
	for _, client := range cs.Hub.Clients {
		clients = append(clients, client)
	}
	cs.Hub.mutex.RUnlock()

	var toRemove []*Client
	for _, client := range clients {
		select {
		case client.Send <- message:
			// sent successfully
		default:
			toRemove = append(toRemove, client)
		}
	}

	for _, c := range toRemove {
		select {
		case cs.Hub.Unregister <- c:
		default:
			log.Printf("Failed to unregister client %s", c.UserID)
		}
	}
}

func (cs *ChatService) broadcastToRoom(roomID string, message []byte) {
	cs.Hub.mutex.RLock()
	roomMap := cs.Hub.Rooms[roomID]
	clients := make([]*Client, 0, len(roomMap))
	for _, client := range roomMap {
		clients = append(clients, client)
	}
	cs.Hub.mutex.RUnlock()

	var toRemove []*Client
	for _, client := range clients {
		select {
		case client.Send <- message:
		default:
			toRemove = append(toRemove, client)
		}
	}

	for _, c := range toRemove {
		select {
		case cs.Hub.Unregister <- c:
		default:
			log.Printf("Failed to unregister client %s from room %s", c.UserID, roomID)
		}
	}
}

func (cs *ChatService) broadcastToRoomExcept(roomID string, response models.WSResponse, except *Client) {
	data := cs.marshalResponse(response)

	cs.Hub.mutex.RLock()
	roomMap := cs.Hub.Rooms[roomID]
	clients := make([]*Client, 0, len(roomMap))
	for _, client := range roomMap {
		clients = append(clients, client)
	}
	cs.Hub.mutex.RUnlock()

	var toRemove []*Client
	for _, client := range clients {
		if client == except {
			continue
		}
		select {
		case client.Send <- data:
		default:
			toRemove = append(toRemove, client)
		}
	}

	for _, c := range toRemove {
		select {
		case cs.Hub.Unregister <- c:
		default:
			log.Printf("Failed to unregister client %s from room %s", c.UserID, roomID)
		}
	}
}

func (cs *ChatService) sendToClient(client *Client, response models.WSResponse) {
	data := cs.marshalResponse(response)
	select {
	case client.Send <- data:
	default:
		go func(cl *Client) {
			cs.Hub.Unregister <- cl
		}(client)
	}
}

func (cs *ChatService) getClientSnapshot(userHex string) *Client {
	cs.Hub.mutex.RLock()
	defer cs.Hub.mutex.RUnlock()
	return cs.Hub.Clients[userHex]
}

func (cs *ChatService) marshalResponse(response models.WSResponse) []byte {
	data, _ := json.Marshal(response)
	return data
}
