package handlers

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"piscord-backend/models"
	"piscord-backend/services"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type RoomHandler struct {
	AuthService  *services.AuthService
	ChatService  *services.ChatService
	RoomService  *services.RoomService
	MongoService *services.MongoService
	RedisService *services.RedisService
}

func NewRoomHandler(authService *services.AuthService, chatService *services.ChatService, roomService *services.RoomService, mongoService *services.MongoService, redisService *services.RedisService) *RoomHandler {
	return &RoomHandler{
		AuthService:  authService,
		ChatService:  chatService,
		RoomService:  roomService,
		MongoService: mongoService,
		RedisService: redisService,
	}
}

func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req models.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	for _, pid := range req.Members {
		if pid == userID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create direct room with yourself"})
			return
		}
	}

	if req.Name == "" {
		req.Name = "Desconhecido"
	}

	room := models.Room{
		ID:          bson.NewObjectID(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Picture:     req.Picture,
		OwnerID:     userObjectID,
		Members:     []bson.ObjectID{userObjectID},
		Admins:      []bson.ObjectID{},
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	roomsCollection := h.MongoService.GetCollection("rooms")

	if req.Type == "direct" {
		room.MaxMembers = 2
		participantID, err := bson.ObjectIDFromHex(req.Members[0])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}
		room.Members = append(room.Members, participantID)

		var userIDs = []bson.ObjectID{userObjectID, participantID}
		slices.SortFunc(userIDs, func(a, b bson.ObjectID) int {
			return strings.Compare(a.String(), b.String())
		})
		room.DirectKey = userIDs[0].Hex() + ":" + userIDs[1].Hex()

		var existing models.Room
		err = roomsCollection.FindOne(context.Background(), bson.M{
			"type":      "direct",
			"directKey": room.DirectKey,
			"isActive":  true,
		}).Decode(&existing)

		if err == nil {
			roomPreview := models.RoomPreview{
				ID:          existing.ID,
				DisplayName: existing.Name,
				Description: existing.Description,
				Type:        existing.Type,
				Picture:     existing.Picture,
				LastMessage: nil,
			}

			if existing.Type == "direct" {
				var otherMemberID bson.ObjectID
				for _, memberID := range existing.Members {
					if memberID != userObjectID {
						otherMemberID = memberID
						break
					}
				}

				user, err := h.AuthService.GetUserByID(otherMemberID)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
					return
				}
				roomPreview.DisplayName = user.Username
				roomPreview.Picture = user.Picture
			} else {
				roomPreview.DisplayName = room.Name
			}

			c.JSON(http.StatusOK, roomPreview)
			return
		}
		if err != mongo.ErrNoDocuments {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing room"})
			return
		}
	} else {
		room.MaxMembers = req.MaxMembers
		for _, pid := range req.Members {
			participantID, err := bson.ObjectIDFromHex(pid)
			if err == nil && participantID != userObjectID {
				room.Members = append(room.Members, participantID)
			}
		}
		room.Admins = []bson.ObjectID{userObjectID}
	}

	h.RedisService.Publish("chat", "room.create", room)
	h.RedisService.AddUserToRoom(userObjectID.Hex(), room.ID.Hex())

	roomPreview := models.RoomPreview{
		ID:          room.ID,
		DisplayName: room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		LastMessage: nil,
	}

	if room.Type == "direct" {
		var otherMemberID bson.ObjectID
		for _, memberID := range room.Members {
			if memberID != userObjectID {
				otherMemberID = memberID
				break
			}
		}

		user, err := h.AuthService.GetUserByID(otherMemberID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}
		roomPreview.DisplayName = user.Username
		roomPreview.Picture = user.Picture
	}

	c.JSON(http.StatusCreated, roomPreview)
}

func (h *RoomHandler) GetRoom(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	var room models.Room
	err = roomsCollection.FindOne(context.Background(), bson.M{
		"_id":      roomObjectID,
		"isActive": true,
	}).Decode(&room)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
	}

	isMember := slices.Contains(room.Members, userObjectID)
	isInMemory := h.ChatService.IsUserInRoom(userObjectID.Hex(), roomObjectID.Hex())

	if room.Type != "public" && !isMember && !isInMemory {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	roomResponse := models.RoomDetails{
		ID:          room.ID,
		DisplayName: room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		IsAdmin:     slices.Contains(room.Admins, userObjectID),
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
	}

	if room.Type == "direct" {
		var otherMemberID bson.ObjectID
		for _, memberID := range room.Members {
			if memberID != userObjectID {
				otherMemberID = memberID
				break
			}
		}

		user, err := h.AuthService.GetUserByID(otherMemberID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}

		roomResponse.IsAdmin = false
		roomResponse.DisplayName = user.Username
		roomResponse.Picture = user.Picture
	}

	members, err := h.RoomService.GetMembersByRoom(userObjectID, roomObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get members"})
		return
	}
	roomResponse.Members = members

	c.JSON(http.StatusOK, roomResponse)
}

