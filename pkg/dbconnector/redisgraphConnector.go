package dbconnector

import (
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

var conn redis.Conn
var graph rg.Graph

// Init creates new redis client.
func Init() {
	connectRedisClient()
}

func connectRedisClient() {
	redisHost := os.Getenv("redisHost")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("redisPort")
	if redisPort == "" {
		redisPort = "6379"
	}
	glog.Info("Initializing new Redis client with redisHost: ", redisHost, " redisPort: ", redisPort)

	conn, _ = redis.Dial("tcp", redisHost+":"+redisPort)
	graph = rg.Graph{}.New("icp-search", conn)
	client = &DbClient{
		Conn:  conn,
		Graph: graph,
		// Insert: insert,
	}
}

// GetDatabaseClient returns the DB client.
func GetDatabaseClient() *DbClient {
	if client == nil {
		Init()
	}

	glog.Info("Validating that Redis connection is still alive.")
	connOK, error := CheckDataConnection()

	if !connOK || error != nil {
		glog.Error("Redis connection problem.", error)
	}

	return client
}

// CheckDataConnection pings Redis to check if the connection is alive.
func CheckDataConnection() (bool, error) {
	if client == nil {
		Init()
	}
	result, err := client.Conn.Do("PING")

	if result == "PONG" {
		return true, nil
	}

	glog.Warning("Error pinging Redis, attempting to reconnect.", err)
	connectRedisClient()

	// TODO: We should validate the state of Redis here.

	result, err = client.Conn.Do("PING")

	return result == "PONG", err
}
