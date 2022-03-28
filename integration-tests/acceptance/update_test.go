package main

import (
	"context"
	"testing"

	"github.com/vlasky/oplogtoredis/integration-tests/helpers"

	"go.mongodb.org/mongo-driver/bson"
)

// Basic test of an update
func TestUpdate(t *testing.T) {
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

	_, err = harness.mongoClient.Collection("Foo").UpdateByID(context.Background(), "someid", bson.M{
		"$set": bson.M{
			"hello": "new",
			"world": "new",
		},
	})
	if err != nil {
		panic(err)
	}

	expectedMessage := helpers.OTRMessage{
		Event: "u",
		Document: map[string]interface{}{
			"_id": "someid",
		},
		Fields: []string{"hello", "world"},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":         {expectedMessage},
		"tests.Foo::someid": {expectedMessage},
	})
}

func TestUpdateReplace(t *testing.T) {
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

	_, err = harness.mongoClient.Collection("Foo").ReplaceOne(context.Background(), bson.M{"_id": "someid"}, bson.M{
		"world": "new",
	})
	if err != nil {
		panic(err)
	}

	expectedMessage := helpers.OTRMessage{
		Event: "u",
		Document: map[string]interface{}{
			"_id": "someid",
		},
		Fields: []string{"_id", "world"},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":         {expectedMessage},
		"tests.Foo::someid": {expectedMessage},
	})
}

func TestUpdateArraySet(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	// Initialize test data
	_, err := harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "someid",
		"hello": []bson.M{
			{"value": 10},
			{"value": 20},
			{"value": 30},
			{"value": 40},
		},
	})
	if err != nil {
		panic(err)
	}

	_, err = harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "someid2",
		"hello": []bson.M{
			{"value": 10},
			{"value": 10},
			{"value": 20},
			{"value": 30},
		},
	})
	if err != nil {
		panic(err)
	}

	harness.resetMessages()

	// Run an update that increments the first value more than 15
	_, err = harness.mongoClient.Collection("Foo").UpdateMany(context.Background(), bson.M{
		"hello.value": bson.M{"$gt": 15},
	}, bson.M{
		"$inc": bson.M{
			"hello.$.value": 1,
		},
	})
	if err != nil {
		panic(err)
	}

	expectedMessage1 := helpers.OTRMessage{
		Event: "u",
		Document: map[string]interface{}{
			"_id": "someid",
		},
		Fields: []string{"hello.1.value"},
	}
	expectedMessage2 := helpers.OTRMessage{
		Event: "u",
		Document: map[string]interface{}{
			"_id": "someid2",
		},
		Fields: []string{"hello.2.value"},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":          {expectedMessage1, expectedMessage2},
		"tests.Foo::someid":  {expectedMessage1},
		"tests.Foo::someid2": {expectedMessage2},
	})
}

func TestUpdateArrayPush(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	// Initialize test data
	_, err := harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "someid",
		"hello": []bson.M{
			{"value": 10},
			{"value": 20},
			{"value": 30},
			{"value": 40},
		},
	})
	if err != nil {
		panic(err)
	}

	harness.resetMessages()

	// Run an update that increments the first value more than 15
	_, err = harness.mongoClient.Collection("Foo").UpdateMany(context.Background(), bson.M{
		"hello.value": bson.M{"$gt": 15},
	}, bson.M{
		"$push": bson.M{
			"hello": bson.M{
				"$each":     []int{25},
				"$position": 1,
			},
		},
	})
	if err != nil {
		panic(err)
	}

	expectedMessage := helpers.OTRMessage{
		Event: "u",
		Document: map[string]interface{}{
			"_id": "someid",
		},
		Fields: []string{"hello"},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":         {expectedMessage},
		"tests.Foo::someid": {expectedMessage},
	})
}
