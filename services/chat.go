package services

import (
	"context"
	"encoding/json"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"piscord-backend/models"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	Hub          *Hub
	MongoService *MongoService
}

func NewChatService(mongoService *MongoService) *ChatService {
	hub := &Hub{
		Clients:    make(map[string]*Client),
		Broadcast:  make(chan []byte, 256),
		Register:   make(chan *Client, 256),
		Unregister: make(chan *Client, 256),
		Rooms:      make(map[string]map[string]*Client),
	}

	chatService := &ChatService{
		Hub:          hub,
		MongoService: mongoService,
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
			go cs.registerClient(client)

		case client := <-cs.Hub.Unregister:
			go cs.unregisterClient(client)

		case message := <-cs.Hub.Broadcast:
			go cs.broadcastMessage(message)
		}
	}
}

func (cs *ChatService) EnterRoom(client *Client, roomID string) error {
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}

	userObjectID, err := primitive.ObjectIDFromHex(client.UserID)
	if err != nil {
		return err
	}

	roomsCollection := cs.MongoService.GetCollection("rooms")
	count, err := roomsCollection.CountDocuments(context.Background(), bson.M{
		"_id":       roomObjectID,
		"members":   userObjectID,
		"is_active": true,
	})
	if err != nil || count == 0 {
		return err
	}

	cs.Hub.mutex.Lock()
	defer cs.Hub.mutex.Unlock()

	for currentRoomID := range client.Rooms {
		cs.leaveRoomInternalUnsafe(client, currentRoomID)
	}

	if cs.Hub.Rooms[roomID] == nil {
		cs.Hub.Rooms[roomID] = make(map[string]*Client)
	}
	cs.Hub.Rooms[roomID][client.UserID] = client
	client.Rooms[roomID] = true

	return nil
}

func (cs *ChatService) JoinRoom(client *Client, roomID string) error {
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}

	var room models.Room
	err = cs.MongoService.GetCollection("rooms").FindOne(
		context.Background(),
		bson.M{"_id": roomObjectID, "is_active": true},
	).Decode(&room)
	if err != nil {
		return err
	}

	cs.Hub.mutex.Lock()
	defer cs.Hub.mutex.Unlock()

	for currentRoomID := range client.Rooms {
		cs.leaveRoomInternalUnsafe(client, currentRoomID)
	}

	if cs.Hub.Rooms[roomID] == nil {
		cs.Hub.Rooms[roomID] = make(map[string]*Client)
	}
	cs.Hub.Rooms[roomID][client.UserID] = client
	client.Rooms[roomID] = true

	cs.Hub.mutex.Unlock()
	cs.Hub.mutex.Lock()

	notification := models.WSResponse{
		Type:    "user_joined",
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

	cs.Hub.mutex.Unlock()
	cs.broadcastToRoomExcept(roomID, notification, client)
	cs.Hub.mutex.Lock()

	return nil
}

func (cs *ChatService) LeaveRoom(client *Client, roomID string) {
	cs.leaveRoomInternal(client, roomID)
}

func (cs *ChatService) GetUserCurrentRoom(client *Client) string {
	cs.Hub.mutex.RLock()
	defer cs.Hub.mutex.RUnlock()

	for roomID := range client.Rooms {
		return roomID // Should only be one room
	}
	return ""
}

func (cs *ChatService) SendMessage(client *Client, roomID, content, messageType string) error {
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return err
	}

	userObjectID, err := primitive.ObjectIDFromHex(client.UserID)
	if err != nil {
		return err
	}

	message := models.Message{
		ID:        primitive.NewObjectID(),
		RoomID:    roomObjectID,
		UserID:    userObjectID,
		Username:  client.Username,
		Picture:   client.Picture,
		Content:   content,
		Type:      messageType,
		IsEdited:  false,
		IsDeleted: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = cs.MongoService.GetCollection("messages").InsertOne(context.Background(), message)
	if err != nil {
		return err
	}

	cs.broadcastToRoomExcept(
		roomID,
		models.WSResponse{
			Type:    "new_message",
			Success: true,
			Data: map[string]any{
				"message":        message,
				"is_own_message": false,
			},
		},
		client,
	)

	cs.sendToClient(client, models.WSResponse{
		Type:    "new_message",
		Success: true,
		Data: map[string]any{
			"message":        message,
			"is_own_message": false,
		},
	})

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

	cs.Hub.mutex.Lock()
	defer cs.Hub.mutex.Unlock()

	for roomID := range client.Rooms {
		cs.leaveRoomInternalUnsafe(client, roomID)
	}

	delete(cs.Hub.Clients, client.UserID)

	go func() {
		close(client.Send)
	}()
	go func() {
		if client.Conn != nil {
			client.Conn.Close()
		}
	}()
}

func (cs *ChatService) leaveRoomInternalUnsafe(client *Client, roomID string) {
	if room, exists := cs.Hub.Rooms[roomID]; exists {
		delete(room, client.UserID)
		if len(room) == 0 {
			delete(cs.Hub.Rooms, roomID)
		}
	}

	delete(client.Rooms, roomID)

	notification := models.WSResponse{
		Type:    "user_left",
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

	if len(cs.Hub.Rooms[roomID]) > 0 {
		cs.Hub.mutex.Unlock()
		cs.broadcastToRoom(roomID, cs.marshalResponse(notification))
		cs.Hub.mutex.Lock()
	}
}

func (cs *ChatService) leaveRoomInternal(client *Client, roomID string) {
	cs.Hub.mutex.Lock()
	defer cs.Hub.mutex.Unlock()
	cs.leaveRoomInternalUnsafe(client, roomID)
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
		if roomID, exists := payload["room_id"].(string); exists {
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
		cCopy := c
		go func(cl *Client) {
			cs.Hub.Unregister <- cl
		}(cCopy)
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
		cCopy := c
		go func(cl *Client) {
			cs.Hub.Unregister <- cl
		}(cCopy)
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
		cCopy := c
		go func(cl *Client) {
			cs.Hub.Unregister <- cl
		}(cCopy)
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

func (cs *ChatService) marshalResponse(response models.WSResponse) []byte {
	data, _ := json.Marshal(response)
	return data
}
