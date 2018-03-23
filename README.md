# oplogtoredis

This program tails the oplog of a Mongo server, and publishes changes to Redis.
It's designed to work with the [redis-oplog Meteor package](https://github.com/cult-of-coders/redis-oplog).

Unlike redis-oplog's default behavior, which requires that everything that
writes to Mongo also publish matching notifications to Redis, using
oplogtoredis in combination with redis-oplog's `externalRedisPublisher` option
guarantees that every change written to Mongo will be automatically published
to Redis.

## Project Status

The project is currently pre-alpha. It shouldn't be used in production environments (or really any environment other than experimentation / testing). We expect to have an initial 0.1 release that is ready for general usage within the next couple months.

## Configuring redis-oplog

To use this with redis-oplog, configure redis-oplog with:

- `externalRedisPublisher: true`
- `redis.prefix: "<name of the Mongo database>."`

For example, if your MONGO_URL for Meteor is `mongodb://mymongoserver/mydb`,
you might use this config:

```
{
    "redisOplog": {
        "redis": {
            "port": 6379,
            "host": "myredisserver",
            "prefix": "mydb."
        },
        "externalRedisPublisher": true
    }
}
```

## Deploying oplogtoredis

You can build oplogtoredis from source with `go build .`. In addition, this repo
includes a Dockerfile you can use. There is not yet a public container on Docker
Hub, but there may be in the future.

You can set the following environment variables:

- `OTR_MONGO_URL`: Required. Mongo URL to read the oplog from. This should
  point to the `local` database of the Mongo server.

- `OTR_REDIS_URL`: Required: Redis URL to publish updates to.

- `OTR_LOG_DEBUG`: Optional. Use debug logging, which is more detailed. See
  `lib/log/main.go` for details.

- `OTR_LOG_QUIET`: Don't print any logs. Useful when running unit tests.

## Development

- Build & run using `go build` or Docker as above.
- Run linters (gofmt and go vet) with `scripts/runLint.sh`
- Run unit tests (not many yet) with `scripts/runUnitTests.sh`
- Run integration test suite & benchmarks with `scripts/runAcceptanceTests.sh`
