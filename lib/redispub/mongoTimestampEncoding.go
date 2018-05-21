package redispub

import (
	"strconv"
	"time"

	"github.com/globalsign/mgo/bson"
)

// Converts a bson.MongoTimestamp into a string (in base-10)
func encodeMongoTimestamp(ts bson.MongoTimestamp) string {
	return strconv.FormatInt(int64(ts), 10)
}

// Converts a string (in base-10) into a bson.MongoTimestamp
func decodeMongoTimestamp(ts string) (bson.MongoTimestamp, error) {
	i, err := strconv.ParseInt(ts, 10, 64)

	if err != nil {
		return bson.MongoTimestamp(0), err
	}

	return bson.MongoTimestamp(i), nil
}

// Returns a time.Time from a bson.MongoTimestamp
//
// Mongo timestamps are 64-bit integers where the first 32 bits are seconds
// since the unix epoch, and the last 32 bits are a unique, monotonically-
// increasing value within that second (to guarantee uniqueness), so we read
// only the first 32 bits to convert to a real time.
func mongoTimestampToTime(ts bson.MongoTimestamp) time.Time {
	unixTS := int64(ts) >> 32
	return time.Unix(unixTS, 0)
}
