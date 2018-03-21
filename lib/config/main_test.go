package config

import (
	"os"
	"strings"
	"testing"
)

var envTests = map[string]struct {
	env            map[string]string
	expectedConfig *oplogtoredisConfiguration
	expectError    bool
}{}

func TestParseEnv(t *testing.T) {
	for name, envTest := range envTests {
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
			t.Errorf(
				"[%s] Expected a error but did not get one for env: %#v.\n    Parsed config was: %#v",
				name,
				envTest.env,
				envTest.expectedConfig,
			)
		}

		if !envTest.expectError && err != nil {
			t.Errorf(
				"[%s] Recevied unexpected error while parsing env: %#v.\n    Error was: %s",
				name,
				envTest.env,
				err,
			)
		}

		// Check config expectations
		if envTest.expectedConfig != nil {
			if envTest.expectedConfig.MongoURL != MongoURL() {
				t.Errorf(
					"[%s] Incorrect Mongo URL while parsing env: %#v.\n    Expected %#v\n    Got %#v",
					name,
					envTest.env,
					envTest.expectedConfig,
					MongoURL(),
				)
			}

			if envTest.expectedConfig.RedisURL != RedisURL() {
				t.Errorf(
					"[%s] Incorrect Redis URL while parsing env: %#v.\n    Expected %#v\n    Got %#v",
					name,
					envTest.env,
					envTest.expectedConfig,
					RedisURL(),
				)
			}
		}
	}
}
