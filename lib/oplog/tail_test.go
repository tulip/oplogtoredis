package oplog

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/go-redis/redis/v8"
	"github.com/kylelemons/godebug/pretty"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tulip/oplogtoredis/lib/config"
)

func encodeMongoTimestamp(ts primitive.Timestamp) string {
	return strconv.FormatUint(uint64(ts.T)<<32|uint64(ts.I), 10)
}

// Converts a time to a mongo timestamp
func mongoTS(d time.Time) primitive.Timestamp {
	return primitive.Timestamp{
		T: uint32(d.Unix()), // Extract Unix seconds and cast to uint32
		I: 0,
	}
}

// Determines if two dates are within a delta
func timestampsWithinDelta(d1, d2 primitive.Timestamp, delta time.Duration) bool {
	d1Seconds := int64(d1.T)
	d2Seconds := int64(d2.T)

	diff := d1Seconds - d2Seconds
	if diff < 0 {
		diff = -diff
	}
	return float64(diff) <= delta.Seconds()
}

func TestGetStartTime(t *testing.T) {
	// Use a tiny retry delay so the persistent-failure case, which exhausts all
	// retries, completes well within the unit test timeout (CI runs -timeout 5s).
	t.Setenv("OTR_RESUME_TS_READ_RETRY_DELAY", "1ms")
	require.NoError(t, config.ParseEnv())

	now := time.Now()
	maxCatchUp := time.Minute
	notTooOld := now.Add(-30 * time.Second)
	tooOld := now.Add(-120 * time.Second)

	tests := map[string]struct {
		redisTimestamp     primitive.Timestamp
		mongoEndOfOplog    primitive.Timestamp
		mongoEndOfOplogErr error
		redisErr           string
		// redisErrAttempts is the number of leading GET calls that should fail with
		// redisErr. Use a large value to simulate a persistent (unrecoverable) failure.
		redisErrAttempts int
		expectedResult   primitive.Timestamp
		expectedErr      bool
		// resumeFromEndOnFailure sets the OTR_RESUME_FROM_END_ON_FAILURE escape hatch
		// for this subtest.
		resumeFromEndOnFailure bool
		// expectMongoFallback indicates whether getStartTime should consult the Mongo
		// end-of-oplog fallback. On a persistent Redis read failure it must NOT, since
		// that would silently skip oplog entries.
		expectMongoFallback bool
	}{
		"Start time is in Redis": {
			redisTimestamp: mongoTS(notTooOld),
			expectedResult: mongoTS(notTooOld),
		},
		"Start time is in redis, but too old": {
			redisTimestamp:      mongoTS(tooOld),
			mongoEndOfOplog:     mongoTS(notTooOld),
			expectedResult:      mongoTS(notTooOld),
			expectMongoFallback: true,
		},
		"Start time not in Redis": {
			// We use tooOld here to make sure we're not applying any kind
			// of cutoff to the latest oplog entry -- it's always fine to use
			// that regardless of how old it is
			mongoEndOfOplog:     mongoTS(tooOld),
			expectedResult:      mongoTS(tooOld),
			expectMongoFallback: true,
		},
		"Start time not in Redis, Mongo errors": {
			mongoEndOfOplogErr:  errors.New("Some mongo error"),
			expectedResult:      mongoTS(now),
			expectedErr:         true,
			expectMongoFallback: true,
		},
		"Start time is in Redis, redis errors first few attempts": {
			redisTimestamp:   mongoTS(notTooOld),
			expectedResult:   mongoTS(notTooOld),
			redisErr:         "Some transient redis error",
			redisErrAttempts: 3,
		},
		"Start time read from Redis fails persistently": {
			// Every read attempt fails. We must NOT fall back to the end of the
			// oplog (which would skip entries); instead getStartTime should return
			// an error so the tailer restarts and retries.
			redisTimestamp:      mongoTS(notTooOld),
			redisErr:            "redis: all sentinels specified in configuration are unreachable",
			redisErrAttempts:    1000,
			expectedErr:         true,
			expectMongoFallback: false,
		},
		"Start time read from Redis fails persistently, escape hatch enabled": {
			// With OTR_RESUME_FROM_END_ON_FAILURE set, a persistent read failure
			// falls back to the end of the oplog (the pre-retry behavior) instead
			// of returning an error.
			redisTimestamp:         mongoTS(notTooOld),
			mongoEndOfOplog:        mongoTS(tooOld),
			redisErr:               "redis: all sentinels specified in configuration are unreachable",
			redisErrAttempts:       1000,
			resumeFromEndOnFailure: true,
			expectedResult:         mongoTS(tooOld),
			expectMongoFallback:    true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			// Set explicitly (and re-parse) every subtest so config state from the
			// escape-hatch case doesn't leak into others via the shared global config
			// (subtests run in randomized order).
			if test.resumeFromEndOnFailure {
				t.Setenv("OTR_RESUME_FROM_END_ON_FAILURE", "true")
			} else {
				t.Setenv("OTR_RESUME_FROM_END_ON_FAILURE", "false")
			}
			require.NoError(t, config.ParseEnv())

			redisServer, err := miniredis.Run()
			if err != nil {
				panic(err)
			}
			defer redisServer.Close()

			redisGetErrorCount := 0
			// Register a hook so we can fail the configured number of leading GETs
			redisServer.Server().SetPreHook(func(c *server.Peer, cmd string, args ...string) bool {
				if cmd == "GET" && test.redisErr != "" && redisGetErrorCount < test.redisErrAttempts {
					redisGetErrorCount += 1
					c.WriteError(test.redisErr)
					return true
				}
				return false
			})

			require.NoError(t, redisServer.Set("someprefix.lastProcessedEntry.0", encodeMongoTimestamp(test.redisTimestamp)))

			redisClient := []redis.UniversalClient{redis.NewUniversalClient(&redis.UniversalOptions{
				Addrs: []string{redisServer.Addr()},
			})}

			tailer := Tailer{
				RedisClients: redisClient,
				RedisPrefix:  "someprefix.",
				MaxCatchUp:   maxCatchUp,
				Denylist:     &sync.Map{},
			}

			mongoFallbackCalled := false
			actualResult, err := tailer.getStartTime(0, func() (*primitive.Timestamp, error) {
				mongoFallbackCalled = true
				if test.mongoEndOfOplogErr != nil {
					return nil, test.mongoEndOfOplogErr
				}

				return &test.mongoEndOfOplog, nil
			})

			if test.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, test.expectMongoFallback, mongoFallbackCalled,
				"unexpected use of the Mongo end-of-oplog fallback")

			// On a persistent Redis read failure we must not return a usable start
			// time at all, so skip the timestamp comparison for that case.
			if !test.expectedErr || test.expectMongoFallback {
				expectedResult := test.expectedResult
				if test.mongoEndOfOplogErr != nil {
					// The Mongo-error fallback returns the current time, so compare against
					// the clock now rather than a value captured at the start of the test
					// (other slow subtests can otherwise push us past the delta).
					expectedResult = mongoTS(time.Now())
				}
				if !timestampsWithinDelta(actualResult, expectedResult, time.Second) {
					t.Errorf("Result was incorrect. Got %d, expected %d", actualResult, expectedResult)
				}
			}
		})
	}
}

