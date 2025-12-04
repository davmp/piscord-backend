package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Room struct {
	ID          bson.ObjectID   `json:"id" bson:"_id,omitempty"`
	Name        string          `json:"name,omitempty" bson:"name,omitempty" binding:"max=50"`
	Description string          `json:"description,omitempty" bson:"description,omitempty"`
	Type        string          `json:"type" bson:"type" binding:"required,oneof=private public direct"`
	Picture     string          `json:"picture,omitempty" bson:"picture,omitempty"`
	OwnerID     bson.ObjectID   `json:"ownerId" bson:"ownerId"`
	Members     []bson.ObjectID `json:"members" bson:"members"`
	Admins      []bson.ObjectID `json:"admins,omitempty" bson:"admins,omitempty"`
	MaxMembers  int             `json:"maxMembers,omitempty" bson:"maxMembers,omitempty"`
	IsActive    bool            `json:"isActive" bson:"isActive"`
	CreatedAt   time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt" bson:"updatedAt"`
	DirectKey   string          `json:"directKey,omitempty" bson:"directKey,omitempty"`
}

type CreateRoomRequest struct {
	Name        string    `json:"name,omitempty" binding:"max=50"`
	Description string    `json:"description,omitempty"`
	Picture     string    `json:"picture,omitempty"`
	Type        string    `json:"type" binding:"required,oneof=private public direct"`
	Members     []string  `json:"members" bson:"members"`
	Admins      []string  `json:"admins" bson:"admins"`
	MaxMembers  int       `json:"maxMembers" binding:"max=100"`
	IsActive    bool      `json:"isActive" bson:"isActive"`
	DirectKey   string    `json:"directKey,omitempty" bson:"directKey,omitempty"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" bson:"updatedAt"`
}

type UpdateRoomRequest struct {
	ID     string `json:"id" bson:"id"`
	UserID string `json:"userId" bson:"userId"`

	Name          string    `json:"name,omitempty" binding:"max=50"`
	Description   string    `json:"description,omitempty"`
	Picture       string    `json:"picture,omitempty"`
	Type          string    `json:"type" binding:"required,oneof=private public direct"`
	OwnerID       string    `json:"ownerId" bson:"ownerId"`
	RemoveMembers []string  `json:"removeMembers,omitempty"`
	AddMembers    []string  `json:"addMembers,omitempty"`
	MaxMembers    int       `json:"maxMembers,omitempty" binding:"max=100"`
	UpdatedAt     time.Time `json:"updatedAt" bson:"updatedAt"`
}

type RoomPreview struct {
	ID          bson.ObjectID   `json:"id"`
	DisplayName string          `json:"displayName"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"`
	Picture     string          `json:"picture,omitempty"`
	LastMessage *MessagePreview `json:"lastMessage"`
}

type RoomDetails struct {
	ID          bson.ObjectID `json:"id"`
	DisplayName string        `json:"displayName"`
	Description string        `json:"description,omitempty"`
	Type        string        `json:"type"`
	Picture     string        `json:"picture,omitempty"`
	MaxMembers  int           `json:"maxMembers,omitempty"`
	IsActive    bool          `json:"isActive"`
	IsAdmin     bool          `json:"isAdmin"`
	Members     []RoomMember  `json:"members"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

type PublicRoom struct {
	RoomPreview
	IsMember bool `json:"isMember"`
}

type RoomMember struct {
	UserID   bson.ObjectID `json:"userId"`
	Username string        `json:"username"`
	Picture  string        `json:"picture,omitempty"`
	IsAdmin  bool          `json:"isAdmin"`
	IsOnline bool          `json:"isOnline"`
	IsMe     bool          `json:"isMe"`
}
