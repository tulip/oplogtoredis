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
	Help:      "Messages processed by Redis publisher, partitioned by whether or not we successfully sent them",
}, []string{"status", "clientIdx"})

var metricTemporaryFailures = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "otr",
	Subsystem: "redispub",
	Name:      "temporary_send_failures",
	Help:      "Number of failures encountered when trying to send a message. We automatically retry, and only register a permanent failure (in otr_redispub_processed_messages) after 30 failures.",
}, []string{"clientIdx"})

// PublishStream reads Publications from the given channel and publishes them
// to Redis.
func PublishStream(clients []redis.UniversalClient, in <-chan *Publication, opts *PublishOpts, stop <-chan bool) {
	// Start up a background goroutine for periodically updating the last-processed
	// timestamp
	timestampC := make(chan primitive.Timestamp)
	for _,client := range clients {
		go periodicallyUpdateTimestamp(client, timestampC, opts)
	}

	// Redis expiration is in integer seconds, so we have to convert the
	// time.Duration
	dedupeExpirationSeconds := int(opts.DedupeExpiration.Seconds())

	var inChans []chan *Publication
	var outChans []chan error

	defer func () {
		for _, c := range(inChans) {
			close(c)
		}
	}()

	for i, client := range clients {
		clientIdx := i
		client := client
		inChan := make(chan *Publication)
		inChans = append(inChans, inChan)
		outChan := make(chan error)
		outChans = append(outChans, outChan)

		go func() {
			defer close(outChan)

			publishFn := func(p *Publication) error {
				return publishSingleMessage(p, client, opts.MetadataPrefix, dedupeExpirationSeconds)
			}

			for p := range inChan {
				log.Log.Debugw("Attempting to publish to", "clientIdx", clientIdx)
				outChan <- publishSingleMessageWithRetries(p, 30, clientIdx, time.Second, publishFn)
			}
		}()
	}

	for {
		select {
		case <-stop:
			close(timestampC)
			return

		case p := <-in:
			for _, inChan := range inChans {
				inChan <- p
			}

			for clientIdx, outChan := range outChans {
				err := <-outChan
				clientIdxStr := strconv.FormatInt(int64(clientIdx), 10)

				if err != nil {
					metricSentMessages.WithLabelValues("failed", clientIdxStr).Inc()
					log.Log.Errorw(
						"Permanent error while trying to publish message; giving up",
						"clientIdx", clientIdx,
						"error", err,
						"message", p,

					)
				} else {
					metricSentMessages.WithLabelValues("sent", clientIdxStr).Inc()
				}
			}

			// We want to make sure we do this *after* we've successfully published
			// the messages
			timestampC <- p.OplogTimestamp
		}
	}
}

func publishSingleMessageWithRetries(p *Publication, maxRetries int, clientIdx int, sleepTime time.Duration, publishFn func(p *Publication) error) error {
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
			metricTemporaryFailures.WithLabelValues(strconv.FormatInt(int64(clientIdx), 10)).Inc()
			retries++
			time.Sleep(sleepTime)
		} else {
			// success, return
			return nil
		}
	}

	return errors.Errorf("sending message (retried %v times)", maxRetries)
}

func publishSingleMessage(p *Publication, client redis.UniversalClient, prefix string, dedupeExpirationSeconds int) error {
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
func periodicallyUpdateTimestamp(client redis.UniversalClient, timestamps <-chan primitive.Timestamp, opts *PublishOpts) {
	var lastFlush time.Time
	var mostRecentTimestamp primitive.Timestamp
	var needFlush bool

	flush := func() {
		if needFlush {
			client.Set(context.Background(), opts.MetadataPrefix+"lastProcessedEntry", encodeMongoTimestamp(mostRecentTimestamp), 0)
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
