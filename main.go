package main

import (
	"fmt"
	"time"

	"github.com/tulip/oplogtoredis/lib/config"
	"github.com/tulip/oplogtoredis/lib/log"
	"github.com/tulip/oplogtoredis/lib/oplog"
	"github.com/tulip/oplogtoredis/lib/redispub"
	"go.uber.org/zap"

	"github.com/globalsign/mgo"
	"github.com/go-redis/redis"
	"github.com/rwynn/gtm"
)

func main() {
	defer log.RawLog.Sync()

	err := config.ParseEnv()
	if err != nil {
		panic("Error parsing environment variables: " + err.Error())
	}

	mongoSession, gtmSession, err := createGTMClient()
	if err != nil {
		panic("Error initialize oplog tailer: " + err.Error())
	}
	defer mongoSession.Close()
	defer gtmSession.Stop()

	redisClient, err := createRedisClient()
	if err != nil {
		panic("Error initializing Redis client: " + err.Error())
	}
	defer redisClient.Close()

	// We crate two goroutines:
	//
	// The readOplog goroutine reads messages from the oplog, and generates the
	// messages that we need to write to redis. It then writes them to a
	// buffered channel.
	//
	// The writeMessages goroutine reads messages from the buffered channel
	// and sends them to Redis.
	//
	// TODO PERF: Use a leaky buffer (https://github.com/tulip/oplogtoredis/issues/2)
	redisPubs := make(chan *redispub.Publication, 10000)

	go oplog.Tail(gtmSession.OpC, redisPubs)

	// This blocks forever; if we end up needing to do more work in the main
	// goroutine we'll have to move this to a background goroutine
	redispub.PublishStream(redisClient, redisPubs, &redispub.PublishOpts{
		FlushInterval:    config.TimestampFlushInterval(),
		DedupeExpiration: config.RedisDedupeExpiration(),
		MetadataPrefix:   config.RedisMetadataPrefix(),
	})
}

// Connects to mongo, starts up a gtm client, and starts up a background
// goroutine to log GTM errors
func createGTMClient() (*mgo.Session, *gtm.OpCtx, error) {
	// configure mgo to use our logger
	stdLog, err := zap.NewStdLogAt(log.RawLog, zap.InfoLevel)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not create a std logger: %s", err)
	}

	mgo.SetLogger(stdLog)

	// get a mgo session
	session, err := mgo.Dial(config.MongoURL())
	if err != nil {
		return nil, nil, err
	}

	session.SetMode(mgo.Monotonic, true)

	// Use gtm to tail to oplog
	//
	// TODO PERF: benchmark other oplog tailers (https://github.com/tulip/oplogtoredis/issues/3)
	//
	// TODO: pick up where we left off on restart (https://github.com/tulip/oplogtoredis/issues/4)
	ctx := gtm.Start(session, &gtm.Options{
		ChannelSize:       10000,
		BufferDuration:    100 * time.Millisecond,
		UpdateDataAsDelta: true,
		WorkerCount:       8,
	})

	// Start a goroutine to log gtm errors
	go func() {
		for {
			err := <-ctx.ErrC

			log.Log.Errorw("Error tailing oplog",
				"error", err)
		}
	}()

	return session, ctx, nil
}

// Goroutine that just reads messages and sends them to Redis. We don't do this
// inline above so that messages can queue up in the channel if we lose our
// redis connection
func createRedisClient() (redis.UniversalClient, error) {
	// Configure go-redis to use our logger
	stdLog, err := zap.NewStdLogAt(log.RawLog, zap.InfoLevel)
	if err != nil {
		return nil, fmt.Errorf("Could not create a std logger: %s", err)
	}

	redis.SetLogger(stdLog)

	// Parse the Redis URL
	parsedRedisURL, err := redis.ParseURL(config.RedisURL())
	if err != nil {
		return nil, fmt.Errorf("Error parsing Redis URL: %s", err)
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
		return nil, fmt.Errorf("Redis ping failed: %s", err)
	}

	return client, nil
}
