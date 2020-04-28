package harness

import (
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/tulip/oplogtoredis/integration-tests/helpers"
)

// RedisVerifier subscribes to the publications that a BackgroundInserter should
// be making, and verifies that they were made.
type RedisVerifier struct {
	client      redis.UniversalClient
	receivedIDs chan string
	pubsub      *redis.PubSub
}

// NewRedisVerifier creates a RedisVerifier and starts reading messages from
// Redis
func NewRedisVerifier(client redis.UniversalClient) *RedisVerifier {
	if pingErr := client.Ping().Err(); pingErr != nil {
		panic("Ping error to redis: " + pingErr.Error())
	}

	verifier := RedisVerifier{
		client:      client,
		receivedIDs: make(chan string, 100),
		pubsub:      client.Subscribe("testdb.Test"),
	}

	go func() {
		for {
			msg, err := verifier.pubsub.ReceiveMessage()
			if err != nil {
				log.Printf("Error receiving pubsub message: %s", err.Error())
				break
			}

			parsedMsg := helpers.OTRMessage{}
			err = json.Unmarshal([]byte(msg.Payload), &parsedMsg)

			if err != nil {
				panic("Error parsing message from Redis: " + err.Error())
			}

			verifier.receivedIDs <- parsedMsg.Document["_id"].(string)
		}
	}()

	return &verifier
}

// Verify verifies that the given IDs match the messages published to Redis.
// It blocks until all expected IDs have been received (timing out if nothing
// is received for 10 seconds)
func (verifier *RedisVerifier) Verify(t *testing.T, ids []string) {
	for idx, id := range ids {
		select {
		case receivedID := <-verifier.receivedIDs:
			if receivedID != id {
				t.Errorf("On message %d, received %s but expected %s",
					idx, receivedID, id)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("Timed out waiting for redis message %d", idx)
			return
		}
	}
}

// ReceivedCount returns the number of Redis message received by this verifier
func (verifier *RedisVerifier) ReceivedCount() int {
	// We wait until we're no longer actively receiving messages
	for {
		startCount := len(verifier.receivedIDs)
		time.Sleep(time.Second)

		if len(verifier.receivedIDs) == startCount {
			return startCount
		}
	}
}
