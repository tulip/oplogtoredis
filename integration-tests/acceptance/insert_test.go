package main

import (
	"context"
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"

	"go.mongodb.org/mongo-driver/bson"
)

// Basic test of an insert
func TestInsert(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	_, err := harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id":   "someid",
		"hello": "world",
	})
	if err != nil {
		panic(err)
	}

	expectedMessage := helpers.OTRMessage{
		Event: "i",
		Document: map[string]interface{}{
			"_id": "someid",
		},
		Fields: []string{"_id", "hello"},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":         {expectedMessage},
		"tests.Foo::someid": {expectedMessage},
	})
}
