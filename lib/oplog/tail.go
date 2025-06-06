// Package oplog tails a MongoDB oplog, process each message, and generates
// the message that should be sent to Redis. It writes these to an output
// channel that should be read by package redispub and sent to the Redis server.
package oplog

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tulip/oplogtoredis/lib/config"
	"github.com/tulip/oplogtoredis/lib/log"
	"github.com/tulip/oplogtoredis/lib/redispub"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.mongodb.org/mongo-driver/bson"
)

// Tailer persistently tails the oplog of a Mongo cluster, handling
// reconnection and resumption of where it left off.
type Tailer struct {
	MongoClient  *mongo.Client
	RedisClients []redis.UniversalClient
	RedisPrefix  string
	MaxCatchUp   time.Duration
	Denylist     *sync.Map
}

// Raw oplog entry from Mongo
type rawOplogEntry struct {
	Timestamp primitive.Timestamp `bson:"ts"`
	Operation string              `bson:"op"`
	Namespace string              `bson:"ns"`
	Doc       bson.Raw            `bson:"o"`
	Update    bson.Raw            `bson:"o2"`
}

// Parsed Cursor Result
type cursorResultStatus struct {
	GotResult       bool
	DidTimeout      bool
	DidLosePosition bool
}

const requeryDuration = time.Second

var (
	// Deprecated: use metricOplogEntriesBySize instead
	metricOplogEntriesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "otr",
		Subsystem: "oplog",
		Name:      "entries_received",
		Help:      "[Deprecated] Oplog entries received, partitioned by database and status",
	}, []string{"database", "status"})

	// Deprecated: use metricOplogEntriesBySize instead
	metricOplogEntriesReceivedSize = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "otr",
		Subsystem: "oplog",
		Name:      "entries_received_size",
		Help:      "[Deprecated] Size of oplog entries received in bytes, partitioned by database",
	}, []string{"database"})

	metricOplogEntriesBySize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "otr",
		Subsystem: "oplog",
		Name:      "entries_by_size",
		Help:      "Histogram of oplog entries received by size in bytes, partitioned by database and status.",
		Buckets:   append([]float64{0}, prometheus.ExponentialBuckets(8, 2, 29)...),
	}, []string{"database", "status"})

	metricMaxOplogEntryByMinute = NewIntervalMaxMetricVec(&IntervalMaxVecOpts{
		IntervalMaxOpts: IntervalMaxOpts{
			Opts: prometheus.Opts{
				Namespace: "otr",
				Subsystem: "oplog",
				Name:      "entries_max_size",
				Help:      "Gauge recording maximum size recorded in the last minute, partitioned by database and status",
			},

			ReportInterval: 1 * time.Minute,
		},
	}, []string{"database", "status"})

	metricOplogEntriesFiltered = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "otr",
		Subsystem: "oplog",
		Name:      "entries_filtered",
		Help:      "Oplog entries filtered by denylist",
	}, []string{"database"})

	metricLastReceivedStaleness = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "otr",
		Subsystem: "oplog",
		Name:      "last_received_staleness",
		Help:      "Gauge recording the difference between this server's clock and the timestamp on the last read oplog entry.",
	}, []string{"ordinal"})

	metricOplogResumeGap = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "otr",
		Subsystem: "oplog",
		Name:      "resume_gap_seconds",
		Help:      "Histogram recording the gap in time that a tailing resume had to catchup and whether it was successful or not.",
		Buckets:   []float64{1, 2.5, 5, 10, 25, 50, 100, 250, 500, 1000},
	}, []string{"status"})
)

func init() {
	prometheus.MustRegister(metricMaxOplogEntryByMinute)
}

// PublisherChannels represents a collection of intake channels for a set of Redis Publishers.
// When multiple redis URLs are specified via OTR_REDIS_URL, each one produce a redis client,
// publisher coroutine, and intake channel. Since we want every message to go to all redis
// destinations, the tailer should send each message to all channels in the array.
type PublisherChannels []chan<- *redispub.Publication

