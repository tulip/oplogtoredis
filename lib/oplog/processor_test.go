package oplog

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/globalsign/mgo/bson"
	"github.com/pkg/errors"
	"github.com/tulip/oplogtoredis/lib/redispub"
)

// nolint: gocyclo
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

		wantError error
	}{
		"Basic insert": {
			in: &oplogEntry{
				DocID:      "someid",
				Operation:  "i",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
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
				DocID:      "someid",
				Operation:  "u",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
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
				DocID:      "someid",
				Operation:  "u",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
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
				DocID:      "someid",
				Operation:  "d",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
				Data:       bson.M{},
				Timestamp:  bson.MongoTimestamp(1234),
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
				DocID:      bson.ObjectIdHex("deadbeefdeadbeefdeadbeef"),
				Operation:  "i",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
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
				DocID:      1234,
				Operation:  "i",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
				Data: bson.M{
					"some": "field",
				},
				Timestamp: bson.MongoTimestamp(1234),
			},
			wantError: ErrUnsupportedDocIDType,
			want:      nil,
		},
		"Index update": {
			in: &oplogEntry{
				DocID:      "someid",
				Operation:  "i",
				Namespace:  "foo.system.indexes",
				Database:   "foo",
				Collection: "system.indexes",
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

			got, err := processOplogEntry(test.in)

			if test.wantError != nil {
				assert.EqualError(t, errors.Cause(err), test.wantError.Error())
			} else {
				assert.NoError(t, err)
			}

			if got == nil {
				require.Nil(t, test.want)
				return
			}

			assert.Equal(t, test.want, decodePublication(got))
		})
	}
}
