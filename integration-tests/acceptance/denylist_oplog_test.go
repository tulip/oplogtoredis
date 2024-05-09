package main

import (
	"context"
	"os"
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"
	"go.mongodb.org/mongo-driver/bson"
)

func TestDenyOplog(t *testing.T) {
	baseURL := os.Getenv("OTR_URL")

	harness := startHarness()
	defer harness.stop()

	_, err := harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "id1",
		"f":   "1",
	})
	if err != nil {
		panic(err)
	}

	expectedMessage1 := helpers.OTRMessage{
		Event: "i",
		Document: map[string]interface{}{
			"_id": "id1",
		},
		Fields: []string{"_id", "f"},
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":      {expectedMessage1},
		"tests.Foo::id1": {expectedMessage1},
	})

	helpers.DoRequest("PUT", baseURL, "/denylist/tests", t, 201)

	_, err = harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "id2",
		"g":   "2",
	})
	if err != nil {
		panic(err)
	}

	// second message should not have been received, since it got denied
	harness.verify(t, map[string][]helpers.OTRMessage{})

	helpers.DoRequest("DELETE", baseURL, "/denylist/tests", t, 204)

	_, err = harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "id3",
		"h":   "3",
	})
	if err != nil {
		panic(err)
	}

	expectedMessage3 := helpers.OTRMessage{
		Event: "i",
		Document: map[string]interface{}{
			"_id": "id3",
		},
		Fields: []string{"_id", "h"},
	}

	// back to normal now that the deny rule is gone
	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":      {expectedMessage3},
		"tests.Foo::id3": {expectedMessage3},
	})
}
