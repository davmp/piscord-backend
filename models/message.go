package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID        primitive.ObjectID  `json:"id"`
	RoomID    primitive.ObjectID  `json:"room_id"`
	UserID    primitive.ObjectID  `json:"user_id"`
	Content   string              `json:"content"`
	Type      string              `json:"type"` // "text", "image", "file", "system"
	FileURL   string              `json:"file_url,omitempty"`
	ReplyTo   *primitive.ObjectID `json:"reply_to,omitempty"`
	IsEdited  bool                `json:"is_edited"`
	IsDeleted bool                `json:"is_deleted"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type SendMessageRequest struct {
	RoomID  string `json:"room_id" binding:"required"`
	Content string `json:"content" binding:"required"`
	Type    string `json:"type,omitempty"`
	ReplyTo string `json:"reply_to,omitempty"`
}

type MessageResponse struct {
	ID           primitive.ObjectID      `json:"id"`
	RoomID       primitive.ObjectID      `json:"room_id"`
	UserID       primitive.ObjectID      `json:"user_id"`
	Username     string                  `json:"username"`
	Picture      string                  `json:"picture,omitempty"`
	Content      string                  `json:"content"`
	Type         string                  `json:"type"`
	FileURL      string                  `json:"file_url,omitempty"`
	IsOwnMessage bool                    `json:"is_own_message"`
	ReplyTo      *MessagePreviewResponse `json:"reply_to,omitempty"`
	IsEdited     bool                    `json:"is_edited"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
}

type MessagePreviewResponse struct {
	ID        primitive.ObjectID `json:"id"`
	Username  string             `json:"username"`
	Content   string             `json:"content"`
	Picture   string             `json:"picture,omitempty"`
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
