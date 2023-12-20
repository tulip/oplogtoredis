package parse

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

// parseRedisURL converts an url string, that may be a redis connection string,
// or may be a sentinel protocol pseudo-url, into a set of redis connection options.
func ParseRedisURL(url string, isSentinel bool) (*redis.UniversalOptions, error) {
	if isSentinel {
		opts, err := parseSentinelURL(url)
		return opts, err
	}

	parsedRedisURL, err := redis.ParseURL(url)
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing Redis URL")
	}

	// non-sentinel redis does not use MasterName, so leave it as ""
	return &redis.UniversalOptions{
		Addrs:     []string{parsedRedisURL.Addr},
		DB:        parsedRedisURL.DB,
		Password:  parsedRedisURL.Password,
		TLSConfig: parsedRedisURL.TLSConfig,
	}, nil

}

// match against redis-sentinel://[something@]something[/db]
var urlMatcher *regexp.Regexp = regexp.MustCompile(`redis-sentinel:\/\/(([^@]+)@)?([^/]+)(\/(\d+))?`)

// match against host:port
var endpointMatcher *regexp.Regexp = regexp.MustCompile(`([^:]+):(\d+)`)

// parseSentinelURL converts a sentinel protocol pseudo-url into a set of redis connection numbers.
// we expect sentinel urls to be of the form redis-sentinel://[password@]host:port[,host2:port2][,hostN:portN][/db][?sentinelMasterId=name]
// because of the redis-sentinel:// protocol, the protocol cannot be rediss://
// and therefore there cannot be any tls config for the options returned from this function
func parseSentinelURL(urlString string) (*redis.UniversalOptions, error) {
	// the comma-separated list of host:port pairs means this is not a true url, and so must be parsed manually

	// parse query params
	queryIdx := strings.Index(urlString, "?")
	base := urlString
	query := ""
	if queryIdx >= 0 {
		base = urlString[0:queryIdx]
		query = urlString[queryIdx+1:]
	}
	queryParams, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}
	sentinelMasterId, ok := queryParams["sentinelMasterId"]
	var name string = ""
	if ok {
		name = sentinelMasterId[0]
	}

	// parse base url
	match := urlMatcher.FindStringSubmatch(base)
	if match == nil || match[0] != base {
		return nil, errors.New("Redis Sentinel URL did not conform to schema")
	}
	password := match[2]
	endpoints := match[3]
	dbStr := match[5]
	// db is optional
	db := 0
	if dbStr != "" {
		db, err = strconv.Atoi(dbStr)
		if err != nil {
			return nil, errors.Wrap(err, "Redis Sentinel URL DB is NaN")
		}
	}

	// check endpoints parse
	endpointsList := strings.Split(endpoints, ",")
	for _, endpoint := range endpointsList {
		if endpointMatcher.FindString(endpoint) != endpoint {
			return nil, errors.New("Redis Sentinel URL Endpoints List did not conform to schema")
		}
	}

	return &redis.UniversalOptions{
		Password:   password,
		Addrs:      endpointsList,
		MasterName: name,
		DB:         db,
	}, nil
}