// Tail begins tailing the oplog. It doesn't return unless it receives a message
// on the stop channel, in which case it wraps up its work and then returns.
func (tailer *Tailer) Tail(out []PublisherChannels, stop <-chan bool, readOrdinal, readParallelism int) {
	childStopC := make(chan bool)
	wasStopped := false

	go func() {
		<-stop
		wasStopped = true
		childStopC <- true
	}()

	for {
		log.Log.Info("Starting oplog tailing")
		tailer.tailOnce(out, childStopC, readOrdinal, readParallelism)
		log.Log.Info("Oplog tailing ended")

		if wasStopped {
			return
		}

		log.Log.Errorw("Oplog tailing stopped prematurely. Waiting a second an then retrying.")
		time.Sleep(requeryDuration)
	}
}

// this accepts an array of PublisherChannels instances whose size is equal to the degree of write-parallelism.
// Each incoming message will be routed to one of the PublisherChannels instances based on its parallelism key
// (hash of the database name), then sent to every channel within that PublisherChannels instance.
func (tailer *Tailer) tailOnce(out []PublisherChannels, stop <-chan bool, readOrdinal, readParallelism int) {
	session, err := tailer.MongoClient.StartSession()
	if err != nil {
		log.Log.Errorw("Failed to start Mongo session", "error", err)
		return
	}

	oplogCollection := session.Client().Database("local").Collection("oplog.rs")

	startTime := tailer.getStartTime(len(out)-1, func() (*primitive.Timestamp, error) {
		// Get the timestamp of the last entry in the oplog (as a position to
		// start from if we don't have a last-written timestamp from Redis)
		var entry rawOplogEntry
		findOneOpts := &options.FindOneOptions{}
		findOneOpts.SetSort(bson.M{"$natural": -1})

		queryContext, queryContextCancel := context.WithTimeout(context.Background(), config.MongoQueryTimeout())
		defer queryContextCancel()

		result := oplogCollection.FindOne(queryContext, bson.M{}, findOneOpts)

		if result.Err() != nil {
			return nil, result.Err()
		}

		decodeErr := result.Decode(&entry)

		if decodeErr != nil {
			return nil, decodeErr
		}

		log.Log.Infow("Got latest oplog entry",
			"timestamp", entry.Timestamp,
			"operation", entry.Operation,
			"namespace", entry.Namespace)
		ts := entry.Timestamp
		return &ts, nil
	})

	query, queryErr := issueOplogFindQuery(oplogCollection, startTime)

	if queryErr != nil {
		log.Log.Errorw("Error issuing tail query", "error", queryErr)
		return
	}

	lastTimestamp := startTime
	for {
		select {
		case <-stop:
			log.Log.Infof("Received stop; aborting oplog tailing")
			return
		default:
		}

		for {
			var rawData bson.Raw
			status, err := readNextFromCursor(query)

			if status.GotResult {
				decodeErr := query.Decode(&rawData)
				if decodeErr != nil {
					log.Log.Errorw("Error decoding oplog entry", "error", decodeErr)
					continue
				}

				ts, pubs, sendMetricsData := tailer.processEntry(rawData, readOrdinal)

				if ts != nil {
					lastTimestamp = *ts
				}

				// we only want to send metrics data once for the whole batch
				metricsDataSent := false

				for _, pub := range pubs {
					if pub != nil {
						inIdx := assignToShard(pub.ParallelismKey, readParallelism)
						if inIdx != readOrdinal {
							// discard this publication
							continue
						}

						// send metrics data only if we didnt discard all the publications due to sharding
						if !metricsDataSent && sendMetricsData != nil {
							metricsDataSent = true
							sendMetricsData()
						}

						// determine which shard this message should route to
						// inIdx and outIdx may be different if there are different #s of read and write routines
						outIdx := assignToShard(pub.ParallelismKey, len(out))
						// get the set of publisher channels for that shard
						pubChans := out[outIdx]
						// send the message to each channel on that shard
						for _, pubChan := range pubChans {
							pubChan <- pub
						}
					} else {
						log.Log.Error("Nil Redis publication")
					}
				}
			} else if status.DidTimeout {

				// Didn't get any messages for a while, keep trying.
				// This is normal, if there are no new messages, the iterator will
				// timeout after our timeout duration, and we'll create a new one.
				log.Log.Debug("Oplog cursor timed out, will retry")

				query, queryErr = issueOplogFindQuery(oplogCollection, lastTimestamp)

				if queryErr != nil {
					log.Log.Errorw("Error issuing tail query", "error", queryErr)
					return
				}

				break
			} else if status.DidLosePosition {
				// Our cursor expired. Make a new cursor to pick up from where we
				// left off.
				query, queryErr = issueOplogFindQuery(oplogCollection, lastTimestamp)

				if queryErr != nil {
					log.Log.Errorw("Error issuing tail query", "error", queryErr)
					return
				}

				break
			} else if err != nil {
				log.Log.Errorw("Error from oplog iterator",
					"error", query.Err())

				closeCursor(query)

				return
			} else {
				log.Log.Errorw("Got no data from cursor, but also no error. This is unexpected; restarting query")

				closeCursor(query)

				return
			}
		}
	}
}

