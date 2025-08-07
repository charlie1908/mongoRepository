package mongo

import (
	"context"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	clientInstance *mongo.Client
	mongoOnce      sync.Once
)

// InitMongoClient - Singleton MongoDB Client
func InitMongoClient(uri string) *mongo.Client {
	mongoOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		clientOpts := options.Client().ApplyURI(uri)
		client, err := mongo.Connect(ctx, clientOpts)
		if err != nil {
			log.Fatalf("Mongo connection error: %v", err)
		}

		if err := client.Ping(ctx, nil); err != nil {
			log.Fatalf("Mongo ping error: %v", err)
		}

		clientInstance = client
	})

	return clientInstance
}
