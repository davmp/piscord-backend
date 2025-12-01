package services

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoService struct {
	client   *mongo.Client
	database *mongo.Database
	uri      string
}

func NewMongoService(uri string) *MongoService {
	return &MongoService{
		uri: uri,
	}
}

func (m *MongoService) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(m.uri)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return err
	}

	m.client = client
	m.database = client.Database("piscord")

	// m.createIndexes()

	return nil
}

func (m *MongoService) Disconnect() error {
	if m.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return m.client.Disconnect(ctx)
	}
	return nil
}

func (m *MongoService) GetDatabase() *mongo.Database {
	return m.database
}

func (m *MongoService) GetCollection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

// func (m *MongoService) createIndexes() {
// 	ctx := context.Background()

// 	// Users collection indexes
// 	usersCollection := m.GetCollection("users")
// 	usersCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
// 		Keys:    map[string]any{"username": 1},
// 		Options: options.Index().SetUnique(true),
// 	})

// 	// Messages collection indexes
// 	messagesCollection := m.GetCollection("messages")
// 	messagesCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
// 		Keys: map[string]any{"room_id": 1, "created_at": -1},
// 	})
// 	messagesCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
// 		Keys: map[string]any{"user_id": 1},
// 	})

// 	// Rooms collection indexes
// 	roomsCollection := m.GetCollection("rooms")
// 	roomsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
// 		Keys: map[string]any{"created_by": 1},
// 	})
// 	roomsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
// 		Keys: map[string]any{"members": 1},
// 	})

// 	// Notifications collection indexes
// 	notificationsCollection := m.GetCollection("notifications")
// 	notificationsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
// 		Keys: map[string]any{"user_id": 1, "created_at": -1},
// 	})
// }
