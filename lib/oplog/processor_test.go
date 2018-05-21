package oplog

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/globalsign/mgo/bson"
	"github.com/tulip/oplogtoredis/lib/redispub"
)

// func TestTail(t *testing.T) {
// 	// This is just a basic test for the flow control of Tail. The full
// 	// behavior of converting oplog to entries to publications is tested
// 	// in TestProcessOplogEntry below.
// 	//
// 	// High-level strategy: spin up the Tail function. Send two oplog entries,
// 	// then send a stop message, then wait for the function to return. Make
// 	// sure the channel is no longer being read from. Make sure two
// 	// publications were sent to the output channel.

// 	inChan := make(chan *oplogEntry)
// 	outChan := make(chan *redispub.Publication, 3)
// 	stopChan := make(chan bool)

// 	waitGroup := sync.WaitGroup{}
// 	waitGroup.Add(1)
// 	go func() {
// 		Tail(&TailConfig{
// 			Connect:    func() <-chan *oplogEntry { return inChan },
// 			Disconnect: func() {},
// 			OutputC:    outChan,
// 			StopC:      stopChan,
// 		})
// 		waitGroup.Done()
// 	}()

// 	// Write two oplog entries
// 	inChan <- &oplogEntry{
// 		ID:        "doc1",
// 		Operation: "i",
// 		Namespace: "foo.bar",
// 		Data:      bson.M{},
// 		Timestamp: bson.MongoTimestamp(1234),
// 	}
// 	inChan <- &oplogEntry{
// 		ID:        "doc2",
// 		Operation: "i",
// 		Namespace: "foo.bar",
// 		Data:      bson.M{},
// 		Timestamp: bson.MongoTimestamp(1235),
// 	}

// 	// Write a stop message
// 	stopChan <- true

// 	// The function should return (we wait here so the test will time out
// 	// if it does not)
// 	waitGroup.Wait()

// 	// Make sure we're no longer reading from that channel
// 	select {
// 	case inChan <- &oplogEntry{ID: "doc3"}:
// 		t.Error("Was able to write to channel after sending a stop message")
// 	default:
// 	}

// 	// We should have gotten two publications
// 	if len(outChan) != 2 {
// 		t.Errorf("Expected 2 publications, got %d", len(outChan))
// 	}

// 	pub1 := <-outChan
// 	pub2 := <-outChan

// 	// we just check one field to make sure ordering is correct -- we check the
// 	// full publicaiton records in TestProcessOplogEntry
// 	if (pub1.SpecificChannel != "foo.bar::doc1") || (pub2.SpecificChannel != "foo.bar::doc2") {
// 		t.Errorf("Got incorrect publications.\n    pub 1: %#v\n    pub 2: %#v",
// 			pub1, pub2)
// 	}
// }

// func TestTailIdleReconnect(t *testing.T) {
// 	// This tests the behavior of Tail where it reconnects if it doesn't
// 	// receive a message for config.ReconnectIdleTime
// 	//
// 	// High-level strategy: spin up the Tail function. With a 200ms timeout.
// 	// Send a message after 100 ms. After 200 ms, there should not have been
// 	// a reconnect. After 400ms, there should have been a reconnect. After
// 	// 600ms, there should have been two reconnects.

// 	inChan := make(chan *oplogEntry)
// 	outChan := make(chan *redispub.Publication, 3)
// 	stopChan := make(chan bool)
// 	reconnectCount := 0

// 	waitGroup := sync.WaitGroup{}
// 	waitGroup.Add(1)
// 	go func() {
// 		Tail(&TailConfig{
// 			ReconnectIdleTime: 200 * time.Millisecond,
// 			Connect:           func() <-chan *oplogEntry { return inChan },
// 			Disconnect:        func() { reconnectCount++ },
// 			OutputC:           outChan,
// 			StopC:             stopChan,
// 		})
// 		waitGroup.Done()
// 	}()

// 	time.Sleep(100 * time.Millisecond) // t = 100
// 	inChan <- &oplogEntry{
// 		ID:        "doc1",
// 		Operation: "i",
// 		Namespace: "foo.bar",
// 		Data:      bson.M{},
// 		Timestamp: bson.MongoTimestamp(1234),
// 	}

// 	time.Sleep(100 * time.Millisecond) // t = 200
// 	if reconnectCount != 0 {
// 		t.Errorf("At t = 200, expected 0 reconnects, but there were %d", reconnectCount)
// 	}

// 	time.Sleep(200 * time.Millisecond) // t = 400
// 	if reconnectCount != 1 {
// 		t.Errorf("At t = 400, expected 1 reconnects, but there were %d", reconnectCount)
// 	}

// 	time.Sleep(200 * time.Millisecond) // t = 600
// 	if reconnectCount != 2 {
// 		t.Errorf("At t = 600, expected 2 reconnects, but there were %d", reconnectCount)
// 	}