// readNextFromCursor gets the next item from the cursor.
// err returns the last error seen by the Cursor (or context), or nil if no error has occurred.
//
//	-> err // https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo#Cursor.Err
//
// A cursor result status object is also returned with the following attrs:
//
//	-> GotResult // https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo#Cursor.Next
//	-> DidTimeout // Did the enclosing context we provided timeout?
//	   We handle this by just retrying the query
//	-> DidLosePostion (See comment below)
//	   We handle this by creating a new cursor
func readNextFromCursor(cursor *mongo.Cursor) (status cursorResultStatus, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.MongoQueryTimeout())
	defer cancel()

	status.GotResult = cursor.Next(ctx)
	err = cursor.Err()

	if err != nil {
		// Wait briefly before determining whether the failure was a timeout: because
		// the MongoDB go driver passes the context on to lower-level networking
		// components, it's possible for the query to fail *before* the context
		// is marked as timed-out
		time.Sleep(100 * time.Millisecond)
		status.DidTimeout = ctx.Err() != nil

		// check if the error is a position-lost error. These errors are best handled
		// by just re-issueing the query on the same connection; no need to surface
		// a major error and re-connect to mongo
		// From: https://github.com/rwynn/gtm/blob/e02a1f9c1b79eb5f14ed26c86a23b920589d84c9/gtm.go#L547
		var serverErr mongo.ServerError
		if errors.As(err, &serverErr) {
			// 136  : cursor capped position lost
			// 286  : change stream history lost
			// 280  : change stream fatal error
			for _, code := range []int{136, 286, 280} {
				if serverErr.HasErrorCode(code) {
					status.DidLosePosition = true
				}
			}
		}

	}

	return
}

func issueOplogFindQuery(c *mongo.Collection, startTime primitive.Timestamp) (*mongo.Cursor, error) {
	queryOpts := &options.FindOptions{}
	queryOpts.SetSort(bson.M{"$natural": 1})
	queryOpts.SetCursorType(options.TailableAwait)

	queryContext, queryContextCancel := context.WithTimeout(context.Background(), config.MongoQueryTimeout())
	defer queryContextCancel()

	return c.Find(queryContext, bson.M{
		"ts": bson.M{"$gt": startTime},
	}, queryOpts)
}

func closeCursor(cursor *mongo.Cursor) {
	queryContext, queryContextCancel := context.WithTimeout(context.Background(), config.MongoQueryTimeout())
	defer queryContextCancel()

	closeErr := cursor.Close(queryContext)
	if closeErr != nil {
		log.Log.Errorw("Error from closing oplog iterator",
			"error", closeErr)
	}
}

// processEntry processes a single entry from the oplog.
//
// The timestamp of the entry is returned so that tailOnce knows the timestamp of the last entry it read, even if it
// ignored it or failed at some later step.
func (tailer *Tailer) processEntry(rawData bson.Raw, readOrdinal int) (timestamp *primitive.Timestamp, pubs []*redispub.Publication, sendMetricsData func()) {
	result := tailer.unmarshalEntryMetadata(rawData)
	if result == nil {
		return
	}

	timestamp = &result.Timestamp

	entries := tailer.parseRawOplogEntry(result, nil)
	log.Log.Debugw("Received oplog entry", "entry", result, "processTime", time.Now().UnixMilli())

	status := "ignored"
	database := "(no database)"
	messageLen := float64(len(rawData))

	if len(entries) > 0 {
		database = entries[0].Database
	}

	sendMetricsData = func() {
		// TODO: remove these in a future version
		metricOplogEntriesReceived.WithLabelValues(database, status).Inc()
		metricOplogEntriesReceivedSize.WithLabelValues(database).Add(messageLen)

		metricOplogEntriesBySize.WithLabelValues(database, status).Observe(messageLen)
		metricMaxOplogEntryByMinute.Report(messageLen, database, status)
		metricLastReceivedStaleness.WithLabelValues(strconv.Itoa(readOrdinal)).Set(float64(time.Since(time.Unix(int64(timestamp.T), 0))))
	}

	type errEntry struct {
		err error
		op  *oplogEntry
	}

	var errs []errEntry
	for i := range entries {
		entry := &entries[i]
		pub, err := processOplogEntry(entry)

		if err != nil {
			errs = append(errs, errEntry{
				err: err,
				op:  entry,
			})
		} else if pub != nil {
			pubs = append(pubs, pub)
		}
	}

	if errs != nil {
		status = "error"

		for _, ent := range errs {
			log.Log.Errorw("Error processing oplog entry",
				"op", ent.op,
				"error", ent.err,
				"database", ent.op.Database,
				"collection", ent.op.Database,
			)
		}
	} else if len(entries) > 0 {
		status = "processed"
	}

	return
}

