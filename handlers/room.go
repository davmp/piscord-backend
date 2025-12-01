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
	AuthService  *services.AuthService
	ChatService  *services.ChatService
	MongoService *services.MongoService
	RedisService *services.RedisService
}

func NewRoomHandler(authService *services.AuthService, chatService *services.ChatService, mongoService *services.MongoService, redisService *services.RedisService) *RoomHandler {
	return &RoomHandler{
		AuthService:  authService,
		ChatService:  chatService,
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

	if req.Name == "" {
		req.Name = "Desconhecido"
	}

	room := models.Room{
		ID:          primitive.NewObjectID(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Picture:     req.Picture,
		OwnerID:     userObjectID,
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
		room.DirectKey = userIDs[0].Hex() + ":" + userIDs[1].Hex()

		var existing models.Room
		err = roomsCollection.FindOne(context.Background(), bson.M{
			"type":       "direct",
			"direct_key": room.DirectKey,
			"is_active":  true,
		}).Decode(&existing)

		if err == nil {
			roomResponse := models.RoomResponse{
				ID:          existing.ID,
				DisplayName: existing.Name,
				Description: existing.Description,
				Picture:     existing.Picture,
				Type:        existing.Type,
				OwnerID:     existing.OwnerID,
				MemberCount: len(existing.Members),
				MaxMembers:  existing.MaxMembers,
				IsActive:    existing.IsActive,
				IsAdmin:     slices.Contains(existing.Admins, userObjectID),
				CreatedAt:   existing.CreatedAt,
				UpdatedAt:   existing.UpdatedAt,
			}

			if existing.Type == "direct" {
				var otherMemberID primitive.ObjectID
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

	h.RedisService.Publish("chat", "room.create", room)

	roomResponse := models.RoomResponse{
		ID:          room.ID,
		DisplayName: room.Name,
		Type:        room.Type,
		Picture:     room.Picture,
		Description: room.Description,
		OwnerID:     room.OwnerID,
		MemberCount: len(room.Members),
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		IsAdmin:     slices.Contains(room.Admins, userObjectID),
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
	}

	if room.Type == "direct" {
		var otherMemberID primitive.ObjectID
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
		roomResponse.DisplayName = user.Username
		roomResponse.Picture = user.Picture
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
		"is_active": true,
		"$or": []bson.M{
			{"members": userObjectID},
			{"type": "public"},
		},
	}).Decode(&room)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room"})
		}
		return
	}

	roomResponse := models.RoomDetailsResponse{
		ID:          room.ID,
		DisplayName: room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		MemberCount: len(room.Members),
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		IsAdmin:     slices.Contains(room.Admins, userObjectID),
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
	}

	if room.Type == "direct" {
		var otherMemberID primitive.ObjectID
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

	c.JSON(http.StatusOK, roomResponse)
}

func (h *RoomHandler) GetRoomMembers(c *gin.Context) {
	roomID := c.Param("id")
	roomObjectID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
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

	memberIDs := room.Members
	adminIDs := room.Admins

	var members []models.RoomMember
	userCollection := h.MongoService.GetCollection("users")

	cursor, err := userCollection.Find(context.Background(), bson.M{
		"_id": bson.M{"$in": memberIDs},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			continue
		}
		members = append(members, models.RoomMember{
			UserID:   user.ID,
			Username: user.Username,
			Picture:  user.Picture,
			IsAdmin:  slices.Contains(adminIDs, user.ID),
			IsOnline: h.ChatService.GetUserStatus(user.ID.Hex()),
			IsMe:     user.ID == userObjectID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(room.Members),
		"members": members,
	})
}

func (h *RoomHandler) GetDirectRoom(c *gin.Context) {
	participantID := c.Param("id")
	participantObjectID, err := primitive.ObjectIDFromHex(participantID)
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

	var userIDs = []primitive.ObjectID{userObjectID, participantObjectID}
	slices.SortFunc(userIDs, func(a, b primitive.ObjectID) int {
		return strings.Compare(a.String(), b.String())
	})
	directKey := userIDs[0].Hex() + ":" + userIDs[1].Hex()

	roomsCollection := h.MongoService.GetCollection("rooms")
	var room models.Room
	err = roomsCollection.FindOne(context.Background(), bson.M{
		"is_active":  true,
		"direct_key": directKey,
		"type":       "direct",
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
		DisplayName: room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		OwnerID:     room.OwnerID,
		MemberCount: 2,
		MaxMembers:  2,
		IsActive:    room.IsActive,
		IsAdmin:     false,
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
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
		"type":      "public",
		"is_active": true,
	}

	if search := c.Query("search"); search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}
	opt := options.Find().SetSort(bson.D{{Key: "members", Value: -1}, {Key: "created_at", Value: 1}})

	cursor, err := roomsCollection.Find(context.Background(), filter, opt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		return
	}
	defer cursor.Close(context.Background())

	rooms := []models.PublicRoomResponse{}

	for cursor.Next(context.Background()) {
		var room models.Room
		if err := cursor.Decode(&room); err != nil {
			continue
		}

		roomResponse := models.PublicRoomResponse{
			RoomResponse: models.RoomResponse{
				ID:          room.ID,
				Description: room.Description,
				Type:        room.Type,
				Picture:     room.Picture,
				OwnerID:     room.OwnerID,
				MemberCount: len(room.Members),
				MaxMembers:  room.MaxMembers,
				IsActive:    room.IsActive,
				IsAdmin:     slices.Contains(room.Admins, userObjectID),
				CreatedAt:   room.CreatedAt,
				UpdatedAt:   room.UpdatedAt,
				LastMessage: nil,
			},
			DisplayName: room.Name,
			IsMember:    !userObjectID.IsZero() && slices.Contains(room.Members, userObjectID),
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
	if len(req.RemoveParticipantIDs) > 0 {
		members := []primitive.ObjectID{}

		for _, pid := range room.Members {
			if !slices.Contains(req.RemoveParticipantIDs, pid.Hex()) {
				members = append(members, pid)
			}
		}

		updateFields["members"] = members
	}
	if req.MaxMembers != 0 && req.MaxMembers != room.MaxMembers {
		if req.MaxMembers >= len(room.Members) {
			room.MaxMembers = req.MaxMembers
			updateFields["max_members"] = room.MaxMembers
		}
	}

	if len(updateFields) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No changes made"})
		return
	}

	room.UpdatedAt = time.Now()
	updateFields["updated_at"] = room.UpdatedAt

	h.RedisService.Publish("chat", "room.update", updateFields)

	roomResponse := models.RoomDetailsResponse{
		ID:          room.ID,
		DisplayName: room.Name,
		Description: room.Description,
		Type:        room.Type,
		Picture:     room.Picture,
		MemberCount: len(room.Members),
		MaxMembers:  room.MaxMembers,
		IsActive:    room.IsActive,
		IsAdmin:     slices.Contains(room.Admins, userObjectID),
		CreatedAt:   room.CreatedAt,
		UpdatedAt:   room.UpdatedAt,
	}

	if room.Type == "direct" {
		var otherMemberID primitive.ObjectID
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

	c.JSON(http.StatusOK, roomResponse)
}

func (h *RoomHandler) GetMyRooms(c *gin.Context) {
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

	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := roomsCollection.Find(context.Background(), filter, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		return
	}
	defer cursor.Close(context.Background())

	rooms := []models.RoomResponse{}

	for cursor.Next(context.Background()) {
		var room models.Room
		if err := cursor.Decode(&room); err != nil {
			continue
		}

		roomResponse := models.RoomResponse{
			ID:          room.ID,
			DisplayName: room.Name,
			Description: room.Description,
			Type:        room.Type,
			Picture:     room.Picture,
			OwnerID:     room.OwnerID,
			MemberCount: len(room.Members),
			MaxMembers:  room.MaxMembers,
			IsActive:    room.IsActive,
			IsAdmin:     slices.Contains(room.Admins, userObjectID),
			CreatedAt:   room.CreatedAt,
			UpdatedAt:   room.UpdatedAt,
			LastMessage: nil,
		}

		if room.Type == "direct" {
			var otherMemberID primitive.ObjectID
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

			roomResponse.DisplayName = user.Username
			roomResponse.Picture = user.Picture
		}

		var message *models.Message = nil
		messageCursor, err := h.MongoService.GetCollection("messages").Find(
			context.Background(),
			bson.M{
				"room_id":    room.ID,
				"is_deleted": false,
			},
			options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(1),
		)
		if err == nil && messageCursor.Next(context.Background()) {
			var lastMessage models.Message
			if err := messageCursor.Decode(&lastMessage); err == nil {
				message = &lastMessage
			}
		}
		messageCursor.Close(context.Background())

		if err == nil && message != nil {
			user, err := h.AuthService.GetUserByID(message.UserID)

			if err == nil {
				roomResponse.LastMessage = &models.MessagePreviewResponse{
					ID:        message.ID,
					Username:  user.Username,
					Picture:   user.Picture,
					Content:   message.Content,
					CreatedAt: message.CreatedAt,
				}
			}
		}

		rooms = append(rooms, roomResponse)
	}

	slices.SortFunc(rooms, func(a, b models.RoomResponse) int {
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

	h.RedisService.Publish("chat", "room.join", bson.M{
		"id":      roomObjectID,
		"user_id": userObjectID,
	})

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

	h.RedisService.Publish("chat", "room.leave", bson.M{
		"id":      roomObjectID,
		"user_id": userObjectID,
	})

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

	messages := []models.MessageResponse{}
	users := make(map[primitive.ObjectID]*models.User)

	for cursor.Next(context.Background()) {
		var message models.Message
		if err := cursor.Decode(&message); err != nil {
			continue
		}

		user := users[message.UserID]
		if user == nil {
			user, err = h.AuthService.GetUserByID(message.UserID)
			if err != nil {
				user = &models.User{
					ID:       message.UserID,
					Username: "Desconhecido",
				}
			} else {
				users[message.UserID] = user
			}
		}

		messageResponse := models.MessageResponse{
			ID:        message.ID,
			RoomID:    message.RoomID,
			UserID:    message.UserID,
			Username:  user.Username,
			Picture:   user.Picture,
			Content:   message.Content,
			Type:      message.Type,
			FileURL:   message.FileURL,
			ReplyTo:   nil,
			IsEdited:  message.IsEdited,
			CreatedAt: message.CreatedAt,
			UpdatedAt: message.UpdatedAt,
		}

		if message.ReplyTo != nil {
			var replyToMessage models.Message

			if err := h.MongoService.GetCollection("messages").FindOne(context.Background(), bson.M{"_id": message.ReplyTo}).Decode(&replyToMessage); err == nil {
				messageResponse.ReplyTo = &models.MessagePreviewResponse{
					ID:        replyToMessage.ID,
					Username:  "Desconhecido",
					Content:   replyToMessage.Content,
					CreatedAt: replyToMessage.CreatedAt,
				}

				if user, err := h.AuthService.GetUserByID(replyToMessage.UserID); err == nil {
					messageResponse.ReplyTo.Username = user.Username
					messageResponse.ReplyTo.Picture = user.Picture
				}
			}
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
