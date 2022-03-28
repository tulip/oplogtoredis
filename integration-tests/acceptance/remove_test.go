package main

import (
	"context"
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"

	"go.mongodb.org/mongo-driver/bson"
)

func TestRemove(t *testing.T) {
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

	_, err = harness.mongoClient.Collection("Foo").DeleteOne(context.Background(), bson.M{"_id": "someid"})
	if err != nil {
		panic(err)
	}

	expectedMessage := helpers.OTRMessage{
		Event: "r",
		Document: map[string]interface{}{
			"_id": "someid",
		},
		Fields: []string{},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":         {expectedMessage},
		"tests.Foo::someid": {expectedMessage},
	})
}

func TestRemoveMultiple(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	// Initialize test data
	_, err := harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "someid",
		"foo": "bar",
	})
	if err != nil {
		panic(err)
	}

	_, err = harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "someid2",
		"foo": "bar",
	})
	if err != nil {
		panic(err)
	}

	harness.resetMessages()

	// Run an update that increments the first value more than 15
	_, err = harness.mongoClient.Collection("Foo").DeleteMany(context.Background(), bson.M{
		"foo": "bar",
	})
	if err != nil {
		panic(err)
	}

	expectedMessage1 := helpers.OTRMessage{
		Event: "r",
		Document: map[string]interface{}{
			"_id": "someid",
		},
		Fields: []string{},
	}
	expectedMessage2 := helpers.OTRMessage{
		Event: "r",
		Document: map[string]interface{}{
			"_id": "someid2",
		},
		Fields: []string{},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":          {expectedMessage1, expectedMessage2},
		"tests.Foo::someid":  {expectedMessage1},
		"tests.Foo::someid2": {expectedMessage2},
	})
}
