package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RoomID    primitive.ObjectID `bson:"room_id" json:"room_id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Username  string             `bson:"username" json:"username"`
	Picture   string             `bson:"picture,omitempty" json:"picture,omitempty"`
	Content   string             `bson:"content" json:"content"`
	Type      string             `bson:"type" json:"type"` // "text", "image", "file", "system"
	FileURL   string             `bson:"file_url,omitempty" json:"file_url,omitempty"`
	ReplyTo   primitive.ObjectID `bson:"reply_to,omitempty" json:"reply_to,omitempty"`
	IsEdited  bool               `bson:"is_edited" json:"is_edited"`
	IsDeleted bool               `bson:"is_deleted" json:"is_deleted"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type SendMessageRequest struct {
	RoomID  string `json:"room_id" binding:"required"`
	Content string `json:"content" binding:"required"`
	Type    string `json:"type,omitempty"`
	ReplyTo string `json:"reply_to,omitempty"`
}

type MessageResponse struct {
	ID           primitive.ObjectID `json:"id"`
	RoomID       primitive.ObjectID `json:"room_id"`
	UserID       primitive.ObjectID `json:"user_id"`
	Username     string             `json:"username"`
	Picture      string             `json:"picture,omitempty"`
	Content      string             `json:"content"`
	Type         string             `json:"type"`
	FileURL      string             `json:"file_url,omitempty"`
	IsOwnMessage bool               `json:"is_own_message"`
	ReplyTo      primitive.ObjectID `json:"reply_to,omitempty"`
	IsEdited     bool               `json:"is_edited"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type MessagePreviewResponse struct {
	ID        primitive.ObjectID `json:"id"`
	Username  string             `json:"username"`
	Content   string             `json:"content"`
	CreatedAt time.Time          `json:"created_at"`
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
