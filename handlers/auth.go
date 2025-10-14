package handlers

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"piscord-backend/config"
	"piscord-backend/models"
	"piscord-backend/services"
	"piscord-backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AuthHandler struct {
	AuthService *services.AuthService
	ChatService *services.ChatService
	RoomService *services.RoomService
	Config      *config.Config
}

func NewAuthHandler(authService *services.AuthService, chatService *services.ChatService, roomService *services.RoomService, config *config.Config) *AuthHandler {
	return &AuthHandler{
		AuthService: authService,
		ChatService: chatService,
		RoomService: roomService,
		Config:      config,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := h.AuthService.GetUserByUsername(req.Username)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	} else if err != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		ID:        primitive.NewObjectID(),
		Username:  req.Username,
		Password:  hashedPassword,
		Picture:   req.Picture,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = h.AuthService.CreateUser(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user record"})
		return
	}

	token, err := utils.GenerateJWT(user.ID.Hex(), user.Username, user.Picture, h.Config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User created but login failed: " + err.Error()})
		return
	}

	userResponse := models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Picture:   user.Picture,
		Bio:       user.Bio,
		CreatedAt: user.CreatedAt,
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user":  userResponse,
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.AuthService.GetUserByUsername(req.Username)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	if !utils.CheckPasswordHash(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := utils.GenerateJWT(user.ID.Hex(), user.Username, user.Picture, h.Config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	userResponse := models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Picture:   user.Picture,
		Bio:       user.Bio,
		CreatedAt: user.CreatedAt,
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  userResponse,
	})
}

func (h *AuthHandler) Profile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.AuthService.GetUserByID(userObjectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	userResponse := models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Picture:   user.Picture,
		Bio:       user.Bio,
		CreatedAt: user.CreatedAt,
	}

	c.JSON(http.StatusOK, userResponse)
}

func (h *AuthHandler) GetProfileByID(c *gin.Context) {
	profileID := c.Param("id")
	profileObjectID, err := primitive.ObjectIDFromHex(profileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, _ := c.Get("user_id")
	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := h.AuthService.GetUserByID(profileObjectID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		}
		return
	}

	profileResponse := models.ProfileResponse{
		UserResponse: models.UserResponse{
			ID:        user.ID,
			Username:  user.Username,
			Picture:   user.Picture,
			Bio:       user.Bio,
			CreatedAt: user.CreatedAt,
		},
		IsOnline:     h.ChatService.GetUserStatus(user.ID.Hex()),
		DirectChatID: nil,
	}

	if userObjectID != profileObjectID {
		var userIDs = []primitive.ObjectID{userObjectID, profileObjectID}
		slices.SortFunc(userIDs, func(a, b primitive.ObjectID) int {
			return strings.Compare(a.String(), b.String())
		})

		room, _ := h.RoomService.GetRoomByDirectKey(userIDs[0].Hex() + ":" + userIDs[1].Hex())
		if room != nil {
			profileResponse.DirectChatID = &room.ID
		}
	}

	c.JSON(http.StatusOK, profileResponse)
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userObjectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	update := bson.M{}

	if req.Picture != "" {
		update = bson.M{
			"picture": req.Picture,
		}
	}

	if req.Username != "" {
		if _, err := h.AuthService.GetUserByUsername(req.Username); err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}

		update["username"] = req.Username
	}

	if req.Bio != "" {
		update["bio"] = req.Bio
	}

	if req.Password != "" {
		hashedPassword, err := utils.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		update["password"] = hashedPassword
	}

	user, err := h.AuthService.UpdateUser(userObjectID, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user record"})
		return
	}

	userResponse := models.UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Picture:   user.Picture,
		Bio:       user.Bio,
		CreatedAt: user.CreatedAt,
	}

	c.JSON(http.StatusOK, userResponse)
}
