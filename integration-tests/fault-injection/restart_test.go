package main

import (
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/integration-tests/fault-injection/harness"
)

// This test restarts oplogtoredis partway through the test to test the
// pick-up-from-where-we-left-off behavior. Between that and redis-level
// deduplication, we expect all 100 writes to come through with no issues.
func TestRestart(t *testing.T) {
	mongo := harness.StartMongoServer()
	defer mongo.Stop()

	redis := harness.StartRedisServer()
	defer redis.Stop()

	otr := harness.StartOTRProcess(mongo.Addr, redis.Addr, 9000)
	defer otr.Stop()

	mongoClient := mongo.Client()
	defer mongoClient.Close()

	redisClient := redis.Client()
	defer redisClient.Close()

	verifier := harness.NewRedisVerifier(redisClient)
	inserter := harness.Run100InsertsInBackground(mongoClient.DB(""))

	time.Sleep(time.Second)
	otr.Stop()

	time.Sleep(3 * time.Second)
	otr.Start()

	insertedIDs := inserter.Result()

	if len(insertedIDs) != 100 {
		t.Errorf("Expected 100 inserted IDs, got %d", len(insertedIDs))
	}

	verifier.Verify(t, insertedIDs)
}
