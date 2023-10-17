package main

import (
	"context"
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

	// Sleeping here for a while as the initial connection seems to be unreliable
	time.Sleep(time.Second * 1)

	redis := harness.StartRedisServer()
	defer redis.Stop()

	otr := harness.StartOTRProcess(mongo.Addr, redis.Addr, 9000)
	defer otr.Stop()

	mongoClient := mongo.Client()
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	redisClient := redis.Client()
	defer redisClient.Close()

	verifier := harness.NewRedisVerifier(redisClient, true)
	inserter := harness.Run100InsertsInBackground(mongoClient.Database(mongo.DBName))

	time.Sleep(3 * time.Second)
	otr.Stop()

	time.Sleep(3 * time.Second)
	otr.Start()

	insertedIDs := inserter.Result()

	if len(insertedIDs) != 100 {
		t.Errorf("Expected 100 inserted IDs, got %d", len(insertedIDs))
	}

	verifier.VerifyFlakyInserts(t, mongoClient.Database(mongo.DBName), insertedIDs)

	// We also want to verify that we picked up from where we left off (rather
	// that starting from the beginning of the oplog or something). The first
	// oplogtoredis should process 30 of the 100 messages, and the second
	// run of oplog to redis should process the remaining 70, plus re-process
	// no more than 10 of the original ones. So we check here that we processed
	// between 70 and 80 messages.
	metrics := otr.GetPromMetrics()
	entriesReceived := harness.FindPromMetric(metrics, "otr_oplog_entries_received", map[string]string{
		"database": "testdb",
		"status":   "processed",
	}).Counter.Value

	if (*entriesReceived < 70) || (*entriesReceived > 80) {
		t.Errorf("Expected second otr run to process between 70 and 80 messages; but it processed %d", entriesReceived)
	}
}