// 	// Clean up background goroutines
// 	stopChan <- true
// 	waitGroup.Wait()
// }
func TestProcessOplogEntry(t *testing.T) {
	// We can't compare raw publications because they contain JSON that can
	// be ordered differently. We have this decodedPublication type that's
	// the same as redispub.Publication but with the JSON decoded
	type decodedPublicationMessage struct {
		Event  string      `json:"e"`
		Doc    interface{} `json:"d"`
		Fields []string    `json:"f"`
	}
	type decodedPublication struct {
		CollectionChannel string
		SpecificChannel   string
		Msg               decodedPublicationMessage
		OplogTimestamp    bson.MongoTimestamp
	}

	tests := map[string]struct {
		// The oplogEntry to send to the tailer
		in *oplogEntry

		// The redispub.Publication we expect the tailer to produce. If the
		// test expects nothing to be published for the op, set this to nil
		want *decodedPublication
	}{
		"Basic insert": {
			in: &oplogEntry{
				DocID:     "someid",
				Operation: "i",
				Namespace: "foo.bar",
				Data: bson.M{
					"some": "field",
				},
				Timestamp: bson.MongoTimestamp(1234),
			},
			want: &decodedPublication{
				CollectionChannel: "foo.bar",
				SpecificChannel:   "foo.bar::someid",
				Msg: decodedPublicationMessage{
					Event: "i",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{"some"},
				},
				OplogTimestamp: bson.MongoTimestamp(1234),
			},
		},
		"Replacement update": {
			in: &oplogEntry{
				DocID:     "someid",
				Operation: "u",
				Namespace: "foo.bar",
				Data: bson.M{
					"some": "field",
					"new":  "field",
				},
				Timestamp: bson.MongoTimestamp(1234),
			},
			want: &decodedPublication{
				CollectionChannel: "foo.bar",
				SpecificChannel:   "foo.bar::someid",
				Msg: decodedPublicationMessage{
					Event: "u",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{"some", "new"},
				},
				OplogTimestamp: bson.MongoTimestamp(1234),
			},
		},
		"Non-replacement update": {
			in: &oplogEntry{
				DocID:     "someid",
				Operation: "u",
				Namespace: "foo.bar",
				Data: bson.M{
					"$v": "1.2.3",
					"$set": map[string]interface{}{
						"a": "foo",
						"b": "foo",
					},
					"$unset": map[string]interface{}{
						"c": "foo",
					},
				},
				Timestamp: bson.MongoTimestamp(1234),
			},
			want: &decodedPublication{
				CollectionChannel: "foo.bar",
				SpecificChannel:   "foo.bar::someid",
				Msg: decodedPublicationMessage{
					Event: "u",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{"a", "b", "c"},
				},
				OplogTimestamp: bson.MongoTimestamp(1234),
			},
		},
		"Delete": {
			in: &oplogEntry{
				DocID:     "someid",
				Operation: "d",
				Namespace: "foo.bar",
				Data:      bson.M{},
				Timestamp: bson.MongoTimestamp(1234),
			},
			want: &decodedPublication{
				CollectionChannel: "foo.bar",
				SpecificChannel:   "foo.bar::someid",
				Msg: decodedPublicationMessage{
					Event: "r",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{},
				},
				OplogTimestamp: bson.MongoTimestamp(1234),
			},
		},
		"ObjectID id": {
			in: &oplogEntry{
				DocID:     bson.ObjectIdHex("deadbeefdeadbeefdeadbeef"),
				Operation: "i",
				Namespace: "foo.bar",
				Data: bson.M{
					"some": "field",
				},
				Timestamp: bson.MongoTimestamp(1234),
			},
			want: &decodedPublication{
				CollectionChannel: "foo.bar",
				SpecificChannel:   "foo.bar::deadbeefdeadbeefdeadbeef",
				Msg: decodedPublicationMessage{
					Event: "i",
					Doc: map[string]interface{}{
						"_id": map[string]interface{}{
							"$type":  "oid",
							"$value": "deadbeefdeadbeefdeadbeef",
						},
					},
					Fields: []string{"some"},
				},
				OplogTimestamp: bson.MongoTimestamp(1234),
			},
		},
		"Unsupported id type": {
			in: &oplogEntry{
				DocID:     1234,
				Operation: "i",
				Namespace: "foo.bar",
				Data: bson.M{
					"some": "field",
				},
				Timestamp: bson.MongoTimestamp(1234),
			},
			want: nil,
		},
		"Index update": {
			in: &oplogEntry{
				DocID:     "someid",
				Operation: "i",
				Namespace: "foo.system.indexes",
				Data: bson.M{
					"some": "field",
				},
				Timestamp: bson.MongoTimestamp(1234),
			},
			want: nil,
		},
	}

	// helper to convert a redispub.Publication to a decodedPublication
	decodePublication := func(pub *redispub.Publication) *decodedPublication {
		msg := decodedPublicationMessage{}
		err := json.Unmarshal(pub.Msg, &msg)
		if err != nil {
			panic(fmt.Sprintf("Error parsing Msg field of publication: %s\n    JSON: %s",
				err, pub.Msg))
		}

		return &decodedPublication{
			CollectionChannel: pub.CollectionChannel,
			SpecificChannel:   pub.SpecificChannel,
			Msg:               msg,
			OplogTimestamp:    pub.OplogTimestamp,
		}
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			// Create an output channel. We create a buffered channel so that
			// we can run Tail

			got := processOplogEntry(test.in)

			if (got == nil) && (test.want != nil) {
				t.Errorf("Got nil when we expected a publication\n    Input: %#v\n    Wanted: %#v",
					test.in, test.want)
			} else if (got != nil) && (test.want == nil) {
				t.Errorf("Got a publication when we expected nil\n    Input: %#v\n    Got: %#v",
					test.in, got)
			} else if (got != nil) && (test.want != nil) {
				decodedGot := decodePublication(got)

				// sort the array of fields so we can compare them
				sort.Strings(test.want.Msg.Fields)
				sort.Strings(decodedGot.Msg.Fields)

				if !reflect.DeepEqual(decodedGot, test.want) {
					t.Errorf("Got incorrect publication\n    Got: %#v\n    Want: %#v",
						decodedGot, test.want)
				}
			}
		})
	}
}
