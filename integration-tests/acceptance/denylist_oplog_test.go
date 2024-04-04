package main

import (
	"context"
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"
	"go.mongodb.org/mongo-driver/bson"
)

func TestDenyOplog(t *testing.T) {
	// TODO

	// start harness
	// send message
	// confirm message arrived

	// create denylist
	// send message
	// confirm message did NOT arrive

	// remove denylist
	// confirm message STILL did not arrive

	// send message
	// confirm message arrived

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

	ruleID := doRequest("PUT", "/denylist", map[string]interface{}{
		"keys":  "ns",
		"regex": "^tests.Foo$",
	}, t, 201).(string)

	_, err = harness.mongoClient.Collection("Foo").InsertOne(context.Background(), bson.M{
		"_id": "id2",
		"g":   "2",
	})
	if err != nil {
		panic(err)
	}

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":      {},
		"tests.Foo::id2": {},
	})

	doRequest("DELETE", "/denylist/"+ruleID, map[string]interface{}{}, t, 204)

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

	harness.verify(t, map[string][]helpers.OTRMessage{
		"tests.Foo":      {expectedMessage3},
		"tests.Foo::id3": {expectedMessage3},
	})

}