func TestParseRawOplogEntry(t *testing.T) {
	tests := map[string]struct {
		in   rawOplogEntry
		want []oplogEntry
	}{
		"Insert": {
			in: rawOplogEntry{
				Timestamp: primitive.Timestamp{T: 1234},
				WallTime:  time.Unix(1234, 0).UTC(),
				Operation: "i",
				Namespace: "foo.Bar",
				Doc:       rawBson(t, map[string]interface{}{"_id": "someid", "foo": "bar"}),
			},
			want: []oplogEntry{{
				Timestamp:  primitive.Timestamp{T: 1234},
				WallTime:   time.Unix(1234, 0).UTC(),
				Operation:  "i",
				Namespace:  "foo.Bar",
				Data:       rawBson(t, map[string]interface{}{"_id": "someid", "foo": "bar"}),
				DocID:      interface{}("someid"),
				Database:   "foo",
				Collection: "Bar",
			}},
		},
		"Update": {
			in: rawOplogEntry{
				Timestamp: primitive.Timestamp{T: 1234},
				WallTime:  time.Unix(1234, 0).UTC(),
				Operation: "u",
				Namespace: "foo.Bar",
				Doc:       rawBson(t, map[string]interface{}{"new": "data"}),
				Update:    rawBson(t, map[string]interface{}{"_id": "updateid"}),
			},
			want: []oplogEntry{{
				Timestamp:  primitive.Timestamp{T: 1234},
				WallTime:   time.Unix(1234, 0).UTC(),
				Operation:  "u",
				Namespace:  "foo.Bar",
				Data:       rawBson(t, map[string]interface{}{"new": "data"}),
				DocID:      interface{}("updateid"),
				Database:   "foo",
				Collection: "Bar",
			}},
		},
		"Remove": {
			in: rawOplogEntry{
				Timestamp: primitive.Timestamp{T: 1234},
				WallTime:  time.Unix(1234, 0).UTC(),
				Operation: "d",
				Namespace: "foo.Bar",
				Doc:       rawBson(t, map[string]interface{}{"_id": "someid"}),
			},
			want: []oplogEntry{{
				Timestamp:  primitive.Timestamp{T: 1234},
				WallTime:   time.Unix(1234, 0).UTC(),
				Operation:  "d",
				Namespace:  "foo.Bar",
				Data:       rawBson(t, map[string]interface{}{"_id": "someid"}),
				DocID:      interface{}("someid"),
				Database:   "foo",
				Collection: "Bar",
			}},
		},
		"Command": {
			in: rawOplogEntry{
				Timestamp: primitive.Timestamp{T: 1234},
				WallTime:  time.Unix(1234, 0).UTC(),
				Operation: "c",
				Namespace: "foo.$cmd",
				Doc:       rawBson(t, map[string]interface{}{"drop": "Foo"}),
			},
			want: nil,
		},
		"Transaction": {
			in: rawOplogEntry{
				Timestamp: primitive.Timestamp{T: 1234},
				WallTime:  time.Unix(1234, 0).UTC(),
				Operation: "c",
				Namespace: "admin.$cmd",
				Doc: rawBson(t, map[string]interface{}{
					"applyOps": []rawOplogEntry{
						{
							Timestamp: primitive.Timestamp{T: 1234},
							WallTime:  time.Unix(1234, 0).UTC(),
							Operation: "c",
							Namespace: "admin.$cmd",
							Doc: rawBson(t, map[string]interface{}{
								"applyOps": []rawOplogEntry{
									{
										Operation: "i",
										Namespace: "foo.Bar",
										Doc: rawBson(t, map[string]interface{}{
											"_id": "id1",
											"foo": "baz",
										}),
										Update: rawBson(t, map[string]interface{}{}),
									},
								},
							}),
							Update: rawBson(t, map[string]interface{}{}),
						},
						{
							Operation: "i",
							Namespace: "foo.Bar",
							Doc: rawBson(t, map[string]interface{}{
								"_id": "id1",
								"foo": "bar",
							}),
							Update: rawBson(t, map[string]interface{}{}),
						},
						{
							Operation: "u",
							Namespace: "foo.Bar",
							Doc: rawBson(t, map[string]interface{}{
								"foo": "quux",
							}),
							Update: rawBson(t, map[string]interface{}{"_id": "id2"}),
						},
						{
							Operation: "d",
							Namespace: "foo.Bar",
							Doc: rawBson(t, map[string]interface{}{
								"_id": "id3",
							}),
							Update: rawBson(t, map[string]interface{}{}),
						},
					},
				}),
			},
			want: []oplogEntry{
				{
					DocID:      "id1",
					Timestamp:  primitive.Timestamp{T: 1234},
					WallTime:   time.Unix(1234, 0).UTC(),
					Operation:  "i",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: rawBson(t, map[string]interface{}{
						"_id": "id1",
						"foo": "baz",
					}),
					TxIdx: 0,
				},
				{
					DocID:      "id1",
					Timestamp:  primitive.Timestamp{T: 1234},
					WallTime:   time.Unix(1234, 0).UTC(),
					Operation:  "i",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: rawBson(t, map[string]interface{}{
						"_id": "id1",
						"foo": "bar",
					}),
					TxIdx: 1,
				},
				{
					DocID:      "id2",
					Timestamp:  primitive.Timestamp{T: 1234},
					WallTime:   time.Unix(1234, 0).UTC(),
					Operation:  "u",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: rawBson(t, map[string]interface{}{
						"foo": "quux",
					}),
					TxIdx: 2,
				},
				{
					DocID:      "id3",
					Timestamp:  primitive.Timestamp{T: 1234},
					WallTime:   time.Unix(1234, 0).UTC(),
					Operation:  "d",
					Namespace:  "foo.Bar",
					Database:   "foo",
					Collection: "Bar",
					Data: rawBson(t, map[string]interface{}{
						"_id": "id3",
					}),
					TxIdx: 3,
				},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			got := (&Tailer{Denylist: &sync.Map{}}).parseRawOplogEntry(&test.in, nil)

			if diff := pretty.Compare(parseEntry(t, got), parseEntry(t, test.want)); diff != "" {
				t.Errorf("Got incorrect result (-got +want)\n%s", diff)
			}
		})
	}
}

