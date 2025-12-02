package services

import (
	"context"
	"encoding/json"
	"fmt"

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
