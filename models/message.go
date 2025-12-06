package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Message struct {
	ID        bson.ObjectID   `json:"id" bson:"_id"`
	RoomID    bson.ObjectID   `json:"roomId" bson:"roomId"`
	Author    UserSummary     `json:"author" bson:"author"`
	Content   string          `json:"content" bson:"content"`
	FileURL   string          `json:"fileUrl,omitempty" bson:"fileUrl,omitempty"`
	ReplyTo   *MessagePreview `json:"replyTo,omitempty" bson:"replyTo,omitempty"`
	IsDeleted bool            `json:"isDeleted" bson:"isDeleted"`
	EditedAt  *time.Time      `json:"editedAt" bson:"editedAt"`
	CreatedAt time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt" bson:"updatedAt"`
}

type MessageSend struct {
	RoomID  bson.ObjectID   `json:"roomId"`
	Author  UserSummary     `json:"author"`
	Content string          `json:"content"`
	FileURL string          `json:"fileUrl,omitempty"`
	ReplyTo *MessagePreview `json:"replyTo,omitempty"`
	SentAt  time.Time       `json:"sentAt"`
}

type MessageUpdate struct {
	ID        bson.ObjectID `json:"id"`
	UserID    bson.ObjectID `json:"userId"`
	Content   string        `json:"content"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type MessagePreview struct {
	ID        bson.ObjectID `json:"id" bson:"_id"`
	Content   string        `json:"content" bson:"content"`
	Author    UserSummary   `json:"author" bson:"author"`
	CreatedAt time.Time     `json:"createdAt" bson:"createdAt"`
}

type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type WSResponse struct {
	Type    string `json:"type"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
