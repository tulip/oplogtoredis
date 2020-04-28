package harness

import (
	"log"
	"os/exec"

	"github.com/go-redis/redis/v7"
)

// RedisServer represents a running Redis server
type RedisServer struct {
	Addr    string
	node    *exec.Cmd
	monitor *exec.Cmd
}

// StartRedisServer starts a Redis server and returns a RedisServer for further
// operations
func StartRedisServer() *RedisServer {
	server := RedisServer{
		Addr: "redis://localhost:6379",
	}

	server.Start()

	return &server
}

// Start starts up the Redis server. This is automatically called by
// StartRedisServer, so you should only need to call this if you've stopped
// the server.
//
// This function does not return until the server is up and ready to accept
// connections.
func (server *RedisServer) Start() {
	log.Print("Startinf up Redis server")
	server.node = exec.Command("redis-server", "--loglevel", "debug") // #nosec

	server.node.Stdout = makeLogStreamer("redis", "stdout")
	server.node.Stderr = makeLogStreamer("mongo", "stderr")

	err := server.node.Start()

	if err != nil {
		panic("Error starting up Redis server: " + err.Error())
	}

	waitTCP("localhost:6379")
	log.Print("Started up Redis server")

	log.Print("Starting up Redis monitor")
	server.monitor = exec.Command("redis-cli", "monitor") // #nosec
	server.monitor.Stdout = makeLogStreamer("redismon", "stdout")
	server.monitor.Stderr = makeLogStreamer("redismon", "stderr")
	err = server.monitor.Start()
	if err != nil {
		panic("Error starting up Redis monitor: " + err.Error())
	}
	log.Print("Started up Redis monitor")
}

// Stop kills the Redis server.
func (server *RedisServer) Stop() {
	log.Print("Shutting down Redis server")

	err := server.node.Process.Kill()
	if err != nil {
		log.Printf("Error killing redis: %s", err)
	}

	if server.monitor != nil {
		err := server.monitor.Process.Kill()
		if err != nil {
			log.Printf("Error killing redis monitor: %s", err)
		}
	}

	// Wait for them to start
	waitTCPDown("localhost:6379")

	log.Printf("Shut down Redis server")
}

// Client returns a go-redis client for this redis server
func (server *RedisServer) Client() redis.UniversalClient {
	parsedRedisURL, err := redis.ParseURL(server.Addr)
	if err != nil {
		panic("Error parsing Redis URL: " + err.Error())
	}

	return redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{parsedRedisURL.Addr},
	})
}
