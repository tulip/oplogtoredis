package harness

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/tulip/oplogtoredis/integration-tests/helpers"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
func NewRedisVerifier(client redis.UniversalClient, stopReceivingOnError bool) *RedisVerifier {
	if pingErr := client.Ping(context.Background()).Err(); pingErr != nil {
		panic("Ping error to redis: " + pingErr.Error())
	}

	verifier := RedisVerifier{
		client:      client,
		receivedIDs: make(chan string, 100),
		pubsub:      client.Subscribe(context.Background(), "testdb.Test"),
	}

	go func() {
		for {
			msg, err := verifier.pubsub.ReceiveMessage(context.Background())
			if err != nil {
				log.Printf("Error receiving pubsub message: %s", err.Error())
				if stopReceivingOnError {
					break
				} else {
					time.Sleep(500 * time.Millisecond)
					continue
				}
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

// Verify verifies that the messages received from Redis were destined to be written there
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

func (verifier *RedisVerifier) VerifyFlakyInserts(t *testing.T, mongoClient *mongo.Database, mongoIDs []string) {
	for idx, id := range mongoIDs {
		select {
		case receivedID := <-verifier.receivedIDs:
			if receivedID != id {
				// Sometimes the Insert is detected as a failure, but the insert actually succeeds.
				// This is a hacky case to check if this document was really written to Mongo
				findOneResult := mongoClient.Collection("Test").FindOne(context.Background(), bson.M{
					"_id": receivedID,
				})

				if findOneResult.Err() != nil {
					t.Errorf("On message %d, received %s but expected %s",
						idx, receivedID, id)
				}
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
