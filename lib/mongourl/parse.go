// Package mongourl parses Mongo URLs. It includes support for URL parameters
// not supported by mgo.ParseURL, such as `?ssl=true`.
package mongourl

/*
This file incorporates code from github.com/hashicorp/vault
https://github.com/hashicorp/vault/blob/44aa151b78976c6da41dc63d93b40d2070b23277/plugins/database/mongodb/connection_producer.go#L156

Material taken from there is licensed under the Mozilla Public License, Version 2.0,
as found at https://github.com/hashicorp/vault/blob/44aa151b78976c6da41dc63d93b40d2070b23277/LICENSE
*/

import (
	"crypto/tls"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/pkg/errors"
)

// DefaultTimeout is the timeout value applied to all parsed URLs.
//
// See https://godoc.org/github.com/globalsign/mgo#DialInfo for details on
// what it does.
//
// You can modify this value.
var DefaultTimeout = 10 * time.Second

// The TLS config we use to connect to mongo via SSL. We keep this as a
// package global so that testing code can modify it.
var tlsConfig = &tls.Config{}

// Parse parses a mongo URL.
func Parse(mongoURL string) (*mgo.DialInfo, error) {
	url, err := url.Parse(mongoURL)
	if err != nil {
		return nil, err
	}

	info := mgo.DialInfo{
		Addrs:    strings.Split(url.Host, ","),
		Database: strings.TrimPrefix(url.Path, "/"),
		Timeout:  DefaultTimeout,
	}

	if url.User != nil {
		info.Username = url.User.Username()
		info.Password, _ = url.User.Password()
	}

	query := url.Query()
	for key, values := range query {
		optionErr := handleOption(&info, key, values)
		if optionErr != nil {
			return nil, optionErr
		}
	}

	return &info, nil
}

func handleOption(info *mgo.DialInfo, key string, values []string) error {
	var value string
	if len(values) > 0 {
		value = values[0]
	}

	switch key {
	case "authSource":
		info.Source = value
	case "authMechanism":
		info.Mechanism = value
	case "gssapiServiceName":
		info.Service = value
	case "replicaSet":
		info.ReplicaSetName = value
	case "maxPoolSize":
		err := handleMaxPoolSize(info, value)
		if err != nil {
			return err
		}
	case "ssl":
		err := handleSSL(info, value)
		if err != nil {
			return err
		}
	case "connect":
		err := handleConnect(info, value)
		if err != nil {
			return err
		}
	default:
		return errors.Errorf("unsupported connection URL query param: %v=%v", key, value)
	}

	return nil
}

func handleMaxPoolSize(info *mgo.DialInfo, value string) error {
	poolLimit, err := strconv.Atoi(value)

	if err != nil {
		return errors.Errorf("bad value for maxPoolSize: %q", value)
	}

	info.PoolLimit = poolLimit
	return nil
}

// Unfortunately, mgo doesn't support the ssl parameter in its MongoDB URI parsing logic, so we have to handle that
// ourselves. See https://github.com/go-mgo/mgo/issues/84
func handleSSL(info *mgo.DialInfo, value string) error {
	ssl, err := strconv.ParseBool(value)
	if err != nil {
		return errors.Errorf("bad value for ssl: %q", value)
	}
	if ssl {
		info.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			return tls.Dial("tcp", addr.String(), tlsConfig)
		}
	}

	return nil
}

func handleConnect(info *mgo.DialInfo, value string) error {
	if value == "direct" {
		info.Direct = true
		return nil
	}

	if value == "replicaSet" {
		return nil
	}

	return errors.Errorf("unsupported ?connect= query parameter: %q", value)
}