// Gets the primitive.Timestamp from which we should start tailing
//
// We take the function to get the timestamp of the last oplog entry (as a
// fallback if we don't have a latest timestamp from Redis) as an arg instead
// of using tailer.mongoClient directly so we can unit test this function
func (tailer *Tailer) getStartTime(maxOrdinal int, getTimestampOfLastOplogEntry func() (*primitive.Timestamp, error)) primitive.Timestamp {
	// Get the earliest "last processed time" for each shard. This assumes that the number of shards is constant.
	ts, tsTime, redisErr := redispub.FirstLastProcessedTimestamp(tailer.RedisClients[0], tailer.RedisPrefix, maxOrdinal)

	gapSeconds := time.Since(tsTime) / time.Second

	if redisErr == nil {
		// we have a last write time, check that it's not too far in the past
		if tsTime.After(time.Now().Add(-1 * tailer.MaxCatchUp)) {
			log.Log.Infof("Found last processed timestamp, resuming oplog tailing",
				"timestamp", tsTime.Unix(),
				"age_seconds", gapSeconds)
			metricOplogResumeGap.WithLabelValues("success").Observe(float64(gapSeconds))
			return ts
		}

		log.Log.Warnw("Found last processed timestamp, but it was too far in the past. Will start from end of oplog",
			"timestamp", tsTime.Unix(),
			"age_seconds", gapSeconds)
	}

	if (redisErr != nil) && (redisErr != redis.Nil) {
		log.Log.Errorw("Error querying Redis for last processed timestamp. Will start from end of oplog.",
			"error", redisErr)
	}

	metricOplogResumeGap.WithLabelValues("failed").Observe(float64(gapSeconds))

	mongoOplogEndTimestamp, mongoErr := getTimestampOfLastOplogEntry()
	if mongoErr == nil {
		log.Log.Infow("Starting tailing from end of oplog",
			"timestamp", mongoOplogEndTimestamp.T)
		return *mongoOplogEndTimestamp
	}

	log.Log.Errorw("Got error when asking for last operation timestamp in the oplog. Returning current time.",
		"error", mongoErr)
	return primitive.Timestamp{T: uint32(time.Now().Unix() << 32)}
}

func parseID(idRaw bson.RawValue) (id interface{}, err error) {
	if idRaw.IsZero() {
		log.Log.Error("failed to get objectId: _id is empty or not set")
		err = errors.New("empty or missing objectId")
		return
	}
	err = idRaw.Unmarshal(&id)
	if err != nil {
		log.Log.Errorf("failed to unmarshal objectId: %v", err)
	}
	return
}

