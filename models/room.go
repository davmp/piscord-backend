package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Room struct {
	ID          primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	Name        string               `bson:"name" json:"name" binding:"required"`
	Description string               `bson:"description,omitempty" json:"description,omitempty"`
	Type        string               `bson:"type" json:"type" binding:"required,oneof=group direct"`
	CreatedBy   primitive.ObjectID   `bson:"created_by" json:"created_by"`
	Members     []primitive.ObjectID `bson:"members" json:"members"`
	Admins      []primitive.ObjectID `bson:"admins,omitempty" json:"admins,omitempty"`
	MaxMembers  int                  `bson:"max_members,omitempty" json:"max_members,omitempty"`
	IsActive    bool                 `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time            `bson:"updated_at" json:"updated_at"`
	DirectKey   string               `bson:"direct_key,omitempty" json:"direct_key,omitempty"`
}

type CreateRoomRequest struct {
	Name           string   `json:"name" binding:"required,min=1,max=50"`
	Description    string   `json:"description,omitempty"`
	Type           string   `json:"type" binding:"required,oneof=group direct"`
	MaxMembers     int      `json:"max_members,omitempty"`
	ParticipantIDs []string `json:"participant_ids,omitempty"`
}

type RoomResponse struct {
	ID          primitive.ObjectID `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Type        string             `json:"type"`
	CreatedBy   primitive.ObjectID `json:"created_by"`
	MemberCount int                `json:"member_count"`
	MaxMembers  int                `json:"max_members,omitempty"`
	IsActive    bool               `json:"is_active"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`

	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar,omitempty"`
}

type RoomMember struct {
	UserID   primitive.ObjectID `bson:"user_id" json:"user_id"`
	Username string             `bson:"username" json:"username"`
	Avatar   string             `bson:"avatar,omitempty" json:"avatar,omitempty"`
	IsOnline bool               `bson:"is_online" json:"is_online"`
	JoinedAt time.Time          `bson:"joined_at" json:"joined_at"`
}
