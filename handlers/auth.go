package handlers

import (
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"piscord-backend/config"
	"piscord-backend/models"
	"piscord-backend/services"
	"piscord-backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AuthHandler struct {
	AuthService  *services.AuthService
	ChatService  *services.ChatService
	RoomService  *services.RoomService
	RedisService *services.RedisService
	Config       *config.Config
}

func NewAuthHandler(authService *services.AuthService, chatService *services.ChatService, roomService *services.RoomService, redisService *services.RedisService, config *config.Config) *AuthHandler {
	return &AuthHandler{
		AuthService:  authService,
		ChatService:  chatService,
		RoomService:  roomService,
		RedisService: redisService,
		Config:       config,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.UserLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.AuthService.GetUserByUsername(req.Username)
	if err != nil {
		log.Println("Error getting user by username: ", err)
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

	h.ChatService.UpdateClient(user.ID.Hex(), user.Username, user.Picture)
	h.RedisService.CacheUserProfile(user.ID.Hex(), *user)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": models.UserSummary{
			ID:       user.ID,
			Username: user.Username,
			Picture:  user.Picture,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.UserRegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "Username") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "O nome de usuário deve ter entre 3 e 30 caracteres."})
		} else if strings.Contains(errMsg, "Password") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A senha deve ter pelo menos 6 caracteres."})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Erro ao processar os dados de registro."})
		}
		return
	}

	_, err := h.AuthService.GetUserByUsername(req.Username)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Usuário já existe."})
		return
	} else if err != mongo.ErrNoDocuments {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro no banco de dados."})
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao criptografar a senha."})
		return
	}

	user := models.User{
		ID:        bson.NewObjectID(),
		Username:  req.Username,
		Password:  hashedPassword,
		Picture:   req.Picture,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = h.AuthService.CreateUser(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Falha ao criar o registro do usuário."})
		return
	}

	token, err := utils.GenerateJWT(user.ID.Hex(), user.Username, user.Picture, h.Config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Usuário criado, mas falha ao gerar o token de login: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": models.UserSummary{
			ID:       user.ID,
			Username: user.Username,
			Picture:  user.Picture,
		},
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
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

	var user *models.User
	err = h.RedisService.GetCachedUser(userObjectID.Hex(), &user)
	if err != nil {
		user, err = h.AuthService.GetUserByID(userObjectID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}
		h.RedisService.CacheUserProfile(userObjectID.Hex(), *user)
	}

	profile := models.ProfileResponse{
		UserSummary: models.UserSummary{
			ID:       user.ID,
			Username: user.Username,
			Picture:  user.Picture,
		},
		Bio:          user.Bio,
		CreatedAt:    user.CreatedAt,
		IsOnline:     true,
		DirectChatID: nil,
	}

	c.JSON(http.StatusOK, profile)
}

func (h *AuthHandler) GetProfileByID(c *gin.Context) {
	profileID := c.Param("id")
	profileObjectID, err := bson.ObjectIDFromHex(profileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	userID, _ := c.Get("userId")
	userObjectID, err := bson.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user *models.User
	err = h.RedisService.GetCachedUser(profileObjectID.Hex(), &user)
	if err != nil {
		user, err = h.AuthService.GetUserByID(profileObjectID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}
		h.RedisService.CacheUserProfile(profileObjectID.Hex(), *user)
	}

	profileResponse := models.ProfileResponse{
		UserSummary: models.UserSummary{
			ID:       user.ID,
			Username: user.Username,
			Picture:  user.Picture,
		},
		Bio:          user.Bio,
		CreatedAt:    user.CreatedAt,
		IsOnline:     h.ChatService.GetUserStatus(user.ID.Hex()),
		DirectChatID: nil,
	}

	if userObjectID != profileObjectID {
		var userIDs = []bson.ObjectID{userObjectID, profileObjectID}
		slices.SortFunc(userIDs, func(a, b bson.ObjectID) int {
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
		c.Set("username", req.Username)
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

	updatedUser, err := h.AuthService.UpdateUser(userObjectID, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user record"})
		return
	}

	h.RedisService.CacheUserProfile(userObjectID.Hex(), *updatedUser)

	err = h.ChatService.UpdateClient(userObjectID.Hex(), req.Username, req.Picture)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user record"})
		return
	}

	user := models.UserSummary{
		ID:       updatedUser.ID,
		Username: updatedUser.Username,
		Picture:  updatedUser.Picture,
	}

	c.JSON(http.StatusOK, user)
}
