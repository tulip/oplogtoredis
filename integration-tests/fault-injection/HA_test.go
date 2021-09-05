package main

import (
	"testing"

	"github.com/vlasky/oplogtoredis/integration-tests/fault-injection/harness"
)

// This test (not really a "fault injection" per se) runs two copies of
// oplogtoredis to ensure messages are never double-sent
func TestHA(t *testing.T) {
	mongo := harness.StartMongoServer()
	defer mongo.Stop()

	redis := harness.StartRedisServer()
	defer redis.Stop()

	otr := harness.StartOTRProcess(mongo.Addr, redis.Addr, 9000)
	defer otr.Stop()

	otr2 := harness.StartOTRProcess(mongo.Addr, redis.Addr, 9001)
	defer otr2.Stop()

	mongoClient := mongo.Client()
	defer mongoClient.Close()

	redisClient := redis.Client()
	defer redisClient.Close()

	verifier := harness.NewRedisVerifier(redisClient, true)
	inserter := harness.Run100InsertsInBackground(mongoClient.DB(""))

	insertedIDs := inserter.Result()

	if len(insertedIDs) != 100 {
		t.Errorf("Expected 100 inserted IDs, got %d", len(insertedIDs))
	}

	verifier.Verify(t, insertedIDs)
}
