package main

import (
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
	defer mongoClient.Close()

	redisClient := redis.Client()
	defer redisClient.Close()

	verifier := harness.NewRedisVerifier(redisClient)
	inserter := harness.Run100InsertsInBackground(mongoClient.DB(""))

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
		t.Errorf("Expected at least 60 recieved messages, got %d", receivedCount)
	}
}