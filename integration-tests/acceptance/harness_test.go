package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/kylelemons/godebug/pretty"
	"github.com/tulip/oplogtoredis/integration-tests/helpers"
	"github.com/tulip/oplogtoredis/lib/log"
	"go.mongodb.org/mongo-driver/mongo"
)

// This file is a simple test harness for acceptance tests

type harness struct {
	redisClient         redis.UniversalClient
	legacyRedisClient   redis.UniversalClient
	subscription        *redis.PubSub
	subscriptionC       <-chan *redis.Message
	legacySubscription  *redis.PubSub
	legacySubscriptionC <-chan *redis.Message
	mongoClient         *mongo.Database
}

// Clears the mongo database, connects to redis, and starts a subscription to
// all Redis publications
func startHarness() *harness {
	h := harness{}
	h.mongoClient = helpers.SeedTestDB(helpers.DBData{})

	// Connect to redis and start listening for the publication we expect
	h.redisClient = helpers.RedisClient()
	h.subscription = h.redisClient.PSubscribe(context.Background(), "*")
	h.subscriptionC = h.subscription.Channel()

	h.legacyRedisClient = helpers.LegacyRedisClient()
	h.legacySubscription = h.legacyRedisClient.PSubscribe(context.Background(), "*")
	h.legacySubscriptionC = h.legacySubscription.Channel()

	return &h
}

// Shuts down all the mongo/redis clients
func (h *harness) stop() {
	_ = h.mongoClient.Client().Disconnect(context.Background())
	h.redisClient.Close()
	h.subscription.Close()

	h.legacyRedisClient.Close()
	h.legacySubscription.Close()
}

// Gets all messages sent to Redis. Returns once it hasn't seen a new message
// in a second.
func (h *harness) getMessagesHelper(legacy bool) map[string][]helpers.OTRMessage {
	msgs := map[string][]helpers.OTRMessage{}
	var ch <-chan *redis.Message
	if legacy {
		ch = h.legacySubscriptionC
	} else {
		ch = h.subscriptionC
	}

	for {
		select {
		case msg := <-ch:
			parsedMsg := helpers.OTRMessage{}
			err := json.Unmarshal([]byte(msg.Payload), &parsedMsg)
			if err != nil {
				// Optional: check for sentinel related messages
				log.Log.Debugw("Error parsing JSON from redis: " + err.Error() + "\n Response text: " + msg.Payload)
			}

			if val, ok := msgs[msg.Channel]; ok {
				// append to existing slice
				msgs[msg.Channel] = append(val, parsedMsg)
			} else {
				// make a new slice
				msgs[msg.Channel] = []helpers.OTRMessage{parsedMsg}
			}

		case <-time.After(400 * time.Millisecond):
			return msgs
		}
	}
}
func (h *harness) getMessages() map[string][]helpers.OTRMessage {
	return h.getMessagesHelper(false)
}
func (h *harness) getLegacyMessages() map[string][]helpers.OTRMessage {
	return h.getMessagesHelper(true)
}

// This is the same as getMessages, it just doesn't return the messages
func (h *harness) resetMessages() {
	h.getMessages()
	h.getLegacyMessages()
}
func (h *harness) verifyPub(t *testing.T, pub map[string][]helpers.OTRMessage, expectedPubs map[string][]helpers.OTRMessage) {
	for _, pubs := range pub {
		for _, pub := range pubs {
			sort.Strings(pub.Fields)
		}

		helpers.SortOTRMessagesByID(pubs)
	}
	for key := range pub {
		if strings.Contains(key, "sentinel") {
			delete(pub, key)
		}
	}
	if diff := pretty.Compare(pub, expectedPubs); diff != "" {
		t.Errorf("Got incorrect publications (-got +want)\n%s", diff)
	}

}

// Check the publications that were actually made against the publications that
// we expect to be made.
//
// expectedPubs is a map of channel name -> list of messages that should
// have been sent there (in any order)
func (h *harness) verify(t *testing.T, expectedPubs map[string][]helpers.OTRMessage) {
	// Receive all the messages (waiting until no messages are received for a
	// second)
	actualPubs := h.getMessages()
	actualLegacyPubs := h.getLegacyMessages()

	// Sort the fields inside each message, and the messages themselves, before we compare
	for _, pubs := range expectedPubs {
		for _, pub := range pubs {
			sort.Strings(pub.Fields)
		}

		helpers.SortOTRMessagesByID(pubs)
	}
	h.verifyPub(t, actualPubs, expectedPubs)
	h.verifyPub(t, actualLegacyPubs, expectedPubs)
	// pop the __sentinel__ entry

	// Verify the correct messages were received on each channel
}
