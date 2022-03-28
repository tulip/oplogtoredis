package redispub

import (
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Converts a primitive.Timestamp into a string (in base-10). For backwards
// compatiblity, we use the same encoding that's using in the MongoDB wire
// protocol (encoding the 2 uint32 components of the timestamp as a single
// uint64).
func encodeMongoTimestamp(ts primitive.Timestamp) string {
	return strconv.FormatUint(uint64(ts.T)<<32|uint64(ts.I), 10)
}

// Converts a string (in base-10) into a primitive.Timestamp
func decodeMongoTimestamp(ts string) (primitive.Timestamp, error) {
	i, err := strconv.ParseUint(ts, 10, 64)

	if err != nil {
		return primitive.Timestamp{}, err
	}

	return primitive.Timestamp{T: uint32(i >> 32), I: uint32(i)}, nil
}

// Returns a time.Time from a primitive.Timestamp
func mongoTimestampToTime(ts primitive.Timestamp) time.Time {
	return time.Unix(int64(ts.T), 0)
}
