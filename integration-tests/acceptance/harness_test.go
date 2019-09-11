package main

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/globalsign/mgo"
	"github.com/go-redis/redis"
	"github.com/kylelemons/godebug/pretty"
	"github.com/tulip/oplogtoredis/integration-tests/helpers"
)

// This file is a simple test harness for acceptance tests

type harness struct {
	redisClient   redis.UniversalClient
	subscription  *redis.PubSub
	subscriptionC <-chan *redis.Message
	mongoClient   *mgo.Database
}

// Clears the mongo database, connects to redis, and starts a subscription to
// all Redis publications
func startHarness() *harness {
	h := harness{}
	h.mongoClient = helpers.SeedTestDB(helpers.DBData{})

	// Connect to redis and start listening for the publication we expect
	h.redisClient = helpers.RedisClient()
	h.subscription = h.redisClient.PSubscribe("*")
	h.subscriptionC = h.subscription.Channel()

	return &h
}

// Shuts down all the mongo/redis clients
func (h *harness) stop() {
	h.mongoClient.Session.Close()
	h.redisClient.Close()
	h.subscription.Close()
}

// Gets all messages sent to Redis. Returns once it hasn't seen a new message
// in a second.
func (h *harness) getMessages() map[string][]helpers.OTRMessage {
	msgs := map[string][]helpers.OTRMessage{}

	for {
		select {
		case msg := <-h.subscriptionC:
			parsedMsg := helpers.OTRMessage{}
			err := json.Unmarshal([]byte(msg.Payload), &parsedMsg)
			if err != nil {
				panic("Error parsing JSON from redis: " + err.Error())
			}

			if val, ok := msgs[msg.Channel]; ok {
				// append to existing slice
				msgs[msg.Channel] = append(val, parsedMsg)
			} else {
				// make a new slice
				msgs[msg.Channel] = []helpers.OTRMessage{parsedMsg}
			}

		case <-time.After(100 * time.Millisecond):
			return msgs
		}
	}
}

// This is the same as getMessages, it just doesn't return the messages
func (h *harness) resetMessages() {
	h.getMessages()
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

	// Sort the fields inside each message, and the messages themselves, before we compare
	for _, pubs := range expectedPubs {
		for _, pub := range pubs {
			sort.Strings(pub.Fields)
		}

		helpers.SortOTRMessagesByID(pubs)
	}
	for _, pubs := range actualPubs {
		for _, pub := range pubs {
			sort.Strings(pub.Fields)
		}

		helpers.SortOTRMessagesByID(pubs)
	}

	// Verify the correct messages were received on each channel
	if diff := pretty.Compare(actualPubs, expectedPubs); diff != "" {
		t.Errorf("Got incorrect publications (-got +want)\n%s", diff)
	}
}
