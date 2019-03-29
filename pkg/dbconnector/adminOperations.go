package dbconnector

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/golang/glog"
)

// ClusterStatus - Status of a single cluster.
type ClusterStatus struct {
	Hash           string
	LastUpdated    string
	TotalResources int
	MaxQueueTime   int
}

// SaveClusterStatus saves the status of a cluster.
func SaveClusterStatus(clusterName string, status ClusterStatus) {

	var clusterStatus = []interface{}{fmt.Sprintf("cluster:%s", clusterName)}
	clusterStatus = append(clusterStatus, "hash", status.Hash)
	clusterStatus = append(clusterStatus, "lastUpdated", status.LastUpdated)
	clusterStatus = append(clusterStatus, "totalResources", status.TotalResources)
	clusterStatus = append(clusterStatus, "maxQueryTime", status.MaxQueueTime)

	glog.Info("Saving cluster status in redis. ", clusterStatus)
	result, err := conn.Do("HMSET", clusterStatus...)
	if err != nil {
		glog.Error("Error saving status of cluster ["+clusterName+"] to Redis.", err)
	}

	// Save the time of the last update. This is needed mostly for legacy reasons with the UI.
	// This is less relevant with the new collector-aggregator architecture. Will remove in the future.
	_, err2 := conn.Do("SET", "lastUpdatedTimestamp", time.Now().UnixNano()/int64(time.Millisecond))
	if err2 != nil {
		glog.Error("Error setting lastUpdatedTimestamp in Redis.", err)
	}
	glog.Info("Saved status of cluster ["+clusterName+"] to Redis. ", result)
}

// GetClusterStatus retrieves the status of a cluster.
func GetClusterStatus(clusterName string) (status ClusterStatus, e error) {

	clusterStatus, err := redis.StringMap(conn.Do("HGETALL", fmt.Sprintf("cluster:%s", clusterName)))
	if err != nil {
		glog.Error("Error getting status of cluster "+clusterName+" from Redis.", err)
		return status, err
	}

	totalResources, _ := strconv.ParseInt(clusterStatus["totalResources"], 10, 32)
	maxQueryTime, _ := strconv.ParseInt(clusterStatus["maxQueryTime"], 10, 32)

	status.Hash = clusterStatus["hash"]
	status.LastUpdated = clusterStatus["lastUpdated"]
	status.TotalResources = int(totalResources)
	status.MaxQueueTime = int(maxQueryTime)

	return status, nil
}
