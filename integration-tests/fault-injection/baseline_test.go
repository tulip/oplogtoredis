package main

import (
	"testing"

	"github.com/tulip/oplogtoredis/integration-tests/fault-injection/harness"
)

// The baseline test does not inject any faults. It's here to test the
// baseline correctness of the test harness.

// It also acts as a soak test -- while the other tests do 100 inserts over
// 10 seconds, this one does 100 inserts as fast as possible.
func TestBaseline(t *testing.T) {
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

	verifier := harness.NewRedisVerifier(redisClient, true)
	inserter := harness.RunInsertsInBackground(mongoClient.DB(""), 100, 0)

	insertedIDs := inserter.Result()

	if len(insertedIDs) != 100 {
		t.Errorf("Expected 100 inserted IDs, got %d", len(insertedIDs))
	}

	verifier.Verify(t, insertedIDs)
}
