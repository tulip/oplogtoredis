package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/tulip/oplogtoredis/acceptance/helpers"

	"github.com/globalsign/mgo/bson"
	"github.com/go-redis/redis"
)

func redisClient() *redis.Client {
	redisOpts, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(err)
	}

	return redis.NewClient(redisOpts)
}

// Basic test of an insert
func TestInsert(t *testing.T) {
	// Connect to redis and start listening for the publication we expect
	client := redisClient()
	defer client.Close()

	subscr := client.Subscribe("tests.Foo")
	defer subscr.Close()

	db := helpers.SeedTestDB(helpers.DBData{})
	defer db.Session.Close()

	err := db.C("Foo").Insert(bson.M{
		"_id":   "someid",
		"hello": "world",
	})
	if err != nil {
		panic(err)
	}

	msg, err := subscr.ReceiveMessage()
	if err != nil {
		panic(err)
	}

	expectedMsg := map[string]interface{}{
		"e": "i",
		"d": map[string]interface{}{
			"_id": "someid",
		},
		"f": []interface{}{"_id", "hello"},
	}

	var actualMsg map[string]interface{}
	err = json.Unmarshal([]byte(msg.Payload), &actualMsg)
	if err != nil {
		panic(err)
	}

	if !reflect.DeepEqual(actualMsg, expectedMsg) {
		t.Errorf("Incorrect message.\n    Actual: %#v\n    Expected: %#v",
			actualMsg, expectedMsg)
	}
}

// TODO TESTING:
//   - updates
//   - deletes
//   - really weird update operators
//   - multi-updates/deletes
//   - reconnect behavior (both mongo and redis)
