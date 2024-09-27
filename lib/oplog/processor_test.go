package oplog

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/pkg/errors"
	"github.com/tulip/oplogtoredis/lib/redispub"
	"go.mongodb.org/mongo-driver/bson"
)

// hash of the database name "foo" to be expected for ParallelismKey
const fooHash = -5843589418109203719

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
		Channels       []string
		Msg            decodedPublicationMessage
		OplogTimestamp primitive.Timestamp
		ParallelismKey int
	}

	testObjectId, err := primitive.ObjectIDFromHex("deadbeefdeadbeefdeadbeef")
	if err != nil {
		panic(err)
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
				Data: rawBson(t, bson.M{
					"some": "field",
				}),
				Timestamp: primitive.Timestamp{T: 1234},
			},
			want: &decodedPublication{
				Channels: []string{"foo.bar", "foo.bar::someid"},
				Msg: decodedPublicationMessage{
					Event: "i",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{"some"},
				},
				OplogTimestamp: primitive.Timestamp{T: 1234},
				ParallelismKey: fooHash,
			},
		},
		"Replacement update": {
			in: &oplogEntry{
				DocID:      "someid",
				Operation:  "u",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
				Data: rawBson(t, bson.M{
					"some": "field",
					"new":  "field",
				}),
				Timestamp: primitive.Timestamp{T: 1234},
			},
			want: &decodedPublication{
				Channels: []string{"foo.bar", "foo.bar::someid"},
				Msg: decodedPublicationMessage{
					Event: "u",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{"some", "new"},
				},
				OplogTimestamp: primitive.Timestamp{T: 1234},
				ParallelismKey: fooHash,
			},
		},
		"Non-replacement update": {
			in: &oplogEntry{
				DocID:      "someid",
				Operation:  "u",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
				Data: rawBson(t, bson.M{
					"$v": "1.2.3",
					"$set": map[string]interface{}{
						"a": "foo",
						"b": "foo",
					},
					"$unset": map[string]interface{}{
						"c": "foo",
					},
				}),
				Timestamp: primitive.Timestamp{T: 1234},
			},
			want: &decodedPublication{
				Channels: []string{"foo.bar", "foo.bar::someid"},
				Msg: decodedPublicationMessage{
					Event: "u",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{"a", "b", "c"},
				},
				OplogTimestamp: primitive.Timestamp{T: 1234},
				ParallelismKey: fooHash,
			},
		},
		"Delete": {
			in: &oplogEntry{
				DocID:      "someid",
				Operation:  "d",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
				Data:       rawBson(t, bson.M{}),
				Timestamp:  primitive.Timestamp{T: 1234},
			},
			want: &decodedPublication{
				Channels: []string{"foo.bar", "foo.bar::someid"},
				Msg: decodedPublicationMessage{
					Event: "r",
					Doc: map[string]interface{}{
						"_id": "someid",
					},
					Fields: []string{},
				},
				OplogTimestamp: primitive.Timestamp{T: 1234},
				ParallelismKey: fooHash,
			},
		},
		"ObjectID id": {
			in: &oplogEntry{
				DocID:      testObjectId,
				Operation:  "i",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
				Data: rawBson(t, bson.M{
					"some": "field",
				}),
				Timestamp: primitive.Timestamp{T: 1234},
			},
			want: &decodedPublication{
				Channels: []string{"foo.bar", "foo.bar::deadbeefdeadbeefdeadbeef"},
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
				OplogTimestamp: primitive.Timestamp{T: 1234},
				ParallelismKey: fooHash,
			},
		},
		"Unsupported id type": {
			in: &oplogEntry{
				DocID:      1234,
				Operation:  "i",
				Namespace:  "foo.bar",
				Database:   "foo",
				Collection: "bar",
				Data: rawBson(t, bson.M{
					"some": "field",
				}),
				Timestamp: primitive.Timestamp{T: 1234},
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
				Data: rawBson(t, bson.M{
					"some": "field",
				}),
				Timestamp: primitive.Timestamp{T: 1234},
			},
			want: nil,
		},
		"Config update": {
			in: &oplogEntry{
				DocID: bson.M{
					"id":  bson.M{"Subtype": 4, "Data": "dGVzdA=="},
					"uid": bson.M{"Subtype": 0, "Data": "MTIz"},
				},
				Operation:  "d",
				Namespace:  "config.transactions",
				Database:   "config",
				Collection: "transactions",
				TxIdx:      0,
				Data: rawBson(t, bson.M{
					"_id": bson.M{
						"id":  bson.M{"Subtype": 4, "Data": "dGVzdA=="},
						"uid": bson.M{"Subtype": 0, "Data": "MTIz"},
					},
				}),
				Timestamp: primitive.Timestamp{T: 1636616135},
			},
			want: nil,
		},
	}

	// helper to convert a redispub.Publication to a decodedPublication
	decodePublication := func(pub *redispub.Publication) *decodedPublication {
		if pub == nil {
			return nil
		}

		msg := decodedPublicationMessage{}
		err := json.Unmarshal(pub.Msg, &msg)
		if err != nil {
			panic(fmt.Sprintf("Error parsing Msg field of publication: %s\n    JSON: %s",
				err, pub.Msg))
		}

		sort.Strings(msg.Fields)

		return &decodedPublication{
			Channels:       pub.Channels,
			Msg:            msg,
			OplogTimestamp: pub.OplogTimestamp,
			ParallelismKey: pub.ParallelismKey,
		}
	}

	for testName, test := range tests {
		if test.want != nil {
			sort.Strings(test.want.Msg.Fields)
		}

		t.Run(testName, func(t *testing.T) {
			// Create an output channel. We create a buffered channel so that
			// we can run Tail

			got, err := processOplogEntry(test.in)

			if test.wantError != nil {
				assert.EqualError(t, errors.Cause(err), test.wantError.Error())
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.want, decodePublication(got))
		})
	}
}
