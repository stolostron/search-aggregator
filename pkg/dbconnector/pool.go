/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.

Copyright (c) 2020 Red Hat, Inc.
*/

package dbconnector

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"time"

	"github.com/golang/glog"
	"github.com/gomodule/redigo/redis"
	"github.com/stolostron/search-aggregator/pkg/config"
)

// A global redis pool for other parts of this package to use
var Pool *redis.Pool

const (
	IDLE_TIMEOUT = 60 // ReadinessProbe runs every 30 seconds, this keeps the connection alive between probe intervals.
	GRAPH_NAME   = "search-db"
)

// Initializes the pool using functions in this file.
// Also initializes the Store interface.
func init() {
	Pool = &redis.Pool{
		MaxIdle:      10, // Idle connections are connections that have been returned to the pool.
		MaxActive:    20, // Active connections = connections in-use + idle connections
		Dial:         getRedisConnection,
		TestOnBorrow: testRedisConnection,
		Wait:         true,
	}
	Store = RedisGraphStoreV2{}

}

func getRedisConnection() (redis.Conn, error) {
	var redisConn redis.Conn
	if config.Cfg.RedisSSHPort != "" {
		glog.V(2).Infof("Initializing new Redis SSH client with redisHost: %s redisSSHPort: %s",
			config.Cfg.RedisHost, config.Cfg.RedisSSHPort)
		caCert, err := ioutil.ReadFile("./rediscert/redis.crt")
		if err != nil {
			glog.Error("Error loading TLS certificate. Redis cert must be mounted at ./sslcert/redis.crt: ", err)
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsconf := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
			RootCAs: caCertPool,
		}
		redisConn, err = redis.Dial("tcp", net.JoinHostPort(config.Cfg.RedisHost, config.Cfg.RedisSSHPort),
			redis.DialTLSConfig(tlsconf),
			redis.DialUseTLS(true)) // Set this to false when you want to connect to redis via SSH from local laptop
		if err != nil {
			glog.Error("Error connecting redis using SSH.  Original error: ", err)
			return nil, err
		}

	} else {
		var err error
		glog.V(2).Infof("Initializing new Redis client with redisHost: %s redisPort: %s",
			config.Cfg.RedisHost, config.Cfg.RedisPort)

		redisConn, err = redis.Dial("tcp", net.JoinHostPort(config.Cfg.RedisHost, config.Cfg.RedisPort))
		if err != nil {
			glog.Error("Error connecting redis host.")
			return nil, err
		}
	}

	// If a password is provided, then use it to authenticate the Redis connection.
	if config.Cfg.RedisPassword != "" {
		glog.V(2).Info("Authenticating Redis client using password from REDIS_PASSWORD.")
		if _, err := redisConn.Do("AUTH", config.Cfg.RedisPassword); err != nil {
			glog.Error("Error authenticating Redis client. Original error: ", err)
			connError := redisConn.Close()
			if connError != nil {
				glog.Warning("Failed to close redis connection. Original error: ", connError)
			}
			return nil, err
		}
	} else {
		glog.Warning("REDIS_PASSWORD wasn't provided. Attempting to communicate without authentication.")
	}

	return redisConn, nil
}

// Used by the pool to test if redis connections are still okay. If they have been idle for less than a minute,
// just assumes they are okay. If not, calls PING.
func testRedisConnection(c redis.Conn, t time.Time) error {
	if time.Since(t) < IDLE_TIMEOUT*time.Second {
		return nil
	}
	_, err := c.Do("PING")
	return err
}
