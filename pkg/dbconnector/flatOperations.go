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
)

// This file contains Arg getter functions used for getting redis args for flat queries.

// ClusterStatus - Status of a single cluster.
type ClusterStatus struct {
	Hash           string
	LastUpdated    string
	TotalResources int
	MaxQueueTime   int
}

// Executes an arbitrary flat redis command.
func Execute(command string, args []interface{}) (interface{}, error) {
	// Get connection from the pool
	// TODO may need a timeout here.
	conn := Pool.Get() // This will block if there aren't any valid connections that are available.
	defer conn.Close()
	redisResponse, err := conn.Do(command, args...)
	return redisResponse, err
}

// Saves the given clusterStatus in the database. First error is for encoding the cluster name, second is from the redis command.
func SaveClusterStatus(clusterName string, status ClusterStatus) (interface{}, error, error) {
	args, clusterNameError := saveClusterStatusArgs(clusterName, status)
	if clusterNameError != nil {
		return nil, clusterNameError, nil
	}
	res, err := Execute("HMSET", args)
	return res, clusterNameError, err
}

// Given a cluster name and status, gives back args to use for saving that cluster's status in redis.
func saveClusterStatusArgs(clusterName string, status ClusterStatus) ([]interface{}, error) {
	err := validateClusterName(clusterName)
	if err != nil {
		return nil, err
	}
	var args = []interface{}{fmt.Sprintf("cluster:%s", clusterName)}
	args = append(args, "hash", status.Hash)
	args = append(args, "lastUpdated", status.LastUpdated)
	args = append(args, "totalResources", status.TotalResources)
	args = append(args, "maxQueryTime", status.MaxQueueTime)
	return args, nil
}

// Gets status of the given cluster. First error is for encoding the cluster name, second is from the redis command.
func GetClusterStatus(clusterName string) (interface{}, error, error) {
	args, clusterNameError := getClusterStatusArgs(clusterName)
	if clusterNameError != nil {
		return nil, clusterNameError, nil
	}
	res, err := Execute("HGETALL", args)
	return res, clusterNameError, err

}

// Given a cluster name, gives back args to use for getting cluster status from redis.
// This returns []interface{} because I wanted all the arg functions to return that same thing, it would also work if it returned string and the GetClusterStatus made it into interface for Execute().
func getClusterStatusArgs(clusterName string) ([]interface{}, error) {
	err := validateClusterName(clusterName)
	if err != nil {
		return nil, err
	}
	return []interface{}{fmt.Sprintf("cluster:%s", clusterName)}, nil
}
