/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

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

	dbClient, dbClientErr := GetDatabaseClient()
	if dbClientErr != nil || dbClient.Conn == nil {
		glog.Warning("Unable to save sync status for cluster [" + clusterName + "] because of Redis connection problem.")
	} else {
		glog.Info("Saving cluster sync status in redis. ", clusterStatus)
		result, err := dbClient.Conn.Do("HMSET", clusterStatus...)
		if err != nil {
			glog.Error("Error saving status of cluster ["+clusterName+"] to Redis.", err)
		}

		// Save the time of the last update. This is needed mostly for legacy reasons with the UI.
		// This is less relevant with the new collector-aggregator architecture. Will remove in the future.
		_, err2 := dbClient.Conn.Do("SET", "lastUpdatedTimestamp", time.Now().UnixNano()/int64(time.Millisecond))
		if err2 != nil {
			glog.Error("Error setting lastUpdatedTimestamp in Redis.", err)
		}
		glog.Info("Saved sync status for cluster ["+clusterName+"] to Redis. ", result)
	}
}

// GetClusterStatus retrieves the status of a cluster.
func GetClusterStatus(clusterName string) (status ClusterStatus, e error) {
	dbClient, dbClientErr := GetDatabaseClient()
	if dbClientErr != nil || dbClient.Conn == nil {
		glog.Warning("Unable to get status for cluster [" + clusterName + "] because of Redis connection problem.")
		return ClusterStatus{}, fmt.Errorf("Unable to get status for cluster [%s] because of Redis connection problem", clusterName)
	}
	clusterStatus, err := redis.StringMap(dbClient.Conn.Do("HGETALL", fmt.Sprintf("cluster:%s", clusterName)))
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
