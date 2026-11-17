package oplog

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

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
		WallTime       time.Time
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
				WallTime:  time.Unix(1234, 0).UTC(),
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
				WallTime:       time.Unix(1234, 0).UTC(),
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
				WallTime:  time.Unix(1234, 0).UTC(),
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
				WallTime:       time.Unix(1234, 0).UTC(),
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
				WallTime:  time.Unix(1234, 0).UTC(),
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
				WallTime:       time.Unix(1234, 0).UTC(),
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
				WallTime:   time.Unix(1234, 0).UTC(),
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
				WallTime:       time.Unix(1234, 0).UTC(),
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
				WallTime:  time.Unix(1234, 0).UTC(),
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
				WallTime:       time.Unix(1234, 0).UTC(),
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
				WallTime:  time.Unix(1234, 0).UTC(),
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
				WallTime:  time.Unix(1234, 0).UTC(),
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
				WallTime:  time.Unix(1234, 0).UTC(),
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
			WallTime:       pub.WallTime,
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

// encodingJSONMessage is the outgoing message as encoding/json writes it,
// which is what appendOutgoingMessage has to reproduce byte for byte.
func encodingJSONMessage(t testing.TB, event, id string, idIsObjectID bool, fields []string) []byte {
	type outgoingMessageDocument struct {
		ID interface{} `json:"_id"`
	}
	type outgoingMessage struct {
		Event  string                  `json:"e"`
		Doc    outgoingMessageDocument `json:"d"`
		Fields []string                `json:"f"`
	}
	var idForMessage interface{} = id
	if idIsObjectID {
		idForMessage = map[string]string{"$type": "oid", "$value": id}
	}
	out, err := json.Marshal(&outgoingMessage{Event: event, Doc: outgoingMessageDocument{idForMessage}, Fields: fields})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAppendOutgoingMessageMatchesEncodingJSON(t *testing.T) {
	strs := []string{
		"", "someid", "deadbeefdeadbeefdeadbeef", "a.b.c", "with space", "quote\"inside",
		"back\\slash", "<script>", "a&b", "x>y", "tab\there", "new\nline", "cr\rx",
		"\b\f", "\x00\x01\x1f", "\x7f", "caf\u00e9", "\u2028\u2029", "emoji \U0001F600",
		"bad\xffutf8", "\xc3", "$set", "_id",
	}
	fieldSets := [][]string{nil, {}, {"a"}, {"a", "b.c", "d"}}
	for _, s := range strs {
		fieldSets = append(fieldSets, []string{s, "plain"})
	}

	for _, event := range []string{"i", "u", "r"} {
		for _, id := range strs {
			for _, isOID := range []bool{false, true} {
				for _, fields := range fieldSets {
					want := encodingJSONMessage(t, event, id, isOID, fields)
					got := appendOutgoingMessage(nil, event, id, isOID, fields)
					if string(got) != string(want) {
						t.Fatalf("event=%q id=%q oid=%v fields=%q\n got: %s\nwant: %s", event, id, isOID, fields, got, want)
					}
				}
			}
		}
	}
}

func TestAppendOutgoingMessageMatchesEncodingJSONOnRandomStrings(t *testing.T) {
	// Bytes weighted toward the ones encoding/json escapes or rewrites.
	alphabet := []byte("ab.$_09 \"\\<>&\t\n\x00\x1f\x7f\x80\xc3\xa9\xe2\x80\xa8\xff")
	rng := rand.New(rand.NewSource(1))
	randomString := func() string {
		b := make([]byte, rng.Intn(12))
		for i := range b {
			b[i] = alphabet[rng.Intn(len(alphabet))]
		}
		return string(b)
	}
	for i := 0; i < 5000; i++ {
		id := randomString()
		var fields []string
		if rng.Intn(4) > 0 {
			fields = make([]string, rng.Intn(4))
			for j := range fields {
				fields[j] = randomString()
			}
		}
		isOID := rng.Intn(2) == 0
		want := encodingJSONMessage(t, "u", id, isOID, fields)
		got := appendOutgoingMessage(nil, "u", id, isOID, fields)
		if string(got) != string(want) {
			t.Fatalf("id=%q oid=%v fields=%q\n got: %s\nwant: %s", id, isOID, fields, got, want)
		}
	}
}

func TestAppendOutgoingMessageAppendsToExistingBuffer(t *testing.T) {
	prefix := []byte("prefix:")
	got := appendOutgoingMessage(prefix, "i", "someid", false, []string{"a"})
	want := `prefix:{"e":"i","d":{"_id":"someid"},"f":["a"]}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
