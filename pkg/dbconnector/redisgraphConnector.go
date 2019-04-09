/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"errors"
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
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	glog.Info("Initializing new Redis client with redisHost: ", redisHost, " redisPort: ", redisPort)

	conn, err := redis.Dial("tcp", redisHost+":"+redisPort)
	if err != nil {
		glog.Error("Error connecting redis host.")
		return err
	}

	// If we have a REDIS_PASSWORD, we'll try to authenticate the Redis client.
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword != "" {
		glog.Info("Authenticating Redis client.")
		if _, err := conn.Do("AUTH", redisPassword); err != nil {
			glog.Error("Error authenticating Redis client.")
			return err
		}
	} else {
		glog.Warning("REDIS_PASSWORD wasn't provided. Attempting to communicate without authentication.")
	}

	graph = rg.Graph{}.New("icp-search", conn)
	client = &DbClient{
		Conn:  conn,
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
