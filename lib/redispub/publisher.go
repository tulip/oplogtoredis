// Package redispub reads messages from an input channel and publishes them to
// redis. It handles deduplicating messages (across multiple running copies of
// oplogtoredis), and tracking the timestamp of the last message we successfully
// publishes (so we can pick up from where we left off if oplogtoredis restarts).
package redispub

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/tulip/oplogtoredis/lib/log"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PublishOpts are configuration options you can pass to PublishStream.
type PublishOpts struct {
	FlushInterval    time.Duration
	DedupeExpiration time.Duration
	MetadataPrefix   string
}

// This script checks whether the keys in KEYS are set. If a key is set, it does
// nothing. If not, it sets the key, using ARGV[1] as the expiration, and then
// publishes the corresponding message from ARGV to the channels from ARGV.
// Returns an array of integers, where 1 means published and 0 means duplicate.
var publishDedupe = redis.NewScript(`
	local results = {}
	local expiration = ARGV[1]

	for i = 1, #KEYS do
		local key = KEYS[i]
		local msg = ARGV[2 + (i-1)*2]
		local channels = ARGV[3 + (i-1)*2]

		if redis.call("GET", key) == false then
			redis.call("SETEX", key, expiration, 1)
			for w in string.gmatch(channels, "([^$]+)") do
				redis.call("PUBLISH", w, msg)
			end
			table.insert(results, 1)
		else
			table.insert(results, 0)
		end
	end

	return results
`)

var metricSentMessages = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "processed_messages",
	Help:      "Messages processed by Redis publisher, partitioned by whether or not we successfully sent them and publish function index",
}, []string{"status", "idx"})

var metricTemporaryFailures = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "temporary_send_failures",
	Help:      "Number of failures encountered when trying to send a message. We automatically retry, and only register a permanent failure (in otr_redispub_processed_messages) after 30 failures.",
})

var redisCommandDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "redis_command_duration_seconds",
	Help:      "A histogram recording the duration in seconds of round trips to redis.",
	Buckets:   []float64{0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5},
}, []string{"ordinal"})

var metricStalenessPreRetries = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "pre_retry_staleness",
	Help:      "Gauge recording the staleness on receiving a message from the tailing routine.",
}, []string{"ordinal"})

var metricLastOplogEntryStaleness = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "last_entry_staleness_seconds",
	Help:      "Gauge recording the difference between this server's clock and the timestamp on the last published oplog entry.",
}, []string{"ordinal", "status"})

var metricOplogEntryStaleness = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "entry_staleness_seconds",
	Help:      "Histogram recording the difference between this server's clock and the timestamp of each processed oplog entry.",
	Buckets:   []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 3, 5, 7, 10, 20, 50, 100},
}, []string{"ordinal", "status"})

// PublishStream reads Publications from the given channel and publishes them
// to Redis.
func PublishStream(clients []redis.UniversalClient, in <-chan *Publication, opts *PublishOpts, stop <-chan bool, ordinal int) {

	// Start up a background goroutine for periodically updating the last-processed
	// timestamp
	timestampC := make(chan primitive.Timestamp)
	for _, client := range clients {
		go periodicallyUpdateTimestamp(client, timestampC, opts, ordinal)
	}

	// Redis expiration is in integer seconds, so we have to convert the
	// time.Duration
	dedupeExpirationSeconds := int(opts.DedupeExpiration.Seconds())

	type PubFn func([]*Publication) error

	var publishFns []PubFn

	for _, client := range clients {
		client := client
		publishFn := func(batch []*Publication) error {
			return publishBatch(batch, client, opts.MetadataPrefix, dedupeExpirationSeconds, ordinal)
		}
		publishFns = append(publishFns, publishFn)
	}

	publishFnsCount := len(publishFns)
	metricsSendFailed := make([]prometheus.Counter, publishFnsCount)  //metricSentMessages.WithLabelValues("failed")
	metricsSendSuccess := make([]prometheus.Counter, publishFnsCount) //metricSentMessages.WithLabelValues("sent")
	for i := 0; i < publishFnsCount; i++ {
		idx := strconv.Itoa(i)
		metricsSendFailed[i] = metricSentMessages.WithLabelValues("failed", idx)
		metricsSendSuccess[i] = metricSentMessages.WithLabelValues("sent", idx)
	}

	const batchSize = 10

	for {
		select {
		case <-stop:
			close(timestampC)
			return

		case p := <-in:
			batch := []*Publication{p}
			// Try to fill batch
		FillBatch:
			for len(batch) < batchSize {
				select {
				case p2 := <-in:
					batch = append(batch, p2)
				default:
					break FillBatch
				}
			}

			metricStalenessPreRetries.WithLabelValues(strconv.Itoa(ordinal)).Set(time.Since(batch[0].WallTime).Seconds())
			for i, publishFn := range publishFns {
				err := publishBatchWithRetries(batch, 30, time.Second, publishFn)
				log.Log.Debugw("Published to", "idx", i)

				if err != nil {
					metricsSendFailed[i].Add(float64(len(batch)))
					log.Log.Errorw("Permanent error while trying to publish message; giving up",
						"error", err,
						"batchSize", len(batch))
				} else {
					metricsSendSuccess[i].Add(float64(len(batch)))

					// We want to make sure we do this *after* we've successfully published
					// the messages
					timestampC <- batch[len(batch)-1].OplogTimestamp
				}
			}
		}
	}
}

