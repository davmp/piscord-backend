package services

import (
	"context"
	"piscord-backend/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type AuthService struct {
	MongoService *MongoService
}

func NewAuthService(mongoService *MongoService) *AuthService {
	return &AuthService{
		MongoService: mongoService,
	}
}

func (as *AuthService) GetUserByID(userID string) (*models.User, error) {
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
	_, err := as.MongoService.GetCollection("users").InsertOne(context.Background(), user)
	return err
}

func (as *AuthService) UpdateUser(userID string, update bson.M) error {
	_, err := as.MongoService.GetCollection("users").UpdateOne(context.Background(), bson.M{"_id": userID}, bson.M{"$set": update})
	return err
}
