package services

import (
	"context"
	"piscord-backend/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type NotificationService struct {
	AuthService  *AuthService
	MongoService *MongoService
	RedisService *RedisService
}

func NewNotificationService(authService *AuthService, mongoService *MongoService, redisService *RedisService) *NotificationService {
	return &NotificationService{
		AuthService:  authService,
		MongoService: mongoService,
		RedisService: redisService,
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
	return ns.RedisService.Publish("notification", "notification.create", notification)
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
	return ns.RedisService.Publish("notification", "notification.read", map[string]primitive.ObjectID{
		"notification_id": notificationID,
		"user_id":         userID,
	})
}

func (ns *NotificationService) MarkAllNotificationsAsRead(userID primitive.ObjectID) error {
	return ns.RedisService.Publish("notification", "notification.read_all", map[string]primitive.ObjectID{
		"user_id": userID,
	})
}

func (ns *NotificationService) DeleteNotification(notificationID, userID primitive.ObjectID) error {
	return ns.RedisService.Publish("notification", "notification.delete", map[string]primitive.ObjectID{
		"notification_id": notificationID,
		"user_id":         userID,
	})
}

func (ns *NotificationService) DeleteAllNotifications(userID primitive.ObjectID) error {
	return ns.RedisService.Publish("notification", "notification.delete_all", map[string]primitive.ObjectID{
		"user_id": userID,
	})
}

func (ns *NotificationService) responseNotification(notification models.Notification) models.NotificationResponse {
	notificationResponse := models.NotificationResponse{
		ID:        notification.ID,
		Title:     notification.Title,
		Link:      notification.Link,
		Picture:   notification.Picture,
		Body:      notification.Body,
		Type:      notification.Type,
		IsRead:    notification.IsRead,
		CreatedAt: notification.CreatedAt,
	}

	return notificationResponse
}
