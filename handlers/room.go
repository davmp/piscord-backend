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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type RoomHandler struct {
	MongoService *services.MongoService
	AuthService  *services.AuthService
}

func NewRoomHandler(mongoService *services.MongoService, authService *services.AuthService) *RoomHandler {
	return &RoomHandler{
		MongoService: mongoService,
		AuthService:  authService,
	}
}

func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req models.CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	for _, pid := range req.ParticipantIDs {
		participantObjectID, err := primitive.ObjectIDFromHex(pid)
		if err == nil && participantObjectID == userObjectID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create direct room with yourself"})
			return
		}
	}

	room := models.Room{
		ID:          primitive.NewObjectID(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Picture:     req.Picture,
		CreatedBy:   userObjectID,
		Members:     []primitive.ObjectID{userObjectID},
		Admins:      []primitive.ObjectID{},
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	roomsCollection := h.MongoService.GetCollection("rooms")

	if req.Type == "direct" {
		room.MaxMembers = 2
		participantID, err := primitive.ObjectIDFromHex(req.ParticipantIDs[0])
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}
		room.Members = append(room.Members, participantID)

		var userIDs = []primitive.ObjectID{userObjectID, participantID}
		slices.SortFunc(userIDs, func(a, b primitive.ObjectID) int {
			return strings.Compare(a.String(), b.String())
		})
		room.DirectKey = userIDs[0].String() + ":" + userIDs[1].String()

		var existing models.Room
		err = roomsCollection.FindOne(context.Background(), bson.M{
			"type":       "direct",
			"direct_key": room.DirectKey,
			"is_active":  true,
		}).Decode(&existing)

		if err == nil {
			roomResponse := models.RoomResponse{
				ID:          existing.ID,
				Name:        existing.Name,
				Description: existing.Description,
				Picture:     existing.Picture,
				Type:        existing.Type,
				CreatedBy:   existing.CreatedBy,
				MemberCount: len(existing.Members),
				MaxMembers:  existing.MaxMembers,
				IsActive:    existing.IsActive,
				CreatedAt:   existing.CreatedAt,
				UpdatedAt:   existing.UpdatedAt,
				LastAction:  nil,
			}

			if existing.Type == "direct" {
				var otherMemberID primitive.ObjectID
				for _, memberID := range existing.Members {
					if memberID != userObjectID {
						otherMemberID = memberID
						break
					}
				}

				user, err := h.AuthService.GetUserByID(otherMemberID.Hex())
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
					return
				}
				roomResponse.DisplayName = user.Username
				roomResponse.Picture = user.Picture
			} else {
				roomResponse.DisplayName = room.Name
			}

			c.JSON(http.StatusOK, roomResponse)
			return
		}
		if err != mongo.ErrNoDocuments {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing room"})
			return
		}
	} else {
		room.MaxMembers = req.MaxMembers
		for _, pid := range req.ParticipantIDs {
			participantID, err := primitive.ObjectIDFromHex(pid)
			if err == nil && participantID != userObjectID {
				room.Members = append(room.Members, participantID)
			}
		}
		room.Admins = []primitive.ObjectID{userObjectID}
	}

	_, err = roomsCollection.InsertOne(context.Background(), room)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create room"})
		return
	}

	roomResponse := models.RoomResponse{
		ID:          room.ID,
		Name:        room.Name,
		Type:        room.Type,
		Picture:     room.Picture,
		Description: room.Description,
		CreatedBy:   room.CreatedBy,
		MemberCount: len(room.Members),
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
		LastAction:  nil,
	}

	if room.Type == "direct" {
		var otherMemberID primitive.ObjectID
		for _, memberID := range room.Members {
			if memberID != userObjectID {
				otherMemberID = memberID
				break
			}
		}

		user, err := h.AuthService.GetUserByID(otherMemberID.Hex())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}
		roomResponse.DisplayName = user.Username
		roomResponse.Picture = user.Picture
	} else {
		roomResponse.DisplayName = room.Name
	}

	c.JSON(http.StatusCreated, roomResponse)
}

