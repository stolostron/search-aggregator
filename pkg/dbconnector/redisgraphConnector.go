package dbconnector

import (
	"os"
	"sync"

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
var once sync.Once

// GetDatabaseClient returns a Redis client (singleton).  This won't be exported in the future.
func GetDatabaseClient() *DbClient {
	once.Do(func() {
		redisHost := os.Getenv("redisHost")
		if redisHost == "" {
			redisHost = "localhost"
		}
		redisPort := os.Getenv("redisPort")
		if redisPort == "" {
			redisPort = "6379"
		}
		glog.Info("Initializing new Redis client with redisHost: ", redisHost, " redisPort: ", redisPort)

		conn, _ := redis.Dial("tcp", redisHost+":"+redisPort)
		graph := rg.Graph{}.New("icp-search", conn)
		client = &DbClient{
			Conn:  conn,
			Graph: graph,
		}

	})
	return client
}

// CheckDataConnection pings Redis to check if the connection is alive.
func CheckDataConnection() (bool, error) {
	db := GetDatabaseClient()
	result, err := db.Conn.Do("PING")
	return result == "PONG", err
}
