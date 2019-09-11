package oplog

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/globalsign/mgo/bson"
	"github.com/go-redis/redis"
	"github.com/kylelemons/godebug/pretty"
	"github.com/stretchr/testify/require"
)

// Converts a time to a mongo timestamp
func mongoTS(d time.Time) bson.MongoTimestamp {
	return bson.MongoTimestamp(d.Unix() << 32)
}

// Determines if two dates are within a delta
func timestampsWithinDelta(d1, d2 bson.MongoTimestamp, delta time.Duration) bool {
	d1Seconds := int64(d1) >> 32
	d2Seconds := int64(d2) >> 32

	diff := d1Seconds - d2Seconds
	if diff < 0 {
		diff = -diff
	}

	return float64(diff) <= delta.Seconds()
}

func TestGetStartTime(t *testing.T) {
	now := time.Now()
	maxCatchUp := time.Minute
	notTooOld := now.Add(-30 * time.Second)
	tooOld := now.Add(-120 * time.Second)

	tests := map[string]struct {
		redisTimestamp     bson.MongoTimestamp
		mongoEndOfOplog    bson.MongoTimestamp
		mongoEndOfOplogErr error
		expectedResult     bson.MongoTimestamp
	}{
		"Start time is in Redis": {
			redisTimestamp: mongoTS(notTooOld),
			expectedResult: mongoTS(notTooOld),
		},
		"Start time is in redis, but too old": {
			redisTimestamp:  mongoTS(tooOld),
			mongoEndOfOplog: mongoTS(notTooOld),
			expectedResult:  mongoTS(notTooOld),
		},
		"Start time not in Redis": {
			// We use tooOld here to make sure we're not applying any kind
			// of cutoff to the latest oplog entry -- it's always fine to use
			// that regardless of how old it is
			mongoEndOfOplog: mongoTS(tooOld),
			expectedResult:  mongoTS(tooOld),
		},
		"Start time not in Redis, Mongo errors": {
			mongoEndOfOplogErr: errors.New("Some mongo error"),
			expectedResult:     mongoTS(now),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			redisServer, err := miniredis.Run()
			if err != nil {
				panic(err)
			}
			defer redisServer.Close()
			require.NoError(t, redisServer.Set("someprefix.lastProcessedEntry", strconv.FormatInt(int64(test.redisTimestamp), 10)))

			redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
				Addrs: []string{redisServer.Addr()},
			})

			tailer := Tailer{
				RedisClient: redisClient,
				RedisPrefix: "someprefix.",
				MaxCatchUp:  maxCatchUp,
			}

			actualResult := tailer.getStartTime(func() (bson.MongoTimestamp, error) {
				if test.mongoEndOfOplogErr != nil {
					return bson.MongoTimestamp(0), test.mongoEndOfOplogErr
				}

				return test.mongoEndOfOplog, nil
			})

			// We need to do an approximate comparison; the function sometimes
			// return time.Now
			if !timestampsWithinDelta(actualResult, test.expectedResult, time.Second) {
				t.Errorf("Result was incorrect. Got %d, expected %d", actualResult, test.expectedResult)
			}
		})
	}
}

func mustRaw(t *testing.T, data interface{}) bson.Raw {
	b, err := bson.Marshal(data)
	require.NoError(t, err)

	var raw bson.Raw
	require.NoError(t, bson.Unmarshal(b, &raw))

	return raw
}

