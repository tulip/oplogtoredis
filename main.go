package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	stdlog "log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/tulip/oplogtoredis/lib/config"
	"github.com/tulip/oplogtoredis/lib/denylist"
	"github.com/tulip/oplogtoredis/lib/log"
	"github.com/tulip/oplogtoredis/lib/oplog"
	"github.com/tulip/oplogtoredis/lib/parse"
	"github.com/tulip/oplogtoredis/lib/redispub"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	defer log.Sync()

	err := config.ParseEnv()
	if err != nil {
		panic("Error parsing environment variables: " + err.Error())
	}

	writeParallelism := config.WriteParallelism()
	// each array of redis clients holds one client for each destination (regular redis, sentinel)
	// the aggregated array holds one such array for every write-parallelism shard
	aggregatedRedisClients := make([][]redis.UniversalClient, writeParallelism)
	// make one PublisherChannels for each parallel writer
	aggregatedRedisPubs := make([]oplog.PublisherChannels, writeParallelism)
	// one stopper channel corresponds to each writer, so it uses the same 2D array structure.
	stopRedisPubs := make([][]chan bool, writeParallelism)

	bufferSize := 10000
	waitGroup := sync.WaitGroup{}

	syncer, err := denylist.NewSyncer(config.PostgresPersistenceURL())
	if err != nil {
		panic("Error setting up persistent denylist: " + err.Error())
	}
	denylist, err := syncer.LoadDenylist()
	if err != nil {
		panic("Error loading persistent denylist: " + err.Error())
	}

	// this loop starts one writer shard on each pass. Repeat it a number of times equal to the write parallelism level.
	for i := 0; i < writeParallelism; i++ {
		redisClients, err := createRedisClients()
		if err != nil {
			panic(fmt.Sprintf("[%d] Error initializing Redis client: %s", i, err.Error()))
		}
		defer func(ordinal int) {
			for _, redisClient := range redisClients {
				redisCloseErr := redisClient.Close()
				if redisCloseErr != nil {
					log.Log.Errorw("Error closing Redis client",
						"error", redisCloseErr,
						"i", ordinal)
				}
			}
		}(i)
		log.Log.Infow("Initialized connection to Redis", "i", i)

		aggregatedRedisClients[i] = redisClients
		clientsSize := len(redisClients)

		// each writer shard is going to make multiple writer coroutines, one for each redis destination,
		// so we create one PublisherChannels for this shard and put each coroutine's intake channel in it.
		// these will all be aggregated in the aggregatedRedisPubs 2D array and passed to the tailer.
		redisPubsAggregationEntry := make(oplog.PublisherChannels, clientsSize)
		stopRedisPubsEntry := make([]chan bool, clientsSize)

		for j := 0; j < clientsSize; j++ {
			redisClient := redisClients[j]

			redisPubs := make(chan *redispub.Publication, bufferSize)
			redisPubsAggregationEntry[j] = redisPubs

			stopRedisPub := make(chan bool)
			stopRedisPubsEntry[j] = stopRedisPub

			waitGroup.Add(1)

			// We create two goroutines:
			//
			// The oplog.Tail goroutine reads messages from the oplog, and generates the
			// messages that we need to write to redis. It then writes them to a
			// buffered channel.
			//
			// The redispub.PublishStream goroutine reads messages from the buffered channel
			// and sends them to Redis.
			//
			// TODO PERF: Use a leaky buffer (https://github.com/tulip/oplogtoredis/issues/2)
			go func(ordinal int, clientIndex int) {
				redispub.PublishStream([]redis.UniversalClient{redisClient}, redisPubs, &redispub.PublishOpts{
					FlushInterval:    config.TimestampFlushInterval(),
					DedupeExpiration: config.RedisDedupeExpiration(),
					MetadataPrefix:   config.RedisMetadataPrefix(),
				}, stopRedisPub, ordinal)
				log.Log.Infow("Redis publisher completed", "ordinal", ordinal, "clientIndex", clientIndex)
				waitGroup.Done()
			}(i, j)
			log.Log.Info("Started up processing goroutines")

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace:   "otr",
				Name:        "buffer_available",
				Help:        "Gauge indicating the available space in the buffer of oplog entries waiting to be written to redis.",
				ConstLabels: prometheus.Labels{"ordinal": strconv.Itoa(i), "clientIndex": strconv.Itoa(j)},
			}, func() float64 {
				return float64(bufferSize - len(redisPubs))
			})

		}

		// aggregate
		aggregatedRedisPubs[i] = redisPubsAggregationEntry
		stopRedisPubs[i] = stopRedisPubsEntry
	}

	readParallelism := config.ReadParallelism()

	stopOplogTails := make([]chan bool, readParallelism)
	aggregatedMongoSessions := make([]*mongo.Client, readParallelism)
	for i := 0; i < readParallelism; i++ {
		mongoSession, err := createMongoClient()
		if err != nil {
			panic(fmt.Sprintf("[%d] Error initializing oplog tailer: %s", i, err.Error()))
		}
		defer func(i int) {
			mongoCloseCtx, cancel := context.WithTimeout(context.Background(), config.MongoConnectTimeout())
			defer cancel()

			mongoCloseErr := mongoSession.Disconnect(mongoCloseCtx)
			if mongoCloseErr != nil {
				log.Log.Errorw("Error closing Mongo client", "i", i, "error", mongoCloseErr)
			}
		}(i)
		log.Log.Infow("Initialized connection to Mongo", "i", i)
		aggregatedMongoSessions[i] = mongoSession

		stopOplogTail := make(chan bool)
		stopOplogTails[i] = stopOplogTail

		waitGroup.Add(1)
		go func(i int) {
			tailer := oplog.Tailer{
				MongoClient:  mongoSession,
				RedisClients: aggregatedRedisClients[0], // the tailer coroutine needs a redis client for determining start timestamp
				// it doesn't really matter which one since this isn't a meaningful amount of load, so just take the first one
				RedisPrefix: config.RedisMetadataPrefix(),
				MaxCatchUp:  config.MaxCatchUp(),
				Denylist:    denylist,
			}
			// pass all intake channels to the tailer, which will route messages accordingly
			tailer.Tail(aggregatedRedisPubs, stopOplogTail, i, readParallelism)

			log.Log.Info("Oplog tailer completed")
			waitGroup.Done()
		}(i)
	}

	// Start one more goroutine for the HTTP server
	httpServer := makeHTTPServer(aggregatedRedisClients, aggregatedMongoSessions, denylist, syncer)
	go func() {
		httpErr := httpServer.ListenAndServe()
		if httpErr != nil {
			panic("Could not start up HTTP server: " + httpErr.Error())
		}
	}()

	// Now we just wait until we get an exit signal, then exit cleanly
	//
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	// See examples from https://golang.org/pkg/os/signal/#Notify
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	sig := <-signalChan

	// We got a SIGINT, cleanly stop background goroutines and then return so
	// that the `defer`s above can close the Mongo and Redis connection.
	//
	// We also call signal.Reset() to clear our signal handler so if we get
	// another SIGINT we immediately exit without cleaning up.
	log.Log.Warnf("Exiting cleanly due to signal %s. Interrupt again to force unclean shutdown.", sig)
	signal.Reset()

	for _, stopOplogTail := range stopOplogTails {
		stopOplogTail <- true
	}
	for _, stopRedisPubEntry := range stopRedisPubs {
		for _, stopRedisPub := range stopRedisPubEntry {
			stopRedisPub <- true
		}
	}

	err = httpServer.Shutdown(context.Background())
	if err != nil {
		log.Log.Errorw("Error shutting down HTTP server",
			"error", err)
	}

	waitGroup.Wait()
}

