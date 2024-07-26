// Package redispub reads messages from an input channel and publishes them to
// redis. It handles deduplicating messages (across multiple running copies of
// oplogtoredis), and tracking the timestamp of the last message we successfully
// publishes (so we can pick up from where we left off if oplogtoredis restarts).
package redispub

import (
	"context"
	"fmt"
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

// This script checks whether KEYS[1] is set. If it is, it does nothing. It not,
// it sets the key, using ARGV[1] as the expiration, and then publishes the
// message ARGV[2] to channels ARGV[3] and ARGV[4].
var publishDedupe = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == false then
		redis.call("SETEX", KEYS[1], ARGV[1], 1)
		for w in string.gmatch(ARGV[3], "([^$]+)") do
			redis.call("PUBLISH", w, ARGV[2])
		end
	end

	return true
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
}, []string{"ordinal"})

var metricOplogEntryStaleness = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "entry_staleness_seconds",
	Help:      "Histogram recording the difference between this server's clock and the timestamp of each processed oplog entry.",
	Buckets:   []float64{0.1, 0.2, 0.5, 1, 2, 5, 10, 20, 50, 100},
}, []string{"ordinal"})

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

	type PubFn func(*Publication) error

	var publishFns []PubFn

	for _, client := range clients {
		client := client
		publishFn := func(p *Publication) error {
			return publishSingleMessage(p, client, opts.MetadataPrefix, dedupeExpirationSeconds, ordinal)
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

	for {
		select {
		case <-stop:
			close(timestampC)
			return

		case p := <-in:
			metricStalenessPreRetries.WithLabelValues(strconv.Itoa(ordinal)).Set(float64(time.Since(time.Unix(int64(p.OplogTimestamp.T), 0)).Seconds()))
			for i, publishFn := range publishFns {
				err := publishSingleMessageWithRetries(p, 30, time.Second, publishFn)
				log.Log.Debugw("Published to", "idx", i)

				if err != nil {
					metricsSendFailed[i].Inc()
					log.Log.Errorw("Permanent error while trying to publish message; giving up",
						"error", err,
						"message", p)
				} else {
					metricsSendSuccess[i].Inc()

					// We want to make sure we do this *after* we've successfully published
					// the messages
					timestampC <- p.OplogTimestamp
				}
			}
		}
	}
}

func publishSingleMessageWithRetries(p *Publication, maxRetries int, sleepTime time.Duration, publishFn func(p *Publication) error) error {
	if p == nil {
		return errors.New("Nil Redis publication")
	}

	retries := 0
	for retries < maxRetries {
		err := publishFn(p)

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

func publishSingleMessage(p *Publication, client redis.UniversalClient, prefix string, dedupeExpirationSeconds int, ordinal int) error {
	start := time.Now()
	ordinalStr := strconv.Itoa(ordinal)
	staleness := float64(time.Since(time.Unix(int64(p.OplogTimestamp.T), 0)).Seconds())
	metricLastOplogEntryStaleness.WithLabelValues(ordinalStr).Set(staleness)
	metricOplogEntryStaleness.WithLabelValues(ordinalStr).Observe(staleness)

	_, err := publishDedupe.Run(
		context.Background(),
		client,
		[]string{
			// The key used for deduplication
			// The oplog timestamp isn't really a timestamp -- it's a 64-bit int
			// where the first 32 bits are a unix timestamp (seconds since
			// the epoch), and the next 32 bits are a monotonically-increasing
			// sequence number for operations within that second. It's
			// guaranteed-unique, so we can use it for deduplication.
			// However, timestamps are shared within transactions, so we need more information to ensure uniqueness.
			// The TxIdx field is used to ensure that each entry in a transaction has its own unique key.
			formatKey(p, prefix),
		},
		dedupeExpirationSeconds,       // ARGV[1], expiration time
		p.Msg,                         // ARGV[2], message
		strings.Join(p.Channels, "$"), // ARGV[3], channels
	).Result()

	redisCommandDuration.WithLabelValues(ordinalStr).Observe(time.Since(start).Seconds())
	return err
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