func TestParseRawOplogEntry(t *testing.T) {
	tests := map[string]struct {
		in   rawOplogEntry
		want []oplogEntry
	}{
		"Insert": {
			in: rawOplogEntry{
				Timestamp: bson.MongoTimestamp(1234),
				Operation: "i",
				Namespace: "foo.Bar",
				Doc:       mustRaw(t, map[string]interface{}{"_id": "someid", "foo": "bar"}),
			},
			want: []oplogEntry{{
				Timestamp:  bson.MongoTimestamp(1234),
				Operation:  "i",
				Namespace:  "foo.Bar",
				Data:       map[string]interface{}{"_id": "someid", "foo": "bar"},
				DocID:      interface{}("someid"),
				Database:   "foo",
				Collection: "Bar",
			}},
		},
		"Update": {
			in: rawOplogEntry{
				Timestamp: bson.MongoTimestamp(1234),
				Operation: "u",
				Namespace: "foo.Bar",
				Doc:       mustRaw(t, map[string]interface{}{"new": "data"}),
				Update:    rawOplogEntryID{ID: "updateid"},
			},
			want: []oplogEntry{{
				Timestamp:  bson.MongoTimestamp(1234),
				Operation:  "u",
				Namespace:  "foo.Bar",
				Data:       map[string]interface{}{"new": "data"},
				DocID:      interface{}("updateid"),
				Database:   "foo",
				Collection: "Bar",
			}},
		},
		"Remove": {
			in: rawOplogEntry{
				Timestamp: bson.MongoTimestamp(1234),
				Operation: "d",
				Namespace: "foo.Bar",
				Doc:       mustRaw(t, map[string]interface{}{"_id": "someid"}),
			},
			want: []oplogEntry{{
				Timestamp:  bson.MongoTimestamp(1234),
				Operation:  "d",
				Namespace:  "foo.Bar",
				Data:       map[string]interface{}{"_id": "someid"},
				DocID:      interface{}("someid"),
				Database:   "foo",
				Collection: "Bar",
			}},
		},
		"Command": {
			in: rawOplogEntry{
				Timestamp: bson.MongoTimestamp(1234),
				Operation: "c",
				Namespace: "foo.$cmd",
				Doc:       mustRaw(t, map[string]interface{}{"drop": "Foo"}),
			},
			want: nil,
		},
		"Transaction": {
			in: rawOplogEntry{
				Timestamp: bson.MongoTimestamp(1234),
				Operation: "c",
				Namespace: "admin.$cmd",
				Doc: mustRaw(t, map[string]interface{}{
					"applyOps": []rawOplogEntry{
						{
							Timestamp: bson.MongoTimestamp(1234),
							Operation: "c",
							Namespace: "admin.$cmd",
							Doc: mustRaw(t, map[string]interface{}{
								"applyOps": []rawOplogEntry{
									{
										Operation: "i",
										Namespace: "foo.Bar",
										Doc: mustRaw(t, map[string]interface{}{
											"_id": "id1",
											"foo": "baz",
										}),
									},
								},
							}),
						},
						{
							Operation: "i",
							Namespace: "foo.Bar",
							Doc: mustRaw(t, map[string]interface{}{
								"_id": "id1",
								"foo": "bar",
							}),
						},
						{
							Operation: "u",
							Namespace: "foo.Bar",
							Doc: mustRaw(t, map[string]interface{}{
								"foo": "quux",
							}),
							Update: rawOplogEntryID{"id2"},
						},
						{
							Operation: "d",
							Namespace: "foo.Bar",
							Doc: mustRaw(t, map[string]interface{}{
								"_id": "id3",
							}),
						},
					},
				}),
			},
			want: []oplogEntry{
				{
					DocID:      "id1",
					Timestamp:  1234,
					Operation:  "i",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: map[string]interface{}{
						"_id": "id1",
						"foo": "baz",
					},
				},
				{
					DocID:      "id1",
					Timestamp:  1234,
					Operation:  "i",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: map[string]interface{}{
						"_id": "id1",
						"foo": "bar",
					},
				},
				{
					DocID:      "id2",
					Timestamp:  1234,
					Operation:  "u",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: map[string]interface{}{
						"foo": "quux",
					},
				},
				{
					DocID:      "id3",
					Timestamp:  1234,
					Operation:  "d",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: map[string]interface{}{
						"_id": "id3",
					},
				},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got := (&Tailer{}).parseRawOplogEntry(test.in, nil)

			if diff := pretty.Compare(got, test.want); diff != "" {
				t.Errorf("Got incorrect result (-got +want)\n%s", diff)
			}
		})
	}
}

func TestParseNamespace(t *testing.T) {
	tests := map[string]struct {
		in             string
		wantDB         string
		wantCollection string
	}{
		"DB and collection": {
			in:             "foo.bar",
			wantDB:         "foo",
			wantCollection: "bar",
		},
		"Dot in collection": {
			in:             "foo.system.indexes",
			wantDB:         "foo",
			wantCollection: "system.indexes",
		},
		"DB only": {
			in:             "foo",
			wantDB:         "foo",
			wantCollection: "",
		},
		"Empty string": {
			in:             "",
			wantDB:         "",
			wantCollection: "",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			gotDB, gotCollection := parseNamespace(test.in)

			if (gotDB != test.wantDB) || (gotCollection != test.wantCollection) {
				t.Errorf("parseNamespace(%s) = %s, %s; want %s, %s",
					test.in, gotDB, gotCollection, test.wantDB, test.wantCollection)
			}
		})
	}
}