// unmarshalEntryMetadata processes the top-level data from an entry and returns a rawOplogEntry object.
// This avoids using bson.Unmarshal on the whole document as that has very poor performance, even with the
// bson.Raw type to limit depth.  While messy, using these raw bson methods here provides far better performance.
func (tailer *Tailer) unmarshalEntryMetadata(rawData bson.Raw) *rawOplogEntry {
	var result rawOplogEntry
	var ok bool
	nsLookup, err := rawData.LookupErr("ns")
	if err == nil {
		result.Namespace, ok = nsLookup.StringValueOK()
		if !ok {
			// this means there was a type mismatch
			log.Log.Error("Error unmarshalling oplog namespace entry")
			return nil
		}
	}

	// filter if db is in denylist
	if len(result.Namespace) > 0 && result.Namespace != "admin.$cmd" {
		db, _ := parseNamespace(result.Namespace)
		if _, denied := tailer.Denylist.Load(db); denied {
			log.Log.Debugw("Skipping oplog entry", "database", db)
			metricOplogEntriesFiltered.WithLabelValues(db).Add(1)
			return nil
		}
	}

	tsLookup, err := rawData.LookupErr("ts")
	if err == nil {
		t, i, ok := tsLookup.TimestampOK()
		if !ok {
			log.Log.Error("Error unmarshalling oplog timestamp entry")
			return nil
		}
		result.Timestamp = primitive.Timestamp{T: t, I: i}
	}

	opLookup, err := rawData.LookupErr("op")
	if err == nil {
		result.Operation, ok = opLookup.StringValueOK()
		if !ok {
			log.Log.Error("Error unmarshalling oplog operation entry")
			return nil
		}
	}

	oLookup, err := rawData.LookupErr("o")
	if err == nil {
		result.Doc, ok = oLookup.DocumentOK()
		if !ok {
			log.Log.Error("Error unmarshalling oplog document entry")
			return nil
		}
	}

	o2Lookup, err := rawData.LookupErr("o2")
	if err == nil {
		result.Update, ok = o2Lookup.DocumentOK()
		if !ok {
			log.Log.Error("Error unmarshalling oplog update entry")
			return nil
		}
	}

	return &result
}

// converts a rawOplogEntry to an oplogEntry
func (tailer *Tailer) parseRawOplogEntry(entry *rawOplogEntry, txIdx *uint) []oplogEntry {
	if txIdx == nil {
		idx := uint(0)
		txIdx = &idx
	}

	switch entry.Operation {
	case operationInsert, operationUpdate, operationRemove:
		out := oplogEntry{
			Operation: entry.Operation,
			Timestamp: entry.Timestamp,
			Namespace: entry.Namespace,
			Data:      entry.Doc,

			TxIdx: *txIdx,
		}

		*txIdx++

		out.Database, out.Collection = parseNamespace(out.Namespace)

		var errID error
		if out.Operation == operationUpdate {
			out.DocID, errID = parseID(entry.Update.Lookup("_id"))
		} else {
			out.DocID, errID = parseID(entry.Doc.Lookup("_id"))
		}
		if errID != nil {
			return nil
		}

		return []oplogEntry{out}

	case operationCommand:
		if entry.Namespace != "admin.$cmd" {
			return nil
		}

		applyOpsLookup, err := entry.Doc.LookupErr("applyOps")
		if err != nil {
			list, errList := entry.Doc.Elements()
			if errList != nil {
				log.Log.Debugf("applyOps key not found in command entry: %v, doc error: %v", err, errList)
				return nil
			}
			keys := []string{}
			for _, rawElem := range list {
				keys = append(keys, rawElem.Key())
			}
			log.Log.Debugf("applyOps key not found in command entry, ignoring. Keys: %v", strings.Join(keys, ", "))
			return nil
		}

		applyOpsValues, ok := applyOpsLookup.ArrayOK()
		if !ok {
			log.Log.Error("Failed to access transaction data as array")
			return nil
		}

		applyOpsArray, err := applyOpsValues.Values()
		if err != nil {
			log.Log.Errorf("Getting transaction ops array: %v", err)
			return nil
		}

		var ret []oplogEntry
		for _, rawEntry := range applyOpsArray {
			rawDoc, ok := rawEntry.DocumentOK()
			if ok {
				v := tailer.unmarshalEntryMetadata(rawDoc)
				if v != nil {
					v.Timestamp = entry.Timestamp
					ret = append(ret, tailer.parseRawOplogEntry(v, txIdx)...)
				}
			} else {
				log.Log.Error("Getting transaction op doc")
			}
		}

		return ret

	default:
		return nil
	}
}

// Parses op.Namespace into (database, collection)
func parseNamespace(namespace string) (string, string) {
	namespaceParts := strings.SplitN(namespace, ".", 2)

	database := namespaceParts[0]
	collection := ""
	if len(namespaceParts) > 1 {
		collection = namespaceParts[1]
	}

	return database, collection
}

// assignToShard determines which shard should process a given key.
// The key is an integer generated from random bytes, which means it can be negative.
// In Go, A%B when A is negative produces a negative result, which is inadequate as an
// array index. We fix this by doing (A%B+B)%B, which is identical to A%B for positive A
// and produces the expected result for negative A.
func assignToShard(key int, shardCount int) int {
	return (key%shardCount + shardCount) % shardCount
}
