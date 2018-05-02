package redispub

import "github.com/go-redis/redis"

// PublishStream reads Publications from the given channel and publishes them
// to Redis.
func PublishStream(client redis.UniversalClient, in <-chan *Publication) {
	for {
		p := <-in
		client.Publish(p.Channel, p.Msg)
	}
}
