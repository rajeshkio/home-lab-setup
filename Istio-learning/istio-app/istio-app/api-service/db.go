package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	dbClient     *mongo.Client
	dbCollection *mongo.Collection
)

func InitDB() {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB connection error: %v", err)
	}

	dbClient = client
	db := client.Database("istiodemo")
	dbCollection = db.Collection("records")

	log.Println("Connected to MongoDB")
}

func InsertRecord(record Record) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := dbCollection.InsertOne(ctx, bson.M{
		"name":      record.Name,
		"value":     record.Value,
		"timestamp": record.Timestamp,
	})
	return err
}
