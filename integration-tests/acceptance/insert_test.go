package main

import (
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/helpers"

	"github.com/globalsign/mgo/bson"
)

// Basic test of an insert
func TestInsert(t *testing.T) {
	harness := startHarness()
	defer harness.stop()

	err := harness.mongoClient.C("Foo").Insert(bson.M{
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
		"tests.Foo":         []helpers.OTRMessage{expectedMessage},
		"tests.Foo::someid": []helpers.OTRMessage{expectedMessage},
	})
}

// TODO TESTING:
//   - updates
//   - deletes
//   - really weird update operators
//   - multi-updates/deletes
//   - reconnect behavior (both mongo and redis)