func (h *RoomHandler) GetRoom(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	var room models.Room
	err = roomsCollection.FindOne(context.Background(), bson.M{
		"_id":       roomObjectID,
		"members":   userObjectID,
		"is_active": true,
	}).Decode(&room)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
	}

	roomResponse := models.RoomResponse{
		ID:          room.ID,
		Name:        room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		CreatedBy:   room.CreatedBy,
		MemberCount: len(room.Members),
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
		LastAction:  nil,
	}

	if room.Type == "direct" {
		var otherMemberID primitive.ObjectID
		for _, memberID := range room.Members {
			if memberID != userObjectID {
				otherMemberID = memberID
				break
			}
		}

		user, err := h.AuthService.GetUserByID(otherMemberID.Hex())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}

		roomResponse.DisplayName = user.Username
		roomResponse.Picture = user.Picture
	} else {
		roomResponse.DisplayName = room.Name
	}

	c.JSON(http.StatusOK, roomResponse)
}

func (h *RoomHandler) GetDirectRoom(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid username"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	participant, err := h.AuthService.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if userObjectID == participant.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create direct room with yourself"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	var room models.Room

	var userIDs = []primitive.ObjectID{userObjectID, participant.ID}
	slices.SortFunc(userIDs, func(a, b primitive.ObjectID) int {
		return strings.Compare(a.String(), b.String())
	})
	directKey := userIDs[0].String() + ":" + userIDs[1].String()

	err = roomsCollection.FindOne(context.Background(), bson.M{
		"type":       "direct",
		"direct_key": directKey,
		"is_active":  true,
	}).Decode(&room)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
	}

	roomResponse := models.RoomResponse{
		ID:          room.ID,
		Name:        room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		CreatedBy:   room.CreatedBy,
		MemberCount: len(room.Members),
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
		LastAction:  nil,
	}

	if room.Type == "direct" {
		var otherMemberID primitive.ObjectID
		for _, memberID := range room.Members {
			if memberID != userObjectID {
				otherMemberID = memberID
				break
			}
		}

		user, err := h.AuthService.GetUserByID(otherMemberID.Hex())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}

		roomResponse.DisplayName = user.Username
		roomResponse.Picture = user.Picture
	} else {
		roomResponse.DisplayName = room.Name
	}

	c.JSON(http.StatusOK, roomResponse)
}

func (h *RoomHandler) GetRooms(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	filter := bson.M{
		"members":   userObjectID,
		"is_active": true,
	}

	if search := c.Query("search"); search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	cursor, err := roomsCollection.Find(context.Background(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		return
	}
	defer cursor.Close(context.Background())

	messagesCollection := h.MongoService.GetCollection("messages")

	var roomsWithMessages = []models.RoomResponse{}

	for cursor.Next(context.Background()) {
		var room models.Room
		if err := cursor.Decode(&room); err != nil {
			continue
		}

		roomResponse := models.RoomResponse{
			ID:          room.ID,
			Name:        room.Name,
			Description: room.Description,
			Type:        room.Type,
			Picture:     room.Picture,
			CreatedBy:   room.CreatedBy,
			MemberCount: len(room.Members),
			MaxMembers:  room.MaxMembers,
			IsActive:    room.IsActive,
			CreatedAt:   room.CreatedAt,
			UpdatedAt:   room.UpdatedAt,
			LastAction:  nil,
		}

		if room.Type == "direct" {
			var otherMemberID primitive.ObjectID
			for _, memberID := range room.Members {
				if memberID != userObjectID {
					otherMemberID = memberID
					break
				}
			}

			user, err := h.AuthService.GetUserByID(otherMemberID.Hex())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
				return
			}

			roomResponse.DisplayName = user.Username
			roomResponse.Picture = user.Picture
		} else {
			roomResponse.DisplayName = room.Name
		}

		messageFilter := bson.M{
			"room_id":    room.ID,
			"is_deleted": false,
		}
		opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})

		var lastMessage models.Message
		err := messagesCollection.FindOne(context.Background(), messageFilter, opts).Decode(&lastMessage)

		if lastMessage.Content == "" || err == mongo.ErrNoDocuments {
			roomsWithMessages = append(roomsWithMessages, roomResponse)
			continue
		}

		var lastMessagePreview models.MessagePreviewResponse
		if err == nil {
			lastMessagePreview = models.MessagePreviewResponse{
				ID:        lastMessage.ID,
				Username:  lastMessage.Username,
				Content:   lastMessage.Content,
				CreatedAt: lastMessage.CreatedAt,
			}
		}

		roomResponse.LastAction = &lastMessagePreview
		roomsWithMessages = append(roomsWithMessages, roomResponse)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  roomsWithMessages,
		"total": len(roomsWithMessages),
	})
}

