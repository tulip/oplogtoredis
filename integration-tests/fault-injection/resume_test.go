package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tulip/oplogtoredis/integration-tests/fault-injection/harness"
	"gopkg.in/mgo.v2/bson"
)

// This test covers the pick-up-where-we-left-off behavior of oplogtoredis.
// It tests that we start from the end of the oplog if we don't have a left-off-at
// record in Redis or if that record is too old, and that we start from where we
// left off if that record exists and is reasonably recent.
func TestResume(t *testing.T) {
	mongo := harness.StartMongoServer()
	defer mongo.Stop()

	redis := harness.StartRedisServer()
	defer redis.Stop()

	mongoClient := mongo.Client()
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	redisClient := redis.Client()
	defer redisClient.Close()

	verifier := harness.NewRedisVerifier(redisClient, true)

	// We insert a couple things into the oplog to make sure they don't
	// get processed by oplogtoredis
	testCollection := mongoClient.Database(mongo.DBName).Collection("Test")

	_, err := testCollection.InsertOne(context.Background(), bson.M{"_id": "id1"})
	require.NoError(t, err)
	_, err = testCollection.InsertOne(context.Background(), bson.M{"_id": "id2"})
	require.NoError(t, err)
	_, err = testCollection.InsertOne(context.Background(), bson.M{"_id": "id3"})
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	otr := harness.StartOTRProcessWithEnv(mongo.Addr, redis.Addr, 9000, []string{
		"OTR_MAX_CATCH_UP=8s",
	})
	defer otr.Stop()

	// Test that on first run, we start from the end of the oplog
	_, err = testCollection.InsertOne(context.Background(), bson.M{"_id": "id4"})
	require.NoError(t, err)
	verifier.Verify(t, []string{"id4"})

	nProcessed := harness.PromMetricOplogEntriesProcessed(otr.GetPromMetrics())
	if nProcessed != 1 {
		t.Errorf("Expected otr to have processed 1 entry, but got: %d", nProcessed)
	}

	// Wait for long enough for us to flush last-handled timestamp to redis
	time.Sleep(1 * time.Second)

	// Pause for less than OTR_MAX_CATCH_UP and make sure we publish the things
	// we missed when we start back up
	otr.Stop()
	_, err = testCollection.InsertOne(context.Background(), bson.M{"_id": "id5"})
	require.NoError(t, err)

	otr.Start()
	_, err = testCollection.InsertOne(context.Background(), bson.M{"_id": "id6"})
	require.NoError(t, err)

	verifier.Verify(t, []string{"id5", "id6"})

	nProcessed = harness.PromMetricOplogEntriesProcessed(otr.GetPromMetrics())
	if nProcessed != 2 {
		t.Errorf("Expected otr to have processed 2 entries, but got: %d", nProcessed)
	}

	// Pause for more than OTR_MAX_CATCH_UP and make sure we don't try to
	// catch up when we start back up
	otr.Stop()
	_, err = testCollection.InsertOne(context.Background(), bson.M{"_id": "id7"})
	require.NoError(t, err)

	time.Sleep(8 * time.Second)
	otr.Start()
	_, err = testCollection.InsertOne(context.Background(), bson.M{"_id": "id8"})
	require.NoError(t, err)

	verifier.Verify(t, []string{"id8"})

	nProcessed = harness.PromMetricOplogEntriesProcessed(otr.GetPromMetrics())
	if nProcessed != 1 {
		t.Errorf("Expected otr to have processed 1 entry, but got: %d", nProcessed)
	}
}