// Connects to mongo
func createMongoClient() (*mongo.Client, error) {
	clientOptions := options.Client()
	clientOptions.ApplyURI(config.MongoURL())

	err := clientOptions.Validate()
	if err != nil {
		return nil, errors.Wrap(err, "parsing Mongo URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.MongoConnectTimeout())
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		return nil, errors.Wrap(err, "connecting to Mongo")
	}

	return client, nil
}

type redisLogger struct {
	log *stdlog.Logger
}

func (l redisLogger) Printf(ctx context.Context, format string, v ...interface{}) {
	l.log.Printf(format, v...)
}

// Goroutine that just reads messages and sends them to Redis. We don't do this
// inline above so that messages can queue up in the channel if we lose our
// redis connection
func createRedisClients() ([]redis.UniversalClient, error) {
	// Configure go-redis to use our logger
	stdLog, err := zap.NewStdLogAt(log.RawLog, zap.InfoLevel)
	if err != nil {
		return nil, errors.Wrap(err, "creating std logger")
	}

	redis.SetLogger(redisLogger{log: stdLog})

	// Parse the Redis URL
	var ret []redis.UniversalClient

	for _, url := range config.RedisURL() {
		clientOptions, err := parse.ParseRedisURL(url, strings.HasPrefix(url, "redis-sentinel://"))
		if err != nil {
			return nil, errors.Wrap(err, "parsing redis url")
		}
		log.Log.Info("Parsed redis url: ", clientOptions)

		if clientOptions.TLSConfig != nil {
			clientOptions.TLSConfig = &tls.Config{
				InsecureSkipVerify: false,
				MinVersion:         tls.VersionTLS12,
			}
		}
		client := redis.NewUniversalClient(clientOptions)
		_, err = client.Ping(context.Background()).Result()
		if err != nil {
			return nil, errors.Wrap(err, "pinging redis")
		}
		ret = append(ret, client)
	}

	return ret, nil
}

func makeHTTPServer(aggregatedClients [][]redis.UniversalClient, aggregatedMongos []*mongo.Client, denylistMap *sync.Map, syncer *denylist.Syncer) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		redisOK := true
		for _, clients := range aggregatedClients {
			for _, redis := range clients {
				redisErr := redis.Ping(context.Background()).Err()
				redisOK = (redisOK && (redisErr == nil))
				if !redisOK {
					log.Log.Errorw("Error connecting to Redis during healthz check",
						"error", redisErr)
				}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), config.MongoConnectTimeout())
		defer cancel()

		mongoOK := true
		for _, mongo := range aggregatedMongos {
			mongoErr := mongo.Ping(ctx, readpref.Primary())
			mongoOK = (mongoOK && (mongoErr == nil))
			if !mongoOK {
				log.Log.Errorw("Error connecting to Mongo during healthz check",
					"error", mongoErr)
			}
		}

		if mongoOK && redisOK {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{
			"mongoOK": mongoOK,
			"redisOK": redisOK,
		})
		if jsonErr != nil {
			log.Log.Errorw("Error writing healthz response",
				"error", jsonErr)
			http.Error(w, jsonErr.Error(), http.StatusInternalServerError)
		}
	})

	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.HandleFunc("/denylist", denylist.CollectionEndpoint(denylistMap, syncer))
	mux.Handle("/denylist/", http.StripPrefix("/denylist/", http.HandlerFunc(denylist.SingleEndpoint(denylistMap, syncer))))

	return &http.Server{Addr: config.HTTPServerAddr(), Handler: mux}
}
