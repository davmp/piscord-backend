package services

import (
	"context"
	"piscord-backend/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type AuthService struct {
	MongoService *MongoService
	RedisService *RedisService
}

func NewAuthService(mongoService *MongoService, redisService *RedisService) *AuthService {
	return &AuthService{
		MongoService: mongoService,
		RedisService: redisService,
	}
}

func (as *AuthService) GetUserByID(userID primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := as.MongoService.GetCollection("users").FindOne(context.Background(), bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (as *AuthService) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := as.MongoService.GetCollection("users").FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (as *AuthService) CreateUser(user *models.User) error {
	return as.RedisService.Publish("user", "user.register", user)
}

func (as *AuthService) UpdateUser(userID primitive.ObjectID, data map[string]any) (*models.User, error) {
	user, err := as.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	data["id"] = userID

	if val, ok := data["username"].(string); ok {
		user.Username = val
	}
	if val, ok := data["picture"].(string); ok {
		user.Picture = val
	}
	if val, ok := data["bio"].(string); ok {
		user.Bio = val
	}

	err = as.RedisService.Publish("user", "user.update", data)
	if err != nil {
		return nil, err
	}

	return user, nil
}
