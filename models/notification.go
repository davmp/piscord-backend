package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationType string

const (
	NotificationTypeNewMessage          NotificationType = "NEW_MESSAGE"
	NotificationTypeUserJoined          NotificationType = "USER_JOINED"
	NotificationTypeUserLeft            NotificationType = "USER_LEFT"
	NotificationTypeFriendRequest       NotificationType = "FRIEND_REQUEST"
	NotificationTypeFriendRequestAccept NotificationType = "FRIEND_REQUEST_ACCEPTED"
	NotificationTypeRoomInvite          NotificationType = "ROOM_INVITE"
	NotificationTypeMention             NotificationType = "MENTION"
	NotificationTypeSystem              NotificationType = "SYSTEM"
)

type Notification struct {
	ID        primitive.ObjectID `json:"id"`
	UserID    primitive.ObjectID `json:"user_id"`
	Title     string             `json:"title"`
	Body      string             `json:"Body"`
	Link      string             `json:"link,omitempty"`
	Picture   string             `json:"picture,omitempty"`
	Type      NotificationType   `json:"type"`
	IsRead    bool               `json:"is_read"`
	CreatedAt time.Time          `json:"created_at"`
}

type NotificationResponse struct {
	ID        primitive.ObjectID `json:"id"`
	Title     string             `json:"title"`
	Link      string             `json:"link"`
	Picture   string             `json:"picture"`
	Body      string             `json:"Body"`
	Type      NotificationType   `json:"type"`
	IsRead    bool               `json:"is_read"`
	CreatedAt time.Time          `json:"created_at"`
}
