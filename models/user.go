package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID        primitive.ObjectID `json:"id"`
	Username  string             `json:"username" binding:"required,min=3,max=30"`
	Password  string             `json:"password,omitempty" binding:"required,min=4"`
	Picture   string             `json:"picture,omitempty"`
	Bio       string             `json:"bio,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type UserResponse struct {
	ID        primitive.ObjectID `json:"id"`
	Username  string             `json:"username"`
	Picture   string             `json:"picture,omitempty"`
	Bio       string             `json:"bio,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
}

type ProfileResponse struct {
	UserResponse
	IsOnline     bool                `json:"is_online"`
	DirectChatID *primitive.ObjectID `json:"direct_chat_id,omitempty"`
}

type UpdateProfileRequest struct {
	Username string `json:"username,omitempty" binding:"omitempty,min=3,max=30"`
	Password string `json:"password,omitempty" binding:"omitempty,min=6"`
	Picture  string `json:"picture,omitempty"`
	Bio      string `json:"bio,omitempty"`
}

type UserRegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30"`
	Password string `json:"password" binding:"required,min=6"`
	Picture  string `json:"picture,omitempty"`
}

type AuthRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string        `json:"token"`
	User  *UserResponse `json:"user"`
}
