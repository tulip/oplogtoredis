package main

import (
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"

	"github.com/globalsign/mgo/bson"
)

func TestRemove(t *testing.T) {
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

	err = harness.mongoClient.C("Foo").RemoveId("someid")
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
		"tests.Foo":         []helpers.OTRMessage{expectedMessage},
		"tests.Foo::someid": []helpers.OTRMessage{expectedMessage},
	})
}

func TestRemoveMultiple(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	// Initialize test data
	err := harness.mongoClient.C("Foo").Insert(bson.M{
		"_id": "someid",
		"foo": "bar",
	})
	if err != nil {
		panic(err)
	}

	err = harness.mongoClient.C("Foo").Insert(bson.M{
		"_id": "someid2",
		"foo": "bar",
	})
	if err != nil {
		panic(err)
	}

	harness.resetMessages()

	// Run an update that increments the first value more than 15
	_, err = harness.mongoClient.C("Foo").RemoveAll(bson.M{
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
		"tests.Foo":          []helpers.OTRMessage{expectedMessage1, expectedMessage2},
		"tests.Foo::someid":  []helpers.OTRMessage{expectedMessage1},
		"tests.Foo::someid2": []helpers.OTRMessage{expectedMessage2},
	})
}
