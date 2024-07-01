// Package config reads oplogtoredis configuration values from environment
// variables. The documentation for this package also documents all of the
// configuration options that are available to configure oplogtoredis.
package config

import (
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type oplogtoredisConfiguration struct {
	RedisURL                      string        `required:"true" split_words:"true"`
	MongoURL                      string        `required:"true" split_words:"true"`
	HTTPServerAddr                string        `default:"0.0.0.0:9000" envconfig:"HTTP_SERVER_ADDR"`
	BufferSize                    int           `default:"10000" split_words:"true"`
	TimestampFlushInterval        time.Duration `default:"1s" split_words:"true"`
	MaxCatchUp                    time.Duration `default:"60s" split_words:"true"`
	RedisDedupeExpiration         time.Duration `default:"120s" split_words:"true"`
	RedisMetadataPrefix           string        `default:"oplogtoredis::" split_words:"true"`
	MongoConnectTimeout           time.Duration `default:"10s" split_words:"true"`
	MongoQueryTimeout             time.Duration `default:"5s" split_words:"true"`
	OplogV2ExtractSubfieldChanges bool          `default:"false" envconfig:"OPLOG_V2_EXTRACT_SUBFIELD_CHANGES"`
	WriteParallelism              int           `default:"1" split_words:"true"`
	ReadParallelism               int           `default:"1" split_words:"true"`
	PostgresPersistenceURL        string        `default:"" envconfig:"PG_PERSISTENCE_URL"`
	SentryEnabled                 bool          `default:"false" split_words:"true"`
	SentryDSN                     string        `default:"" envconfig:"SENTRY_DSN"`
	SentryEnvironment             string        `default:"unknown" split_words:"true"`
	SentryRelease                 string        `default:"unknown" split_words:"true"`
}

var globalConfig *oplogtoredisConfiguration

// RedisURL is the configuration for connecting to a Redis instance using the 'OTR_REDIS_URL' environment variable.
// For TLS, use 'rediss://'; for non-TLS, use 'redis://'.
// Multiple URLs can be configured by separating them with commas.
func RedisURL() []string {
	return strings.Split(globalConfig.RedisURL, ",")
}

// MongoURL is the Mongo URL configuration. Is is required, and is set via the
// environment variable `OTR_MONGO_URL`.
func MongoURL() string {
	return globalConfig.MongoURL
}

// HTTPServerAddr the address we bind our HTTP server to. The HTTP server
// exposes a health-checking endpoint on `/healthz` and Prometheus metrics on
// `/metrics`. It is set via the environment variable `OTR_HTTP_SERVER_ADDR` and
// defaults to `0.0.0.0:9000`
func HTTPServerAddr() string {
	return globalConfig.HTTPServerAddr
}

// BufferSize is the size of the internal buffers that hold oplog messages while
// they're being processed. It is set via the environment variable
// `OTR_BUFFER_SIZE` and defaults to 10,000.
func BufferSize() int {
	return globalConfig.BufferSize
}

// TimestampFlushInterval is how frequently to flush the timestamp of the last
// processed message to Redis. When we start up, we start tailing the oplog from
// where we left off (as indicated by this timestamp). It is set via the
// environment variable `OTR_TIMESTAMP_FLUSH_INTERVAL and defaults to
// 1s.
func TimestampFlushInterval() time.Duration {
	return globalConfig.TimestampFlushInterval
}

// MaxCatchUp is the maximum length of time for which we process old oplog
// entries. When starting up, if the timestamp of the last entry processes is
// more than MaxCatchUp ago, we don't try to catch up and just start processing
// the oplog from the end. If it's less than MaxCatchUp, we process oplog
// entries starting from the timestamp. This allows us to catch up if
// oplogtoredis exists and then starts back up. It is set via the environment
// variable `OTR_MAX_CATCH_UP` and defaults to 60s.
func MaxCatchUp() time.Duration {
	return globalConfig.MaxCatchUp
}

// RedisDedupeExpiration controls the expiration of the Redis keys that are used
// to ensure we process oplog entries at most once. Every time we publish an
// oplog entry to Redis, we write its unique timestamp as a Redis expiring key,
// and check for the existence of that key before doing the actual publish. This
// allows us to both run multiple copies of oplogtoredis (only one will get to
// write the key and send the message, the other one will see the key exists and
// skip publishing), and also ensures that on restart we don't re-publish
// entries from in between the last time the latest-processed-timestamp was
// updated in Redis and whne the process existed. It is set via the environment
// variable `OTR_REDIS_DEDUPE_EXPIRATION` and defaults to 120s.
func RedisDedupeExpiration() time.Duration {
	return globalConfig.RedisDedupeExpiration
}

// RedisMetadataPrefix controls the prefix for keys used to store oplogtoredis
// metadata (such as the timestamp of the last oplog entry processed). If you're
// running multiple instances of oplogtoredis for the same MongoDB (for high
// availability), you should use the same RedisMetadataPrefix for both. If
// you're running multiple instances for different MongoDBs (because you're
// using many MongoDB instances with a shared Redis instace, for example), you
// should have different RedisMetadataPrefixes for each.
//
// This *does not* affect the channel names used to publish oplog entries. The
// channel names are always `<db-name>.<collection-name>“ and
// `<db-name>.<collection-name>::<document-id>`.`
//
// It is set via the environment variable `OTR_REDIS_METADATA_PREFIX` and
// defaults to "oplogtoredis::".
func RedisMetadataPrefix() string {
	return globalConfig.RedisMetadataPrefix
}

// MongoConnectTimeout controls how long we'll spend connecting to Mongo before
// timing out at startup.
func MongoConnectTimeout() time.Duration {
	return globalConfig.MongoConnectTimeout
}

// MongoQueryTimeout controls how long we'll spend waiting for the result of
// a query before timing out. This includes how long we'll wait for an oplog
// entry before timing out and re-issuing the oplog query if there is no
// oplog activity, so if you set this to a short duration on a rarely-active
// cluster, you'll see a lot of (harmless) timeouts.
func MongoQueryTimeout() time.Duration {
	return globalConfig.MongoQueryTimeout
}

// OplogV2ExtractSubfieldChanges controls whether we perform an in-depth
// analysis of v2 oplog entries (from Mongo 5.x+) to extract not just
// which top-levels fields of documents have changed, but also which sub-fields
// have changed. This provides the closest compatibility with pre-5.x behavior,
// and enables some optimizations in redis-oplog, but at the expense of being
// heavily dependent on the (undocumented) Mongo oplog format that's subject
// to change without notice.
func OplogV2ExtractSubfieldChanges() bool {
	return globalConfig.OplogV2ExtractSubfieldChanges
}

// WriteParallelism controls how many parallel write loops will be run (sharded based on a hash
// of the database name.) Each parallel loop has its own redis connection and internal buffer.
// Healthz endpoint will report fail if anyone of them dies.
func WriteParallelism() int {
	return globalConfig.WriteParallelism
}

// ReadParallelism controls how many parallel read loops will be run. Each read loop has its own mongo
// connection. Each loop consumes the entire oplog, but will hash the database name and discard any
// messages that don't match the loop's ordinal (effectively the same shard algorithm the write loops use).
// Healthz endpoint will report fail if anyone of them dies.
func ReadParallelism() int {
	return globalConfig.ReadParallelism
}

// PostgresPersistenceURL is the optional configuration for persisting a denylist entry to a postgres database
// If configured, the denylist will be written to the DB on every change, and loaded on startup
func PostgresPersistenceURL() string {
	return globalConfig.PostgresPersistenceURL
}

// SentryEnabled is the optional configuration to enable sentry for logging
// If configured, sentry will be initialized on startup.
func SentryEnabled() bool {
	return globalConfig.SentryEnabled
}

// SentryDSN is the DSN for initializing Sentry
// Required if SentryEnabled is set
func SentryDSN() string {
	return globalConfig.SentryDSN
}

// SentryEnvironment is the environment parameter for sentry (e.g., host)
// Required if SentryEnabled is set
func SentryEnvironment() string {
	return globalConfig.SentryEnvironment
}

// SentryRelease is the release parameter for sentry (e.g., version or commit)
// Required if SentryEnabled is set
func SentryRelease() string {
	return globalConfig.SentryRelease
}

// ParseEnv parses the current environment variables and updates the stored
// configuration. It is *not* threadsafe, and should just be called once
// at the start of the program.
func ParseEnv() error {
	var config oplogtoredisConfiguration

	err := envconfig.Process("otr", &config)
	if err != nil {
		return err
	}

	globalConfig = &config
	return nil
}
