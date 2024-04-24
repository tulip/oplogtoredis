package redispub

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func startMiniredis() (*miniredis.Miniredis, redis.UniversalClient) {
	server, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	// Run your code and see if it behaves.
	// An example using the redigo library from "github.com/gomodule/redigo/redis":
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{server.Addr()},
	})

	return server, client
}

func TestLastProcessedTimestampSuccess(t *testing.T) {
	now := time.Now()
	nowTS := primitive.Timestamp{T: uint32(now.Unix()), I: 1234}

	redisServer, redisClient := startMiniredis()
	defer redisServer.Close()

	require.NoError(t, redisServer.Set("someprefix.lastProcessedEntry.0", encodeMongoTimestamp(nowTS)))

	gotTS, gotTime, err := LastProcessedTimestamp(redisClient, "someprefix.", 0)

	if err != nil {
		t.Errorf("Got unexpected error: %s", err)
	}

	if gotTS != nowTS {
		t.Errorf("Incorrect mongo timestamp. Got %d, expected %d", gotTS, nowTS)
	}

	if gotTime.Unix() != now.Unix() {
		t.Errorf("Incorrect time. Got %d, expected %d", gotTime.Unix(), now.Unix())
	}
}

func TestLastProcessedTimestampNoRecord(t *testing.T) {
	redisServer, redisClient := startMiniredis()
	defer redisServer.Close()

	_, _, err := LastProcessedTimestamp(redisClient, "someprefix.", 0)

	if err == nil {
		t.Errorf("Expected redis.Nil error, got no error")
	} else if err != redis.Nil {
		t.Errorf("Expected redis.Nil error, got: %s", err)
	}
}

func TestLastProcessedTimestampInvalidRecord(t *testing.T) {
	redisServer, redisClient := startMiniredis()
	defer redisServer.Close()
	require.NoError(t, redisServer.Set("someprefix.lastProcessedEntry.0", "not a number"))

	_, _, err := LastProcessedTimestamp(redisClient, "someprefix.", 0)

	if err == nil {
		t.Errorf("Expected strconv error, got no error")
	} else if err.Error() != `strconv.ParseUint: parsing "not a number": invalid syntax` {
		t.Errorf("Expected strconv error, got: %s", err)
	}
}

func TestLastProcessedTimestampRedisError(t *testing.T) {
	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{"not a server"},
	})

	_, _, err := LastProcessedTimestamp(redisClient, "someprefix.", 0)

	if err == nil {
		t.Errorf("Expected TCP error, got no error")
	} else if err.Error() != "dial tcp: address not a server: missing port in address" {
		t.Errorf("Expected TCP error, got: %s", err)
	}
}
