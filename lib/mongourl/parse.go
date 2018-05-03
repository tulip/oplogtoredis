package mongourl

/*
This file incorporates code from github.com/hashicorp/vault
https://github.com/hashicorp/vault/blob/44aa151b78976c6da41dc63d93b40d2070b23277/plugins/database/mongodb/connection_producer.go#L156

Material taken from there is licensed under the Mozilla Public License, Version 2.0,
as found at https://github.com/hashicorp/vault/blob/44aa151b78976c6da41dc63d93b40d2070b23277/LICENSE
*/

import (
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/globalsign/mgo"
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
			poolLimit, err := strconv.Atoi(value)
			if err != nil {
				return nil, errors.New("bad value for maxPoolSize: " + value)
			}
			info.PoolLimit = poolLimit
		case "ssl":
			// Unfortunately, mgo doesn't support the ssl parameter in its MongoDB URI parsing logic, so we have to handle that
			// ourselves. See https://github.com/go-mgo/mgo/issues/84
			ssl, err := strconv.ParseBool(value)
			if err != nil {
				return nil, errors.New("bad value for ssl: " + value)
			}
			if ssl {
				info.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
					return tls.Dial("tcp", addr.String(), tlsConfig)
				}
			}
		case "connect":
			if value == "direct" {
				info.Direct = true
				break
			}
			if value == "replicaSet" {
				break
			}
			fallthrough
		default:
			return nil, errors.New("unsupported connection URL option: " + key + "=" + value)
		}
	}

	return &info, nil
}
