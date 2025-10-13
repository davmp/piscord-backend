package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username  string             `bson:"username" json:"username" binding:"required,min=3,max=30"`
	Password  string             `bson:"password" json:"password,omitempty" binding:"required,min=4"`
	Picture   string             `bson:"picture,omitempty" json:"picture,omitempty"`
	Bio       string             `bson:"bio,omitempty" json:"bio,omitempty"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type UserResponse struct {
	ID        primitive.ObjectID `json:"id"`
	Username  string             `json:"username"`
	Picture   string             `json:"picture,omitempty"`
	Bio       string             `json:"bio,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	IsOnline  bool               `json:"is_online"`
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
