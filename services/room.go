package services

import (
	"context"
	"piscord-backend/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type RoomService struct {
	AuthService  *AuthService
	MongoService *MongoService
}

func NewRoomService(authService *AuthService, mongoService *MongoService) *RoomService {
	return &RoomService{
		AuthService:  authService,
		MongoService: mongoService,
	}
}

func (rs *RoomService) GetRoomById(roomID primitive.ObjectID) (*models.Room, error) {
	room := models.Room{}
	err := rs.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{"_id": roomID, "is_active": true}).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (rs *RoomService) GetIsUserMember(roomID, userID primitive.ObjectID) error {
	count, err := rs.MongoService.GetCollection("rooms").CountDocuments(context.Background(), bson.M{"_id": roomID, "members": userID, "is_active": true})

	if err != nil || count == 0 {
		return err
	}
	return nil
}

func (rs *RoomService) GetRoomByDirectKey(directKey string) (*models.Room, error) {
	room := models.Room{}
	err := rs.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{"direct_key": directKey, "is_active": true}).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}
