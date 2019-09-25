package main

import (
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/integration-tests/fault-injection/harness"
)

// This test triggers a mongo stepdown during execution. We expect that every
// insert that was confirmed by mongo was picked up by oplogtoredis.
func TestMongoStepdown(t *testing.T) {
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
	mongo.StepDown()

	insertedIDs := inserter.Result()

	if len(insertedIDs) < 50 {
		// We should have recovered fast enough for at least 50 writes to
		// succeed
		t.Errorf("Expected at least 50 inserted IDs, got %d", len(insertedIDs))
	}

	if len(insertedIDs) >= 100 {
		// If every insert was successful, then we definitely didn't step down
		// correctly; fail this test because it wasn't a valid test
		t.Errorf("Expected no more than 99 successful writes, got %d", len(insertedIDs))
	}

	verifier.Verify(t, insertedIDs)
}
