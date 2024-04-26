package redispub

import (
	"context"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
func LastProcessedTimestamp(redisClient redis.UniversalClient, metadataPrefix string, ordinal int) (primitive.Timestamp, time.Time, error) {
	str, err := redisClient.Get(context.Background(), metadataPrefix+"lastProcessedEntry."+strconv.Itoa(ordinal)).Result()
	if err != nil {
		return primitive.Timestamp{}, time.Unix(0, 0), err
	}

	ts, err := decodeMongoTimestamp(str)
	if err != nil {
		return primitive.Timestamp{}, time.Unix(0, 0), err
	}

	time := mongoTimestampToTime(ts)
	return ts, time, nil
}

// FirstLastProcessedTimestamp runs LastProcessedTimestamp for each ordinal up to the provided count,
// then returns the earliest such timestamp obtained. If any ordinal produces an error, that error is returned.
func FirstLastProcessedTimestamp(redisClient redis.UniversalClient, metadataPrefix string, maxOrdinal int) (primitive.Timestamp, time.Time, error) {
	var minTs primitive.Timestamp
	var minTime time.Time
	for i := 0; i <= maxOrdinal; i++ {
		ts, time, err := LastProcessedTimestamp(redisClient, metadataPrefix, i)
		if err != nil {
			return ts, time, err
		}

		if i == 0 || ts.Before(minTs) {
			minTs = ts
			minTime = time
		}
	}
	return minTs, minTime, nil
}