func (h *RoomHandler) JoinRoom(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	var room models.Room
	err = roomsCollection.FindOne(context.Background(), bson.M{
		"_id":       roomObjectID,
		"is_active": true,
	}).Decode(&room)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
	}

	if slices.Contains(room.Members, userObjectID) {
		c.JSON(http.StatusConflict, gin.H{"error": "Already a member of this room"})
		return
	}

	if room.MaxMembers > 0 && len(room.Members) >= room.MaxMembers {
		c.JSON(http.StatusForbidden, gin.H{"error": "Room is full"})
		return
	}

	update := bson.M{
		"$push": bson.M{"members": userObjectID},
		"$set":  bson.M{"updated_at": time.Now()},
	}

	_, err = roomsCollection.UpdateOne(context.Background(), bson.M{"_id": roomObjectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (h *RoomHandler) LeaveRoom(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	update := bson.M{
		"$pull": bson.M{
			"members": userObjectID,
			"admins":  userObjectID,
		},
		"$set": bson.M{"updated_at": time.Now()},
	}

	_, err = roomsCollection.UpdateOne(context.Background(), bson.M{"_id": roomObjectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (h *RoomHandler) GetMessages(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	count, err := roomsCollection.CountDocuments(context.Background(), bson.M{
		"_id":       roomObjectID,
		"members":   userObjectID,
		"is_active": true,
	})
	if err != nil || count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
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
		"room_id":    roomObjectID,
		"is_deleted": false,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := messagesCollection.Find(context.Background(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}
	defer cursor.Close(context.Background())

	var messages []models.MessageResponse
	for cursor.Next(context.Background()) {
		var message models.Message
		if err := cursor.Decode(&message); err != nil {
			continue
		}

		messageResponse := models.MessageResponse{
			ID:        message.ID,
			RoomID:    message.RoomID,
			UserID:    message.UserID,
			Username:  message.Username,
			Picture:   message.Picture,
			Content:   message.Content,
			Type:      message.Type,
			FileURL:   message.FileURL,
			ReplyTo:   message.ReplyTo,
			IsEdited:  message.IsEdited,
			CreatedAt: message.CreatedAt,
			UpdatedAt: message.UpdatedAt,
		}

		if message.UserID == userObjectID {
			messageResponse.IsOwnMessage = true
		} else {
			messageResponse.IsOwnMessage = false
		}

		messages = append(messages, messageResponse)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  messages,
		"page":  page,
		"size":  limit,
		"total": len(messages),
	})
}

func (h *RoomHandler) GetDirectMessages(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid username"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.AuthService.GetUserByUsername(username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	roomsCollection := h.MongoService.GetCollection("rooms")
	var roomDoc models.Room
	err = roomsCollection.FindOne(context.Background(), bson.M{
		"type":      "direct",
		"members":   []primitive.ObjectID{userObjectID, user.ID},
		"is_active": true,
	}).Decode(&roomDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Direct message room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
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
		"room_id":    roomDoc.ID,
		"is_deleted": false,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := messagesCollection.Find(context.Background(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}
	defer cursor.Close(context.Background())

	var messages []models.MessageResponse
	for cursor.Next(context.Background()) {
		var message models.Message
		if err := cursor.Decode(&message); err != nil {
			continue
		}

		messageResponse := models.MessageResponse{
			ID:        message.ID,
			RoomID:    message.RoomID,
			UserID:    message.UserID,
			Username:  message.Username,
			Picture:   message.Picture,
			Content:   message.Content,
			Type:      message.Type,
			FileURL:   message.FileURL,
			ReplyTo:   message.ReplyTo,
			IsEdited:  message.IsEdited,
			CreatedAt: message.CreatedAt,
			UpdatedAt: message.UpdatedAt,
		}
		if message.UserID == userObjectID {
			messageResponse.IsOwnMessage = true
		} else {
			messageResponse.IsOwnMessage = false
		}

		messages = append(messages, messageResponse)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  messages,
		"page":  page,
		"size":  limit,
		"total": len(messages),
	})
}
