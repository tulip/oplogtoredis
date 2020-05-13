package mongourl

import (
	"errors"
	"reflect"
	"testing"

	"github.com/globalsign/mgo"
)

func TestParse(t *testing.T) {
	tests := map[string]struct {
		URL               string
		expectedParsedURL *mgo.DialInfo
		expectedError     error
	}{
		"Just a server": {
			URL: "mongodb://foo.x.y.z",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:   []string{"foo.x.y.z"},
				Timeout: DefaultTimeout,
			},
		},
		"Replica set": {
			URL: "mongodb://foo.x.y.z,bar.x.y.z?replicaSet=somers&connect=replicaSet",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:          []string{"foo.x.y.z", "bar.x.y.z"},
				Timeout:        DefaultTimeout,
				ReplicaSetName: "somers",
			},
		},
		"Database name": {
			URL: "mongodb://foo.x.y.z/somedb",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:    []string{"foo.x.y.z"},
				Timeout:  DefaultTimeout,
				Database: "somedb",
			},
		},
		"Auth": {
			URL: "mongodb://user:pass@foo.x.y.z",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:    []string{"foo.x.y.z"},
				Timeout:  DefaultTimeout,
				Username: "user",
				Password: "pass",
			},
		},
		"authSource": {
			URL: "mongodb://foo.x.y.z/somedb?authSource=otherdb",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:    []string{"foo.x.y.z"},
				Timeout:  DefaultTimeout,
				Database: "somedb",
				Source:   "otherdb",
			},
		},
		"GSSAPI": {
			URL: "mongodb://foo.x.y.z?authMechanism=gssapi&gssapiServiceName=someService",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:     []string{"foo.x.y.z"},
				Timeout:   DefaultTimeout,
				Mechanism: "gssapi",
				Service:   "someService",
			},
		},
		"maxPoolSize": {
			URL: "mongodb://foo.x.y.z?maxPoolSize=10",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:     []string{"foo.x.y.z"},
				Timeout:   DefaultTimeout,
				PoolLimit: 10,
			},
		},
		"direct": {
			URL: "mongodb://foo.x.y.z?connect=direct",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:   []string{"foo.x.y.z"},
				Timeout: DefaultTimeout,
				Direct:  true,
			},
		},
		"ssl false": {
			URL: "mongodb://foo.x.y.z?ssl=false",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:   []string{"foo.x.y.z"},
				Timeout: DefaultTimeout,
			},
		},
		"ssl 0": {
			URL: "mongodb://foo.x.y.z?ssl=0",
			expectedParsedURL: &mgo.DialInfo{
				Addrs:   []string{"foo.x.y.z"},
				Timeout: DefaultTimeout,
			},
		},
		"malformed URL": {
			// Go's URL parser is pretty permissive -- bad %-escaped values are
			// one of the few ways to get it to error.
			URL:           "mongodb://foo/bar%",
			expectedError: errors.New("parse mongodb://foo/bar%: invalid URL escape \"%\""),
		},
		"bad ?maxPoolSize": {
			URL:           "mongodb://foo?maxPoolSize=notANumber",
			expectedError: errors.New(`bad value for maxPoolSize: "notANumber"`),
		},
		"bad ?ssl": {
			URL:           "mongodb://foo?ssl=notABoolean",
			expectedError: errors.New(`bad value for ssl: "notABoolean"`),
		},
		"bad ?connect": {
			URL:           "mongodb://foo?connect=notValid",
			expectedError: errors.New(`unsupported ?connect= query parameter: "notValid"`),
		},
		"unknown option": {
			URL:           "mongodb://foo?xxx=yyy",
			expectedError: errors.New("unsupported connection URL query param: xxx=yyy"),
		},
	}

	for name, test := range tests {
		result, err := Parse(test.URL)

		if err != nil {
			if test.expectedError == nil {
				t.Errorf("[%s] Got unexpected error: %s", name, err)
			} else if err.Error() != test.expectedError.Error() {
				t.Errorf("[%s] Wrong error.\n    Actual: %s\n    Expected: %s",
					name, err, test.expectedError)
			}
		} else {
			if test.expectedError != nil {
				t.Errorf("[%s] Expected error, but did not get one", name)
			} else if !reflect.DeepEqual(result, test.expectedParsedURL) {
				t.Errorf("[%s] Incorrect result\n    Actual: %#v\n    Expected: %#v",
					name, result, test.expectedParsedURL)
			}
		}
	}
}
