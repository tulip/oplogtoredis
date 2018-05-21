package main

import (
	"testing"

	mgo "github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/tulip/oplogtoredis/integration-tests/helpers"
)

// We should ignore ensureIndex
func TestAddIndex(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	err := harness.mongoClient.C("Foo").EnsureIndex(mgo.Index{
		Key: []string{"foo"},
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

	err := harness.mongoClient.C("Foo").Insert(bson.M{
		"_id":   "someid",
		"hello": "world",
	})
	if err != nil {
		panic(err)
	}
	harness.resetMessages()

	err = harness.mongoClient.C("Foo").DropCollection()
	if err != nil {
		panic(err)
	}

	harness.verify(t, map[string][]helpers.OTRMessage{})
}
