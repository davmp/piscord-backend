package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Room struct {
	ID          primitive.ObjectID   `json:"id"`
	Name        string               `json:"name,omitempty" binding:"max=50"`
	Description string               `json:"description,omitempty"`
	Type        string               `json:"type" binding:"required,oneof=private public direct"`
	Picture     string               `json:"picture,omitempty"`
	OwnerID     primitive.ObjectID   `json:"owner_id"`
	Members     []primitive.ObjectID `json:"members"`
	Admins      []primitive.ObjectID `json:"admins,omitempty"`
	MaxMembers  int                  `json:"max_members,omitempty"`
	IsActive    bool                 `json:"is_active"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	DirectKey   string               `json:"direct_key,omitempty"`
}

type CreateRoomRequest struct {
	Name           string   `json:"name,omitempty" binding:"max=50"`
	Description    string   `json:"description,omitempty"`
	Type           string   `json:"type" binding:"required,oneof=private public direct"`
	Picture        string   `json:"picture,omitempty"`
	MaxMembers     int      `json:"max_members,omitempty" binding:"max=100"`
	ParticipantIDs []string `json:"participant_ids,omitempty"`
}

type UpdateRoomRequest struct {
	Name                 string   `json:"name,omitempty" binding:"max=50"`
	Description          string   `json:"description,omitempty"`
	Picture              string   `json:"picture,omitempty"`
	MaxMembers           int      `json:"max_members,omitempty" binding:"max=100"`
	RemoveParticipantIDs []string `json:"remove_participant_ids,omitempty"`
}

type RoomResponse struct {
	ID          primitive.ObjectID      `json:"id"`
	DisplayName string                  `json:"display_name"`
	Description string                  `json:"description,omitempty"`
	Type        string                  `json:"type"`
	Picture     string                  `json:"picture,omitempty"`
	OwnerID     primitive.ObjectID      `json:"owner_id"`
	MemberCount int                     `json:"member_count"`
	MaxMembers  int                     `json:"max_members,omitempty"`
	IsActive    bool                    `json:"is_active"`
	IsAdmin     bool                    `json:"is_admin"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	LastMessage *MessagePreviewResponse `json:"last_message"`
}

type RoomDetailsResponse struct {
	ID          primitive.ObjectID `json:"id"`
	DisplayName string             `json:"display_name"`
	Description string             `json:"description,omitempty"`
	Type        string             `json:"type"`
	Picture     string             `json:"picture,omitempty"`
	MemberCount int                `json:"member_count"`
	MaxMembers  int                `json:"max_members,omitempty"`
	IsActive    bool               `json:"is_active"`
	IsAdmin     bool               `json:"is_admin"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

type PublicRoomResponse struct {
	RoomResponse
	DisplayName string `json:"display_name"`
	IsMember    bool   `json:"is_member"`
}

type RoomMember struct {
	UserID   primitive.ObjectID `json:"user_id"`
	Username string             `json:"username"`
	Picture  string             `json:"picture,omitempty"`
	IsAdmin  bool               `json:"is_admin"`
	IsOnline bool               `json:"is_online"`
	IsMe     bool               `json:"is_me"`
}
