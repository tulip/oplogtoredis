package config

import (
	"github.com/kelseyhightower/envconfig"
)

type oplogtoredisConfiguration struct {
	RedisURL string `required:"true" split_words:"true"`
	MongoURL string `required:"true" split_words:"true"`
}

var globalConfig *oplogtoredisConfiguration

// RedisURL is a read-only accessor for the Redis URL configuration
func RedisURL() string {
	return globalConfig.RedisURL
}

// MongoURL is a read-only accessor for the Mongo URL configuration
func MongoURL() string {
	return globalConfig.MongoURL
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
