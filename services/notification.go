package services

import (
	"context"
	"piscord-backend/models"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type NotificationService struct {
	MongoService *MongoService
}

func NewNotificationService(mongoService *MongoService) *NotificationService {
	return &NotificationService{
		MongoService: mongoService,
	}
}

func (ns *NotificationService) GetMyNotifications(page, limit int, userObjectID primitive.ObjectID) ([]models.NotificationResponse, error) {
	notificationsCollection := ns.MongoService.GetCollection("notifications")
	filter := bson.M{
		"user_id": userObjectID,
	}

	skip := (page - 1) * limit

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := notificationsCollection.Find(context.Background(), filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	notifications := []models.NotificationResponse{}

	for cursor.Next(context.Background()) {
		var notification models.Notification
		if err := cursor.Decode(&notification); err != nil {
			continue
		}

		notificationResponse := ns.responseNotification(notification)
		notifications = append(notifications, notificationResponse)
	}

	return notifications, nil
}

func (ns *NotificationService) GetUnreadNotificationCount(userObjectID primitive.ObjectID) (int64, error) {
	notificationsCollection := ns.MongoService.GetCollection("notifications")
	count, err := notificationsCollection.CountDocuments(context.Background(), bson.M{
		"user_id": userObjectID,
		"read_at": nil,
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (ns *NotificationService) CreateNotification(notification *models.Notification) error {
	_, err := ns.MongoService.GetCollection("notifications").InsertOne(context.Background(), notification)
	return err
}

func (ns *NotificationService) GetNotificationsByUserID(userID string) ([]models.Notification, error) {
	var notifications []models.Notification
	cursor, err := ns.MongoService.GetCollection("notifications").Find(context.Background(), bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	for cursor.Next(context.Background()) {
		var notification models.Notification
		if err := cursor.Decode(&notification); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, nil
}

func (ns *NotificationService) MarkNotificationAsRead(notificationID, userID primitive.ObjectID) error {
	_, err := ns.MongoService.GetCollection("notifications").UpdateOne(
		context.Background(),
		bson.M{
			"_id":     notificationID,
			"user_id": userID,
		},
		bson.M{
			"$set": bson.M{
				"read_at": time.Now(),
				"content": "Você recebeu uma notificação",
			},
		},
	)
	return err
}

func (ns *NotificationService) MarkAllNotificationsAsRead(userID primitive.ObjectID) error {
	_, err := ns.MongoService.GetCollection("notifications").UpdateMany(
		context.Background(),
		bson.M{
			"user_id": userID,
		},
		bson.M{
			"$set": bson.M{
				"read_at": time.Now(),
			},
		},
	)
	return err
}

func (ns *NotificationService) DeleteNotification(notificationID, userID primitive.ObjectID) error {
	_, err := ns.MongoService.GetCollection("notifications").DeleteOne(context.Background(), bson.M{"_id": notificationID, "user_id": userID})
	return err
}

func (ns *NotificationService) DeleteAllNotifications(userID primitive.ObjectID) error {
	_, err := ns.MongoService.GetCollection("notifications").DeleteMany(context.Background(), bson.M{"user_id": userID})
	return err
}

func (ns *NotificationService) responseNotification(notification models.Notification) models.NotificationResponse {
	notificationResponse := models.NotificationResponse{
		ID:        notification.ID,
		Content:   notification.Content,
		Type:      notification.Type,
		ReadAt:    notification.ReadAt,
		CreatedAt: notification.CreatedAt,
	}

	switch notification.Type {
	case models.NotificationTypeNewMessage:
		var message models.Message
		if err := ns.MongoService.GetCollection("messages").FindOne(context.Background(), bson.M{"_id": notification.ObjectID}).Decode(&message); err == nil {
			notificationResponse.Title = "Nova mensagem de " + message.Username
			notificationResponse.Link = "/chat/" + message.RoomID.Hex()
			notificationResponse.Picture = message.Picture

			var room models.Room
			if err := ns.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{"_id": message.RoomID}).Decode(&message); err == nil {
				notificationResponse.Title = "Nova mensagem em " + room.Name

				var content string
				if len(message.Content) > 50 {
					content = strings.ReplaceAll(message.Content[:50], "\n", " ")
				} else {
					content = strings.ReplaceAll(message.Content, "\n", " ")
				}
				notificationResponse.Content = message.Username + ": " + content
			}
		}
	case models.NotificationTypeUserJoined:
		var room models.Room
		if err := ns.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{"_id": notification.ObjectID}).Decode(&room); err == nil {
			notificationResponse.Title = notification.Content + " entrou em " + room.Name
			notificationResponse.Link = "/chat/" + room.ID.Hex()
			notificationResponse.Picture = room.Picture
			notificationResponse.Content = "Um novo usuário" + notification.Content + " entrou na sala" + room.Name
		}
	case models.NotificationTypeUserLeft:
		var room models.Room
		if err := ns.MongoService.GetCollection("rooms").FindOne(context.Background(), bson.M{"_id": notification.ObjectID}).Decode(&room); err == nil {
			notificationResponse.Title = notification.Content + " saiu de " + room.Name
			notificationResponse.Link = "/chat/" + room.ID.Hex()
			notificationResponse.Picture = room.Picture
			notificationResponse.Content = notification.Content + " saiu da sala" + room.Name
		}
	}

	return notificationResponse
}
