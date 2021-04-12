/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.

Copyright (c) 2020 Red Hat, Inc.
*/
// Copyright Contributors to the Open Cluster Management project

package dbconnector

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"time"

	"github.com/golang/glog"
	"github.com/gomodule/redigo/redis"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
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
		TestOnBorrow: validateRedisConnection,
		Wait:         true,
	}
	Store = RedisGraphStoreV2{}

}

func getRedisConnection() (redis.Conn, error) {
	var port string
	var sslEnabled bool

	host := config.Cfg.RedisHost
	if config.Cfg.RedisSSHPort != "" {
		port = config.Cfg.RedisSSHPort
		sslEnabled = true
	} else {
		port = config.Cfg.RedisPort
		sslEnabled = false
	}

	glog.V(2).Infof("Initializing Redis client with Host: %s, Port: %s, using SSL: %t", host, port, sslEnabled)

	tlsconf := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
		// RootCAs: caCertPool,
	}

	// Attempt to add RootCAs to tlsconf.
	caCert, certErr := ioutil.ReadFile("./rediscert/redis.crt")
	if certErr != nil {
		if sslEnabled {
			// If REDIS_SSL_PORT was provided we assume that SSL is required.
			glog.Error("REDIS_SSH_PORT is configured, but can't load cert. ", certErr)
			return nil, certErr
		} else {
			glog.Warning("Using insecure Redis connection.")
			glog.Warning("To enable SSL provide REDIS_SSL_PORT and ./rediscert/redis.crt")
		}
	} else {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsconf.RootCAs = caCertPool
	}

	redisConn, err := redis.Dial("tcp",
		net.JoinHostPort(host, port),
		redis.DialTLSConfig(tlsconf),
		redis.DialUseTLS(sslEnabled),
		redis.DialConnectTimeout(30*time.Second))
	if err != nil {
		glog.Error("Error connecting redis. Original error: ", err)
		return nil, err
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
func validateRedisConnection(c redis.Conn, t time.Time) error {
	if time.Since(t) < IDLE_TIMEOUT*time.Second {
		return nil
	}
	_, err := c.Do("PING")
	return err
}
