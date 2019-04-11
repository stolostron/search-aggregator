/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"os"

	"github.com/golang/glog"

	"github.com/gomodule/redigo/redis"
	rg "github.com/redislabs/redisgraph-go"
)

// DbClient is exported for now, but it shouldn't be in the future.
type DbClient struct {
	Conn  redis.Conn
	Graph rg.Graph
}

var client *DbClient

var graph rg.Graph

// Init creates new redis client.
func Init() {
	_ = connectRedisClient()
}

func connectRedisClient() error {
	var redisconn redis.Conn
	var rediserr error
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisSSHPort := os.Getenv("REDIS_SSH_PORT")
	if redisSSHPort != "" {
		glog.Info("Initializing new Redis SSH client with redisHost: ", redisHost, " redisSSHPort: ", redisSSHPort)
		caCert, err := ioutil.ReadFile("./rediscert/redis.crt")
		if err != nil {
			glog.Error("Error loading TLS certificate. Redis Certificate must be mounted at ./sslcert/redis.crt")
			glog.Error(err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsconf := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
			RootCAs: caCertPool,
		}
		redisconn, rediserr = redis.Dial("tcp", redisHost+":"+redisSSHPort,
			redis.DialTLSConfig(tlsconf),
			redis.DialUseTLS(true))
		if rediserr != nil {
			glog.Error("Error connecting redis using SSH .")
			return rediserr
		}

	} else {
		glog.Info("Initializing new Redis client with redisHost: ", redisHost, " redisPort: ", redisPort)

		redisconn, rediserr = redis.Dial("tcp", redisHost+":"+redisPort)
		if rediserr != nil {
			glog.Error("Error connecting redis host.")
			return rediserr
		}

	}
	// If we have a REDIS_PASSWORD, we'll try to authenticate the Redis client.
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword != "" {
		glog.Info("Authenticating Redis client.")
		if _, rediserr := redisconn.Do("AUTH", redisPassword); rediserr != nil {
			glog.Error("Error authenticating Redis client.")
			return rediserr
		}
	} else {
		glog.Warning("REDIS_PASSWORD wasn't provided. Attempting to communicate without authentication.")
	}
	graph = rg.Graph{}.New("icp-search", redisconn)
	client = &DbClient{
		Conn:  redisconn,
		Graph: graph,
	}

	return nil
}

// GetDatabaseClient returns the DB client.
func GetDatabaseClient() (*DbClient, error) {
	// singleton get
	if client == nil {
		if err := connectRedisClient(); err != nil {
			return nil, err
		}
	}

	glog.Info("Validating that Redis connection is still alive.")
	result, err := client.Conn.Do("PING")
	if err != nil {
		return nil, err
	}

	if result != "PONG" {
		return nil, errors.New("Error validating redis connection")
	}

	return client, nil
}
