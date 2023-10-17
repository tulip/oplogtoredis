package helpers

import (
	"os"

	"github.com/go-redis/redis/v8"
)

// RedisClient returns a redis client to the URL specified in the REDIS_URL
// env var
func RedisClient() *redis.Client {
	redisOpts, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(err)
	}

	return redis.NewClient(redisOpts)
}
