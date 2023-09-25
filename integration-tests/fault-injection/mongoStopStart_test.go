package main

import (
	"context"
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/integration-tests/fault-injection/harness"
)

// This test stops and restarts mongo during the test. We expect that every
// insert that was confirmed by Mongo was picked up by oplogtoredis.
//
// This test infrequently (~10% of the time) triggers bad behavior in either
// mgo or gtm where the oplog tailing locks up after the reconnect. To keep this
// test reliable, we enable the IDLE_RECONNECT_TIMEOUT option of oplogtoredis
// to detect and recover from this condition.
func TestMongoStopStart(t *testing.T) {
	mongo := harness.StartMongoServer()
	defer mongo.Stop()

	redis := harness.StartRedisServer()
	defer redis.Stop()

	otr := harness.StartOTRProcessWithEnv(mongo.Addr, redis.Addr, 9000, []string{
		"OTR_IDLE_RECONNECT_TIMEOUT=5s",
	})
	defer otr.Stop()

	mongoClient := mongo.Client()
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	redisClient := redis.Client()
	defer redisClient.Close()

	verifier := harness.NewRedisVerifier(redisClient, true)

	// We do this test over 60 seconds instead of 10 because we have to give
	// mongo some extra time to run an election after the restart, and the mongo client
	// can take extra time to reconnect to the primary after a reelection
	inserter := harness.RunInsertsInBackground(mongoClient.Database(mongo.DBName), 100, 600*time.Millisecond)

	time.Sleep(3 * time.Second)
	mongo.Stop()

	time.Sleep(3 * time.Second)
	mongo.Start()

	insertedIDs := inserter.Result()

	if len(insertedIDs) < 50 {
		// We should have recovered fast enough for at least 50 writes to
		// succedd
		t.Errorf("Expected at least 50 inserted IDs, got %d", len(insertedIDs))
	}

	verifier.VerifyFlakyInserts(t, mongoClient.Database(mongo.DBName), insertedIDs)
}
