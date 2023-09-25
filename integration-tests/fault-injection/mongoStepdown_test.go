package main

import (
	"context"
	"log"
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
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	redisClient := redis.Client()
	defer redisClient.Close()

	verifier := harness.NewRedisVerifier(redisClient, true)
	// replset leader election is fast in 4.4, so we need a shorter retry or we won't catch it
	inserter := harness.RunInsertsInBackground(mongoClient.Database(mongo.DBName), 100, 50*time.Millisecond)
	time.Sleep(time.Second)
	log.Print("Stepping down mongo .. ")
	mongo.StepDown()

	insertedIDs := inserter.Result()

	if len(insertedIDs) < 50 {
		// We should have recovered fast enough for at least 50 writes to
		// succeed
		t.Errorf("Expected at least 50 inserted IDs, got %d", len(insertedIDs))
	}

	if len(insertedIDs) > 100 {
		// The new failover / leader election logic is very fast,
		// So it is possible that we got all 100 Inserted IDs. However, make
		// sure we didn't get any duplicates.
		t.Errorf("Expected no more than 100 successful writes, got %d", len(insertedIDs))
	}

	verifier.VerifyFlakyInserts(t, mongoClient.Database(mongo.DBName), insertedIDs)
}
