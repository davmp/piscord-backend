package services

import (
	"context"
	"piscord-backend/models"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type RoomService struct {
	MongoService *MongoService
	RedisService *RedisService
}

func NewRoomService(mongoService *MongoService, redisService *RedisService) *RoomService {
	return &RoomService{
		MongoService: mongoService,
		RedisService: redisService,
	}
}

func (rs *RoomService) GetRoomByID(roomID bson.ObjectID) (*models.Room, error) {
	var room models.Room
	err := rs.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{"_id": roomID}).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (rs *RoomService) GetRoomsByUserID(userID bson.ObjectID) ([]*models.Room, error) {
	var rooms []*models.Room
	cursor, err := rs.MongoService.GetCollection("rooms").Find(context.Background(), bson.M{"members": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &rooms); err != nil {
		return nil, err
	}

	return rooms, nil
}

func (rs *RoomService) GetMembersByRoom(userID bson.ObjectID, roomID bson.ObjectID) ([]models.RoomMember, error) {
	var room models.Room
	err := rs.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{
		"_id":      roomID,
		"isActive": true,
	}).Decode(&room)
	if err != nil {
		return nil, err
	}

	var members []models.RoomMember
	userCollection := rs.MongoService.GetCollection("users")

	cursor, err := userCollection.Find(context.Background(), bson.M{
		"_id": bson.M{"$in": room.Members},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var user models.User
		if err := cursor.Decode(&user); err != nil {
			continue
		}

		isOnline := false
		if isOnline, err = rs.RedisService.IsUserOnline(user.ID.Hex()); err != nil {
			continue
		}

		members = append(members, models.RoomMember{
			UserID:   user.ID,
			Username: user.Username,
			Picture:  user.Picture,
			IsAdmin:  slices.Contains(room.Admins, user.ID),
			IsOnline: isOnline,
			IsMe:     user.ID == userID,
		})
	}

	return members, nil
}

func (rs *RoomService) CreateRoom(room *models.Room) error {
	err := rs.RedisService.Publish("room", "room.create", room)
	if err != nil {
		return err
	}

	rs.RedisService.AddUserToRoom(room.ID.Hex(), room.OwnerID.Hex())
	return nil
}

func (rs *RoomService) UpdateRoom(roomID bson.ObjectID, data map[string]any) (*models.Room, error) {
	room, err := rs.GetRoomByID(roomID)
	if err != nil {
		return nil, err
	}

	data["id"] = roomID

	if val, ok := data["name"].(string); ok {
		room.Name = val
	}
	if val, ok := data["description"].(string); ok {
		room.Description = val
	}
	if val, ok := data["picture"].(string); ok {
		room.Picture = val
	}
	if val, ok := data["maxMembers"].(int); ok {
		room.MaxMembers = val
	}

	err = rs.RedisService.Publish("room", "room.update", data)
	if err != nil {
		return nil, err
	}

	return room, nil
}

func (rs *RoomService) AddMember(roomID bson.ObjectID, userID bson.ObjectID) error {
	return rs.RedisService.Publish("room", "room.join", map[string]any{
		"roomId": roomID,
		"userId": userID,
	})
}

func (rs *RoomService) RemoveMember(roomID bson.ObjectID, userID bson.ObjectID) error {
	return rs.RedisService.Publish("room", "room.leave", map[string]any{
		"roomId": roomID,
		"userId": userID,
	})
}

func (rs *RoomService) GetPublicRooms(limit int64, offset int64) ([]*models.Room, error) {
	var rooms []*models.Room
	opts := options.Find().SetLimit(limit).SetSkip(offset)
	cursor, err := rs.MongoService.GetCollection("rooms").Find(context.Background(), bson.M{"type": "public"}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &rooms); err != nil {
		return nil, err
	}

	return rooms, nil
}

func (rs *RoomService) GetRoomByDirectKey(key string) (*models.Room, error) {
	var room models.Room
	err := rs.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{"directKey": key}).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}
