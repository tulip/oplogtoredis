package main

import (
	"context"
	"testing"
	"time"

	"github.com/tulip/oplogtoredis/integration-tests/fault-injection/harness"
)

// This test stops and restart redis during the test.
//
// This test does looser verification that the other ones -- oplogtoredis
// should recover, and should retry the messages it failed to send; however,
// we can't guarantee that the listener
func TestRedisStopStart(t *testing.T) {
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

	verifier := harness.NewRedisVerifier(redisClient, false)
	inserter := harness.Run100InsertsInBackground(mongoClient.Database(mongo.DBName))

	time.Sleep(2 * time.Second)
	redis.Stop()

	time.Sleep(2 * time.Second)
	redis.Start()

	inserter.Result()

	// We can't do full verification -- our listener might not have re-connected
	// before we flushed our queued messages, so it's expected that the verifier
	// will lose some messages. We're going to check that we got at least 60
	// messages to make sure that we did eventually reconnect and start sending
	// messages (rather than staying disconnected when redis went offline)
	receivedCount := verifier.ReceivedCount()

	if receivedCount < 60 {
		// We should have recovered fast enough for at least 50 writes to
		// succedd
		t.Errorf("Expected at least 60 received messages, got %d", receivedCount)
	}

	// We want to make sure that even if our verifier didn't get all 100
	// messages, we did send all 100. To do this, we look at the metrics.
	// We want to see exactly 100 successful redis publications, 0 permanent
	// failures, and >0 temporary failures
	metrics := otr.GetPromMetrics()

	nSuccess := harness.FindPromMetricCounter(metrics, "otr_redispub_processed_messages", map[string]string{
		"status": "sent",
		"idx":    "0",
	})
	if nSuccess != 100 {
		t.Errorf("Metric otr_redispub_processed_messages(status: sent) = %d, expected 100", nSuccess)
	}

	nPermFail := harness.FindPromMetricCounter(metrics, "otr_redispub_processed_messages", map[string]string{
		"status": "failed",
		"idx":    "0",
	})
	if nPermFail != 0 {
		t.Errorf("Metric otr_redispub_processed_messages(status: failed) = %d, expected 0", nPermFail)
	}

	nTempFail := harness.FindPromMetricCounter(metrics, "otr_redispub_temporary_send_failures", map[string]string{})
	if nTempFail <= 0 {
		t.Errorf("Metric otr_redispub_processed_messages = %d, expected >0", nTempFail)
	}
}