func (h *RoomHandler) GetDirectRoom(c *gin.Context) {
	participantID := c.Param("id")
	participantObjectID, err := bson.ObjectIDFromHex(participantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var userIDs = []bson.ObjectID{userObjectID, participantObjectID}
	slices.SortFunc(userIDs, func(a, b bson.ObjectID) int {
		return strings.Compare(a.String(), b.String())
	})
	directKey := userIDs[0].Hex() + ":" + userIDs[1].Hex()

	roomsCollection := h.MongoService.GetCollection("rooms")
	var room models.Room
	err = roomsCollection.FindOne(context.Background(), bson.M{
		"isActive":  true,
		"directKey": directKey,
		"type":      "direct",
	}).Decode(&room)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
	}

	user, err := h.AuthService.GetUserByID(userObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	participant, err := h.AuthService.GetUserByID(participantObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get participant info"})
		return
	}

	roomResponse := models.RoomDetails{
		ID:          room.ID,
		DisplayName: participant.Username,
		Description: participant.Bio,
		Type:        room.Type,
		Picture:     participant.Picture,
		MaxMembers:  2,
		IsActive:    room.IsActive,
		IsAdmin:     false,
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
	}

	isOnline := false
	if isOnline, err = h.RedisService.IsUserOnline(participantID); err != nil {
	}

	roomResponse.Members = []models.RoomMember{
		{
			UserID:   userObjectID,
			Username: user.Username,
			Picture:  user.Picture,
			IsAdmin:  false,
			IsOnline: true,
			IsMe:     true,
		},
		{
			UserID:   participantObjectID,
			Username: participant.Username,
			Picture:  participant.Picture,
			IsAdmin:  false,
			IsOnline: isOnline,
			IsMe:     false,
		},
	}

	c.JSON(http.StatusOK, roomResponse)
}

func (h *RoomHandler) GetRooms(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	filter := bson.M{
		"type":     "public",
		"isActive": true,
	}

	if search := c.Query("search"); search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}
	opt := options.Find().SetSort(bson.D{{Key: "members", Value: -1}, {Key: "createdAt", Value: 1}})

	cursor, err := roomsCollection.Find(context.Background(), filter, opt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		return
	}
	defer cursor.Close(context.Background())

	rooms := []models.PublicRoom{}

	for cursor.Next(context.Background()) {
		var room models.Room
		if err := cursor.Decode(&room); err != nil {
			continue
		}

		roomResponse := models.PublicRoom{
			RoomPreview: models.RoomPreview{
				ID:          room.ID,
				DisplayName: room.Name,
				Description: room.Description,
				Type:        room.Type,
				Picture:     room.Picture,
				LastMessage: nil,
			},
			IsMember: !userObjectID.IsZero() && slices.Contains(room.Members, userObjectID),
		}

		rooms = append(rooms, roomResponse)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  rooms,
		"total": len(rooms),
	})
}

func (h *RoomHandler) UpdateRoom(c *gin.Context) {
	var req models.UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roomID := c.Param("id")
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	var room models.Room
	err = roomsCollection.FindOne(context.Background(), bson.M{
		"_id":    roomObjectID,
		"admins": userObjectID,
	}).Decode(&room)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
	}

	updateFields := bson.M{}

	if req.Name != "" && req.Name != room.Name {
		room.Name = req.Name
		updateFields["name"] = room.Name
	}
	if req.Description != "" && req.Description != room.Description {
		room.Description = req.Description
		updateFields["description"] = room.Description
	}
	if req.Picture != "" && req.Picture != room.Picture {
		room.Picture = req.Picture
		updateFields["picture"] = room.Picture
	}
	if len(req.RemoveMembers) > 0 {
		members := []bson.ObjectID{}

		for _, pid := range room.Members {
			if !slices.Contains(req.RemoveMembers, pid.Hex()) {
				members = append(members, pid)
			}
		}

		updateFields["members"] = members
	}
	if req.MaxMembers != 0 && req.MaxMembers != room.MaxMembers {
		if req.MaxMembers >= len(room.Members) {
			room.MaxMembers = req.MaxMembers
			updateFields["maxMembers"] = room.MaxMembers
		}
	}

	if len(updateFields) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No changes made"})
		return
	}

	room.UpdatedAt = time.Now()
	updateFields["updatedAt"] = room.UpdatedAt

	h.RedisService.Publish("chat", "room.update", updateFields)

	roomResponse := models.RoomDetails{
		ID:          room.ID,
		DisplayName: room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		IsAdmin:     slices.Contains(room.Admins, userObjectID),
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
	}

	if room.Type == "direct" {
		var otherMemberID bson.ObjectID
		for _, memberID := range room.Members {
			if memberID != userObjectID {
				otherMemberID = memberID
				break
			}
		}

		user, err := h.AuthService.GetUserByID(otherMemberID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}

		roomResponse.IsAdmin = false
		roomResponse.DisplayName = user.Username
		roomResponse.Picture = user.Picture
	}

	members, err := h.RoomService.GetMembersByRoom(userObjectID, roomObjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get members"})
		return
	}
	roomResponse.Members = members

	c.JSON(http.StatusOK, roomResponse)
}