type oplogEntryConverted struct {
	DocID      interface{}
	Timestamp  primitive.Timestamp
	WallTime   time.Time
	Data       map[string]interface{}
	Operation  string
	Namespace  string
	Database   string
	Collection string

	TxIdx uint
}

func parseEntry(t *testing.T, op []oplogEntry) []oplogEntryConverted {
	opc := make([]oplogEntryConverted, len(op))

	for i := 0; i < len(op); i++ {
		data := map[string]interface{}{}
		err := bson.Unmarshal(op[i].Data, &data)
		if err != nil {
			t.Error("Error unmarshalling oplog data", err)
		}
		opc[i].DocID = op[i].DocID
		opc[i].Timestamp = op[i].Timestamp
		opc[i].WallTime = op[i].WallTime
		opc[i].Data = data
		opc[i].Operation = op[i].Operation
		opc[i].Namespace = op[i].Namespace
		opc[i].Database = op[i].Database
		opc[i].Collection = op[i].Collection
		opc[i].TxIdx = op[i].TxIdx
	}
	return opc
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

func TestPrematureStopEscalation(t *testing.T) {
	// A run that lasted at least minSuccessfulTailDuration resets the streak,
	// so an isolated premature stop after healthy operation stays a warning.
	require.Equal(t, 1, nextPrematureStopStreak(4, minSuccessfulTailDuration))
	require.False(t, prematureStopIsError(nextPrematureStopStreak(4, minSuccessfulTailDuration)))

	// A run shorter than minSuccessfulTailDuration counts toward the streak.
	require.Equal(t, 5, nextPrematureStopStreak(4, minSuccessfulTailDuration-time.Millisecond))

	// Repeated rapid failures eventually escalate to an error.
	streak := 0
	for i := 1; i < maxConsecutivePrematureStops; i++ {
		streak = nextPrematureStopStreak(streak, 0)
		require.Falsef(t, prematureStopIsError(streak), "streak %d should be a warning", streak)
	}
	streak = nextPrematureStopStreak(streak, 0)
	require.Equal(t, maxConsecutivePrematureStops, streak)
	require.True(t, prematureStopIsError(streak), "streak should escalate to error at the threshold")

	// Once a healthy run occurs, the streak resets back below the threshold.
	streak = nextPrematureStopStreak(streak, minSuccessfulTailDuration)
	require.False(t, prematureStopIsError(streak))
}
