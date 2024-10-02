package helpers

import (
	"github.com/go-redis/redis/v8"
	"os"
	"strings"
)

func isSentinel(url string) bool {
	return strings.Contains(url, "sentinel")
}

func createOptions(url string, sentinel bool) redis.UniversalOptions {
	redisOpts, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}
	var clientOptions redis.UniversalOptions
	if sentinel {
		clientOptions = redis.UniversalOptions{
			Addrs:      []string{redisOpts.Addr},
			DB:         redisOpts.DB,
			Password:   redisOpts.Password,
			TLSConfig:  redisOpts.TLSConfig,
			MasterName: "mymaster",
		}
	} else {
		clientOptions = redis.UniversalOptions{
			Addrs:     []string{redisOpts.Addr},
			DB:        redisOpts.DB,
			Password:  redisOpts.Password,
			TLSConfig: redisOpts.TLSConfig,
		}
	}
	return clientOptions

}

func redisClient(sentinel bool) redis.UniversalClient {
	var urls = strings.Split(os.Getenv("REDIS_URL"), ",")

	for _, url := range urls {
		if isSentinel(url) == sentinel {
			clientOptions := createOptions(url, sentinel)
			return redis.NewUniversalClient(&clientOptions)
		}
	}
	panic(nil)
}

func LegacyRedisClient() redis.UniversalClient {
	return redisClient(false)
}

// RedisClient returns the second redis client to the URL specified in the REDIS_URL
// The first one is the legacy fallback URL
func RedisClient() redis.UniversalClient {
	return redisClient(true)
}
