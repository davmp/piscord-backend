package services

import (
	"context"
	"encoding/json"
	"fmt"
	"piscord-backend/models"

	"github.com/redis/go-redis/v9"
)

type RedisService struct {
	Client *redis.Client
}

func NewRedisService(redisURI string) *RedisService {
	opt, err := redis.ParseURL(redisURI)
	if err != nil {
		panic(err)
	}

	client := redis.NewClient(opt)

	return &RedisService{
		Client: client,
	}
}

func (rs *RedisService) Publish(stream string, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	err = rs.Client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{
			"type":    eventType,
			"payload": data,
		},
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to publish to stream %s: %w", stream, err)
	}

	return nil
}

func (rs *RedisService) AddUserToRoom(userID, roomID string) error {
	key := fmt.Sprintf("room.%s:members", roomID)
	return rs.Client.SAdd(context.Background(), key, userID).Err()
}

func (rs *RedisService) RemoveUserFromRoom(userID, roomID string) error {
	key := fmt.Sprintf("room.%s:members", roomID)
	return rs.Client.SRem(context.Background(), key, userID).Err()
}

func (rs *RedisService) IsUserInRoom(userID, roomID string) (bool, error) {
	key := fmt.Sprintf("room.%s:members", roomID)
	return rs.Client.SIsMember(context.Background(), key, userID).Result()
}

func (rs *RedisService) CacheUserProfile(userID string, user models.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}
	key := fmt.Sprintf("user:%s:profile", userID)
	return rs.Client.Set(context.Background(), key, data, 0).Err() // no expiration
}

func (rs *RedisService) GetCachedUser(userID string, target any) error {
	key := fmt.Sprintf("user:%s:profile", userID)
	data, err := rs.Client.Get(context.Background(), key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func (rs *RedisService) InvalidateUserCache(userID string) error {
	key := fmt.Sprintf("user:%s:profile", userID)
	return rs.Client.Del(context.Background(), key).Err()
}

// User Presence / Registration
func (rs *RedisService) SetUserOnline(userID string) error {
	return rs.Client.SAdd(context.Background(), "online_users", userID).Err()
}

func (rs *RedisService) SetUserOffline(userID string) error {
	return rs.Client.SRem(context.Background(), "online_users", userID).Err()
}

func (rs *RedisService) GetOnlineUsers() ([]string, error) {
	return rs.Client.SMembers(context.Background(), "online_users").Result()
}

func (rs *RedisService) IsUserOnline(userID string) (bool, error) {
	return rs.Client.SIsMember(context.Background(), "online_users", userID).Result()
}
