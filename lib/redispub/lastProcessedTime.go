package redispub

import (
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/go-redis/redis/v7"
)

// LastProcessedTimestamp returns the timestamp of the last oplog entry that
// oplogtoredis processed.
//
// It returns both the bson.MongoTimestamp, as well as the time.Time value that
// timestamp represents (accurate to within 1 second; mongo timestamps only
// store second resolution)
//
// If oplogtoredis has not processed any messages, returns redis.Nil as an
// error.
func LastProcessedTimestamp(redisClient redis.UniversalClient, metadataPrefix string) (bson.MongoTimestamp, time.Time, error) {
	str, err := redisClient.Get(metadataPrefix + "lastProcessedEntry").Result()
	if err != nil {
		return 0, time.Unix(0, 0), err
	}

	ts, err := decodeMongoTimestamp(str)
	if err != nil {
		return 0, time.Unix(0, 0), err
	}

	time := mongoTimestampToTime(ts)
	return ts, time, nil
}
