# oplogtoredis

[![Build Status](https://travis-ci.org/tulip/oplogtoredis.svg?branch=master)](https://travis-ci.org/tulip/oplogtoredis)
[![Go Report Card](https://goreportcard.com/badge/github.com/tulip/oplogtoredis)](https://goreportcard.com/report/github.com/tulip/oplogtoredis)
[![GoDoc](https://godoc.org/github.com/tulip/oplogtoredis?status.svg)](http://godoc.org/github.com/tulip/oplogtoredis)

This program tails the oplog of a Mongo server, and publishes changes to Redis.
It's designed to work with the [redis-oplog Meteor package](https://github.com/cult-of-coders/redis-oplog).

Unlike redis-oplog's default behavior, which requires that everything that
writes to Mongo also publish matching notifications to Redis, using
oplogtoredis in combination with redis-oplog's `externalRedisPublisher` option
guarantees that every change written to Mongo will be automatically published
to Redis.

## Project Status

The project is currently stable and used in production in Tulip. Before using this project in production, review the Known Limitations below, and test it out in a staging environment.

### Known Limitations

There are a few things that don't currently work in `redis-oplog` when using the `externalRedisPublisher` option, so those features won't work when using `redis-oplog` together with `oplogtoredis`. These features are part of [`redis-oplog`'s fine-tuning options](https://github.com/cult-of-coders/redis-oplog/blob/master/docs/finetuning.md). If you don't use any of redis-oplog's fine-tuning options, you won't run into any of these limitations.

- Custom namespaces and channels ([`redis-oplog` issue #279](https://github.com/cult-of-coders/redis-oplog/issues/279))
- Synthetic mutations ([`redis-oplog` issue #277](https://github.com/cult-of-coders/redis-oplog/issues/277))

## Configuring redis-oplog

To use this with redis-oplog, configure redis-oplog with:

- `externalRedisPublisher: true`
- `globalRedisPrefix: "<name of the Mongo database>."`

For example, if your MONGO_URL for Meteor is `mongodb://mymongoserver/mydb`,
you might use this config:

```
{
    "redisOplog": {
        "redis": {
            "port": 6379,
            "host": "myredisserver"
        },
        "globalRedisPrefix": "mydb.",
        "externalRedisPublisher": true
    }
}
```

## Deploying oplogtoredis

You can build oplogtoredis from source with `go build .`, which produces a
statically-linked binary you can run. Alternatively, you can use [the public
docker image](https://hub.docker.com/r/tulip/oplogtoredis/tags/)

You must set the following environment variables:

- `OTR_MONGO_URL`: Required. Mongo URL to read the oplog from. This should
  point to the `local` database of the Mongo server and will match the
  `MONGO_OPLOG_URL` you give to your Meteor server.

- `OTR_REDIS_URL`: Required: Redis URL to publish updates to.

- `OTR_REDIS_TLS`: Optional: Defaults to `false`. Set to `true` in order connect via TLS to redis.

You may also set the following environment variables to configure the
level of logging:

- `OTR_LOG_DEBUG`: Optional. Use debug logging, which is more detailed. See
  `lib/log/main.go` for details.

- `OTR_LOG_QUIET`: Don't print any logs. Useful when running unit tests.

There are a number of other environment variables you can set to tune
various performance and reliability settings. See the
[config package docs](https://godoc.org/github.com/tulip/oplogtoredis/lib/config)
for more details.

## Running oplogtoredis in production

oplogtoredis includes a number of features to support its use in
production-critical scenarios.

### High Availability

oplogtoredis uses Redis to deduplicate messages, so it's safe (and recommended!)
to run multiple instances of oplogtoredis to ensure availability in the event
of a partial outage or a bug in oplogtoredis that causes it to crash or hang.

Just run two copies of oplogtoredis with the same configuration. Each copy
will process each oplog message and send it to Redis, where we use a small
Lua script to deduplicate the messages. It's not recommended to run more than
2 or 3 copies of oplogtoredis, because the load it puts on your Mongo and Redis
databases increases linearly with the number of copies of oplogtoredis that
you're running.

### Resumption

oplogtoredis uses Redis to keep track of the last message it processed. When
it starts up, it checks to see where it left off, and will resume tailing the
oplog from that timestamp, as long as it's not too far in the past (we don't
want to replay too much of the oplog, or we could could overload the
Redis or Meteor servers). The [config package docs](https://godoc.org/github.com/tulip/oplogtoredis/lib/config)
have more detail on how to tune this behavior, but this feature should keep
your system working propertly even if every copy of oplogtoredis that you're
running goes down for a brief period.

### Monitoring

oplogtoredis exposes an HTTP server that can be used to monitor the state of
the program. By default, it serves on `0.0.0.0:9000`, but you can change this
with the environment variable `OTR_HTTP_SERVER_ADDR`.

The HTTP server exposes a health-checking endpoint at `/healthz`. This endpoint
checks connectivity to Mongo and Redis, and then returns 200. If HTTP requests
to this endpoint time out or return non-200 codes for more than a brief
period (10-15 seconds), you should consider the program unhealthy and restart
it. You can do this using [Kubernetes liveness probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/),
an [Icinga health check](https://www.icinga.com/docs/icinga2/latest/doc/10-icinga-template-library/#http),
or any other mechanism.

The HTTP server also exposes a [Prometheus](https://prometheus.io/) endpoint
at `/metrics` that your Prometheus server can scrape to collect a number
of useful metrics. In particular, if you see the value of the metric
`otr_redispub_processed_messages` with the label `status=sent` fall to lower
than the writes to your Mongo database, it likely indicates an issue with
oplogtoredis.

### Logging

oplogtoredis by default emits info, warning, and error messages as JSON,
for easy consumption by structured logging systems. When debugging
oplogtoredis, you may want to set `OTR_LOG_DEBUG=true`, which will log
more detailed messages and log in a human-readable format for manual review.

## Development

You can use `go build` to build and test oplogtoredis, or you can use
the docker-compose environment (`docker-compose up`), which spins us a full
environment with Mongo, Redis, and oplogtoredis.

The components of the docker-compose environment are:

- `oplogtoredis`: A container running oplogtoredis, live-reloading whenever
  the source code changes (via [fresh](https://github.com/pilu/fresh)).

- `mongo`: A MongoDB server. The other containers are configured to use
  the `dev` db. Connect to Mongo with `docker-compose exec mongo mongo dev`

- `redis`: A Redis server. Connect to the CLI with
  `docker-compose exec redis redis-cli`. The `monitor` command in that CLI
  will show you everything being published.

You can optionally also spin up 2 meteor app servers with
`docker-compose -f docker-compose.yml -f docker-compose.meteor.yml`. These
servers are running a simple todos app, using redis-oplog, and pointing at the
same Mongo and Redis servers. Note that the first run will take a long time
(5-10 minutes) while Meteor does initial downloads/builds; caching makes
subsequent startups much, much faster.

The additional `docker-compose.meteor.yml` file contains:

- `meteor1` and `meteor2`: Two meteor servers, serving the app at `testapp/`.
  You can go to `localhost:9091` and `localhost:9092` to hit the two app
  servers. When everything's working correctly, changes made on a client
  connected to one app server (or made directly in the Mongo database) should
  show up in real-time to clients on the other app server.

## Testing

There are a number of testing tools and suites that are used to check the
correctness of oplogtoredis. All of these are run by Travis on each commit.

You can run all of the tests locally with `scripts/runAllTests.sh`.

### Linting and static analysis

We use `gometalinter` to detect stylistic and correctness issues. Run
`scripts/runLint.sh` to run the suite.

You'll need `gometalinter` and its dependencies installed. You can install
them with `go get github.com/alecthomas/gometalinter; gometalinter --install`.

### Unit tests

We use the standard `go test` tool. We wrap it as `scripts/runUnitTests.sh`
to set timeout and enable the race detector.

### Integration tests part 1: acceptance tests

These acceptance tests test a production-ready docker build of oplogtoredis.
They run the production docker container alongside Mongo and Redis docker
containers (using docker-compose), and then test the behavior of that whole
system.

They are run both with and without the race detector, and against a number of
different version of Mongo and Redis.

Run these tests with `scripts/runIntegrationAcceptance.sh`. This suite takes
a while to run, because it's run against many different combinations of
Redis, Mongo, and race detections. Use `scripts/runIntegrationAcceptanceSingle.sh`
for a quick run with only a single configuration.


### Integration tests part 2: fault-injection tests

These tests observe the behavior of oplogtoredis under a number of fault
conditions, such as a crash and restart of oplogtoredis, and temporary
unavailability of Mongo and Redis. To provide the test harness more control
over the environment, they operate on a compiled binary of oplogtoredis
rather than a docker image. They run inside a single docker container, with
oplogtoredis, Mongo, and Redis spun up and down by the test harness itself.

Run these tests with `scripts/runIntegrationFaultInjectionsh`.

### Integration tests part 3: meteor tests

These tests use docker-compose to spin up oplogtoredis, Mongo, Redis, and two
Meteor application servers (running the app from `./testapp`), and then connect
to the Meteor servers via websockets/DDP and observe the behavior of traffic
on the wire to ensure that oplogtoredis is working correctly in concert with
redis-oplog.

Run these tests with `scripts/runIntegrationMeteor.sh`.

### Integration tests part 4: performance tests

These tests are run in a similar environment to the acceptance tests, but
only againt a single Mongo and Redis version, and without the race detector.

We run two tests: one where we just fire-and-forget a bunch of writes to Mongo,
and one where we do the same writes, but wait until we've received notifications
about all of those writes from Redis. The difference between these two numbers
gives us an upper bound on the latency overhead of using oplogtoredis. These
tests fail if the overhead is greater than 35%.

Run these tests with `scripts/runIntegrationPerformance.sh`.
