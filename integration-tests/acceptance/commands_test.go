package main

import (
	"context"
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// We should ignore ensureIndex
func TestAddIndex(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	_, err := harness.mongoClient.Collection("Foo").Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "name", Value: 1}},
	})
	if err != nil {
		panic(err)
	}

	harness.verify(t, map[string][]helpers.OTRMessage{})
}

// We should ignore commands like dropCollection
func TestDropCollection(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	_, err := harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id":   "someid",
		"hello": "world",
	})
	if err != nil {
		panic(err)
	}
	harness.resetMessages()

	err = harness.mongoClient.Collection("Foo").Drop(context.Background())
	if err != nil {
		panic(err)
	}

	harness.verify(t, map[string][]helpers.OTRMessage{})
}
