package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/tulip/oplogtoredis/lib/config"
	"github.com/tulip/oplogtoredis/lib/log"
	"github.com/tulip/oplogtoredis/lib/mongourl"
	"github.com/tulip/oplogtoredis/lib/oplog"
	"github.com/tulip/oplogtoredis/lib/redispub"

	"github.com/globalsign/mgo"
	"github.com/go-redis/redis"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	defer log.Sync()

	err := config.ParseEnv()
	if err != nil {
		panic("Error parsing environment variables: " + err.Error())
	}

	mongoSession, err := createMongoClient()
	if err != nil {
		panic("Error initialize oplog tailer: " + err.Error())
	}
	defer mongoSession.Close()
	log.Log.Info("Initialized connection to Mongo")

	redisClient, err := createRedisClient()
	if err != nil {
		panic("Error initializing Redis client: " + err.Error())
	}
	defer func() {
		redisCloseErr := redisClient.Close()
		if redisCloseErr != nil {
			log.Log.Errorw("Error closing Redis client",
				"error", redisCloseErr)
		}
	}()
	log.Log.Info("Initialized connection to Redis")

	// We crate two goroutines:
	//
	// The oplog.Tail goroutine reads messages from the oplog, and generates the
	// messages that we need to write to redis. It then writes them to a
	// buffered channel.
	//
	// The redispub.PublishStream goroutine reads messages from the buffered channel
	// and sends them to Redis.
	//
	// TODO PERF: Use a leaky buffer (https://github.com/tulip/oplogtoredis/issues/2)
	redisPubs := make(chan *redispub.Publication, 10000)
	waitGroup := sync.WaitGroup{}

	stopOplogTail := make(chan bool)
	waitGroup.Add(1)
	go func() {
		tailer := oplog.Tailer{
			MongoClient: mongoSession,
			RedisClient: redisClient,
			RedisPrefix: config.RedisMetadataPrefix(),
			MaxCatchUp:  config.MaxCatchUp(),
		}
		tailer.Tail(redisPubs, stopOplogTail)

		log.Log.Info("Oplog tailer completed")
		waitGroup.Done()
	}()

	stopRedisPub := make(chan bool)
	waitGroup.Add(1)
	go func() {
		redispub.PublishStream(redisClient, redisPubs, &redispub.PublishOpts{
			FlushInterval:    config.TimestampFlushInterval(),
			DedupeExpiration: config.RedisDedupeExpiration(),
			MetadataPrefix:   config.RedisMetadataPrefix(),
		}, stopRedisPub)

		log.Log.Info("Redis publisher completed")
		waitGroup.Done()
	}()
	log.Log.Info("Started up processing goroutines")

	// Start one more goroutine for the HTTP server
	httpServer := makeHTTPServer(redisClient, mongoSession)
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

	stopOplogTail <- true
	stopRedisPub <- true

	err = httpServer.Shutdown(context.Background())
	if err != nil {
		log.Log.Errorw("Error shutting down HTTP server",
			"error", err)
	}

	waitGroup.Wait()
}

// Connects to mongo
func createMongoClient() (*mgo.Session, error) {
	// configure mgo to use our logger
	stdLog, err := zap.NewStdLogAt(log.RawLog, zap.InfoLevel)
	if err != nil {
		return nil, errors.Wrap(err, "creating std logger")
	}

	mgo.SetLogger(stdLog)

	// get a mgo session
	dialInfo, err := mongourl.Parse(config.MongoURL())
	if err != nil {
		return nil, errors.Wrap(err, "parsing Mongo URL")
	}

	session, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to mongo")
	}

	session.SetMode(mgo.Monotonic, true)

	return session, nil
}

// Goroutine that just reads messages and sends them to Redis. We don't do this
// inline above so that messages can queue up in the channel if we lose our
// redis connection
func createRedisClient() (redis.UniversalClient, error) {
	// Configure go-redis to use our logger
	stdLog, err := zap.NewStdLogAt(log.RawLog, zap.InfoLevel)
	if err != nil {
		return nil, errors.Wrap(err, "creating std logger")
	}

	redis.SetLogger(stdLog)

	// Parse the Redis URL
	parsedRedisURL, err := redis.ParseURL(config.RedisURL())
	if err != nil {
		return nil, errors.Wrap(err, "parsing redis url")
	}

	// Create a Redis client
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    []string{parsedRedisURL.Addr},
		DB:       parsedRedisURL.DB,
		Password: parsedRedisURL.Password,
	})

	// Check that we have a connection
	_, err = client.Ping().Result()
	if err != nil {
		return nil, errors.Wrap(err, "pinging redis")
	}

	return client, nil
}

func makeHTTPServer(redis redis.UniversalClient, mongo *mgo.Session) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		redisErr := redis.Ping().Err()
		redisOK := redisErr == nil
		if !redisOK {
			log.Log.Errorw("Error connecting to Redis during healthz check",
				"error", redisErr)
		}

		mongoErr := mongo.Ping()
		mongoOK := mongoErr == nil

		if !mongoOK {
			log.Log.Errorw("Error connecting to Mongo during healthz check",
				"error", mongoErr)
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

	return &http.Server{Addr: config.HTTPServerAddr(), Handler: mux}
}
