package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

var envTests = map[string]struct {
	env            map[string]string
	expectedConfig *oplogtoredisConfiguration
	expectError    bool
}{
	"Full env": {
		env: map[string]string{
			"OTR_REDIS_URL":                "redis://something",
			"OTR_MONGO_URL":                "mongodb://something",
			"OTR_HTTP_SERVER_ADDR":         "localhost:1234",
			"OTR_BUFFER_SIZE":              "10",
			"OTR_TIMESTAMP_FLUSH_INTERVAL": "10m",
			"OTR_MAX_CATCH_UP":             "0",
			"OTR_REDIS_DEDUPE_EXPIRATION":  "12s",
			"OTR_REDIS_METADATA_PREFIX":    "someprefix.",
		},
		expectedConfig: &oplogtoredisConfiguration{
			RedisURL:               "redis://something",
			MongoURL:               "mongodb://something",
			HTTPServerAddr:         "localhost:1234",
			BufferSize:             10,
			TimestampFlushInterval: 10 * time.Minute,
			MaxCatchUp:             0,
			RedisDedupeExpiration:  12 * time.Second,
			RedisMetadataPrefix:    "someprefix.",
		},
	},
	"Minimal env": {
		env: map[string]string{
			"OTR_REDIS_URL": "redis://yyy",
			"OTR_MONGO_URL": "mongodb://xxx",
		},
		expectedConfig: &oplogtoredisConfiguration{
			RedisURL:               "redis://yyy",
			MongoURL:               "mongodb://xxx",
			HTTPServerAddr:         "0.0.0.0:9000",
			BufferSize:             10000,
			TimestampFlushInterval: time.Second,
			MaxCatchUp:             2 * time.Minute,
			RedisDedupeExpiration:  2 * time.Minute + 30 * time.Second,
			RedisMetadataPrefix:    "oplogtoredis::",
		},
	},
	"Missing redis URL": {
		env: map[string]string{
			"OTR_MONGO_URL": "mongodb://xxx",
		},
		expectError: true,
	},
	"Missing mongo URL": {
		env: map[string]string{
			"OTR_REDIS_URL": "redis://yyy",
		},
		expectError: true,
	},
}

// nolint: gocyclo
func TestParseEnv(t *testing.T) {
	for name, envTest := range envTests {
		t.Run(name, func(t *testing.T) {
			// clear env
			for _, envPair := range os.Environ() {
				if strings.HasPrefix(envPair, "OTR_") {
					// envPair is of the format "KEY=VALUE" so we split on "="
					os.Unsetenv(strings.SplitN(envPair, "=", 2)[0])
				}
			}

			// Set up env
			for k, v := range envTest.env {
				os.Setenv(k, v)
			}

			// Run parseEnv
			err := ParseEnv()

			// Check error expectations
			if envTest.expectError && err == nil {
				t.Fatalf(
					"Expected a error but did not get one for env: %#v.\n    Parsed config was: %#v",
					envTest.env, envTest.expectedConfig,
				)
			}

			if !envTest.expectError && err != nil {
				t.Fatalf(
					"Received unexpected error while parsing env: %#v.\n    Error was: %s",
					envTest.env, err,
				)
			}

			// Check config expectations
			if envTest.expectedConfig != nil {
				checkConfigExpectation(t, envTest.expectedConfig)
			}
		})
	}
}

func checkConfigExpectation(t *testing.T, expectedConfig *oplogtoredisConfiguration) {
	if expectedConfig.MongoURL != MongoURL() {
		t.Errorf("Incorrect Mongo URL. Expected \"%s\", Got \"%s\"",
			expectedConfig.MongoURL, MongoURL())
	}

	if expectedConfig.RedisURL != strings.Join(RedisURL()[:], "") {
		t.Errorf("Incorrect Redis URL. Expected \"%s\", Got \"%s\"",
			expectedConfig.RedisURL, RedisURL())
	}

	if expectedConfig.HTTPServerAddr != HTTPServerAddr() {
		t.Errorf("Incorrect HTTPServerAddr. Expected \"%s\", Got \"%s\"",
			expectedConfig.HTTPServerAddr, HTTPServerAddr())
	}

	if expectedConfig.BufferSize != BufferSize() {
		t.Errorf("Incorrect BufferSize. Expected %d, Got %d",
			expectedConfig.BufferSize, BufferSize())
	}

	if expectedConfig.TimestampFlushInterval != TimestampFlushInterval() {
		t.Errorf("Incorrect TimestampFlushInterval. Expected %d, Got %d",
			expectedConfig.TimestampFlushInterval, TimestampFlushInterval())
	}

	if expectedConfig.MaxCatchUp != MaxCatchUp() {
		t.Errorf("Incorrect MaxCatchUp. Expected %d, Got %d",
			expectedConfig.MaxCatchUp, MaxCatchUp())
	}

	if expectedConfig.RedisDedupeExpiration != RedisDedupeExpiration() {
		t.Errorf("Incorrect RedisDedupeExpiration. Expected %d, Got %d",
			expectedConfig.RedisDedupeExpiration, RedisDedupeExpiration())
	}

	if expectedConfig.RedisMetadataPrefix != RedisMetadataPrefix() {
		t.Errorf("Incorrect RedisMetadataPrefix. Expected \"%s\", Got \"%s\"",
			expectedConfig.RedisMetadataPrefix, RedisMetadataPrefix())
	}
}