func (h *RoomHandler) GetMyRooms(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	filter := bson.M{
		"members":  userObjectID,
		"isActive": true,
	}

	if search := c.Query("search"); search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	opts := options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}})

	cursor, err := roomsCollection.Find(context.Background(), filter, opts)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "No rooms found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		}
		return
	}
	defer cursor.Close(context.Background())

	rooms := []models.RoomPreview{}

	for cursor.Next(context.Background()) {
		var room models.Room
		if err := cursor.Decode(&room); err != nil {
			continue
		}

		roomPreview := models.RoomPreview{
			ID:          room.ID,
			DisplayName: room.Name,
			Description: room.Description,
			Type:        room.Type,
			Picture:     room.Picture,
			LastMessage: nil,
		}

		if room.Type == "direct" {
			var otherMemberID bson.ObjectID
			for _, memberID := range room.Members {
				if memberID != userObjectID {
					otherMemberID = memberID
					break
				}
			}

			user, err := h.AuthService.GetUserByID(otherMemberID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
				return
			}

			roomPreview.DisplayName = user.Username
			roomPreview.Picture = user.Picture
			roomPreview.Description = user.Bio
		}

		var message *models.Message = nil
		messageCursor, err := h.MongoService.GetCollection("messages").Find(
			context.Background(),
			bson.M{
				"roomId":  room.ID,
				"deleted": false,
			},
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(1),
		)
		if err == nil && messageCursor.Next(context.Background()) {
			var lastMessage models.Message
			if err := messageCursor.Decode(&lastMessage); err == nil {
				message = &lastMessage
			}
		}
		messageCursor.Close(context.Background())

		if err == nil && message != nil {
			user, err := h.AuthService.GetUserByID(message.Author.ID)

			if err == nil {
				roomPreview.LastMessage = &models.MessagePreview{
					ID:      message.ID,
					Content: message.Content,
					Author: models.UserSummary{
						ID:       user.ID,
						Username: user.Username,
						Picture:  user.Picture,
					},
					CreatedAt: message.CreatedAt,
				}
			}
		}

		rooms = append(rooms, roomPreview)
	}

	slices.SortFunc(rooms, func(a, b models.RoomPreview) int {
		var aTime, bTime time.Time
		if a.LastMessage != nil {
			aTime = a.LastMessage.CreatedAt
		}
		if b.LastMessage != nil {
			bTime = b.LastMessage.CreatedAt
		}
		return bTime.Compare(aTime)
	})

	c.JSON(http.StatusOK, gin.H{
		"data":  rooms,
		"total": len(rooms),
	})
}

func (h *RoomHandler) JoinRoom(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	h.RedisService.Publish("chat", "room.join", bson.M{
		"id":     roomObjectID,
		"userId": userObjectID,
	})
	h.RedisService.AddUserToRoom(userObjectID.Hex(), roomObjectID.Hex())

	c.JSON(http.StatusOK, gin.H{})
}

func (h *RoomHandler) LeaveRoom(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	h.RedisService.Publish("chat", "room.leave", bson.M{
		"id":     roomObjectID,
		"userId": userObjectID,
	})
	h.RedisService.RemoveUserFromRoom(userObjectID.Hex(), roomObjectID.Hex())

	c.JSON(http.StatusOK, gin.H{})
}

func (h *RoomHandler) GetMessages(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if !h.ChatService.IsUserInRoom(userObjectID.Hex(), roomObjectID.Hex()) {
		roomsCollection := h.MongoService.GetCollection("rooms")
		count, err := roomsCollection.CountDocuments(context.Background(), bson.M{
			"_id":      roomObjectID,
			"members":  userObjectID,
			"isActive": true,
		})

		if err != nil || count == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	page := 1
	limit := 50
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	skip := (page - 1) * limit

	messagesCollection := h.MongoService.GetCollection("messages")
	filter := bson.M{
		"roomId":  roomObjectID,
		"deleted": false,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := messagesCollection.Find(context.Background(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}
	defer cursor.Close(context.Background())

	messages := []models.Message{}
	if err = cursor.All(context.Background(), &messages); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  messages,
		"page":  page,
		"size":  limit,
		"total": len(messages),
	})
}