func publishBatchWithRetries(batch []*Publication, maxRetries int, sleepTime time.Duration, publishFn func(batch []*Publication) error) error {
	if len(batch) == 0 {
		return errors.New("Empty Redis publication batch")
	}

	retries := 0
	for retries < maxRetries {
		err := publishFn(batch)

		if err != nil {
			log.Log.Errorw("Error publishing message, will retry",
				"error", err,
				"retryNumber", retries)

			// failure, retry
			metricTemporaryFailures.Inc()
			retries++
			time.Sleep(sleepTime)
		} else {
			// success, return
			return nil
		}
	}

	return errors.Errorf("sending message (retried %v times)", maxRetries)
}

func publishBatch(batch []*Publication, client redis.UniversalClient, prefix string, dedupeExpirationSeconds int, ordinal int) error {
	start := time.Now()
	ordinalStr := strconv.Itoa(ordinal)

	// Clock skew can cause the difference between Mongo's reported wall time and
	// OTR's current time to be a negative value. During a metrics scrape, if
	// these negative values cause the histogram sum to be negative, Prometheus
	// will treat it as a metric reset because the sum is otherwise assumed to be
	// monotonic. As a result, Grafana charts that use the histogram sum will
	// have artifacts, e.g. large spikes.
	//
	// The skew should only be a few milliseconds at most when using NTP, so the
	// simplest fix is to round up to 0.
	staleness := math.Max(time.Since(p.WallTime).Seconds(), 0)

	keys := make([]string, len(batch))
	args := make([]interface{}, 0, 1+len(batch)*2)
	args = append(args, dedupeExpirationSeconds)

	for i, p := range batch {
		keys[i] = formatKey(p, prefix)
		args = append(args, p.Msg, strings.Join(p.Channels, "$"))
	}

	res, err := publishDedupe.Run(
		context.Background(),
		client,
		keys,
		args...,
	).Slice()

	if err != nil {
		redisCommandDuration.WithLabelValues(ordinalStr).Observe(time.Since(start).Seconds())
		return err
	}

	for i, item := range res {
		p := batch[i]
		staleness := time.Since(p.WallTime).Seconds()

		var status string
		if item.(int64) == 1 {
			status = "published"
		} else {
			status = "duplicate"
		}
		metricLastOplogEntryStaleness.WithLabelValues(ordinalStr, status).Set(staleness)
		metricOplogEntryStaleness.WithLabelValues(ordinalStr, status).Observe(staleness)
	}

	redisCommandDuration.WithLabelValues(ordinalStr).Observe(time.Since(start).Seconds())
	return nil
}

func formatKey(p *Publication, prefix string) string {
	return fmt.Sprintf("%vprocessed::%v::%v", prefix, encodeMongoTimestamp(p.OplogTimestamp), p.TxIdx)
}

// Periodically updates the last-processed-entry timestamp in Redis.
// PublishStream sends the timestamp for *every* entry it processes to the
// channel, and this function throttles that to only update occasionally.
//
// This blocks forever; it should be run in a goroutine
func periodicallyUpdateTimestamp(client redis.UniversalClient, timestamps <-chan primitive.Timestamp, opts *PublishOpts, ordinal int) {
	var lastFlush time.Time
	var mostRecentTimestamp primitive.Timestamp
	var needFlush bool

	flush := func() {
		if needFlush {
			client.Set(context.Background(), opts.MetadataPrefix+"lastProcessedEntry."+strconv.Itoa(ordinal), encodeMongoTimestamp(mostRecentTimestamp), 0)
			lastFlush = time.Now()
			needFlush = false
		}
	}

	for {
		select {
		case timestamp, ok := <-timestamps:
			if !ok {
				// channel got closed
				return
			}

			mostRecentTimestamp = timestamp
			needFlush = true

			if time.Since(lastFlush) > opts.FlushInterval {
				flush()
			}
		case <-time.After(opts.FlushInterval):
			if needFlush {
				flush()
			}
		}
	}
}
