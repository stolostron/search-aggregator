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

// Saves the given clusterStatus in the database.
func SaveClusterStatus(clusterName string, status ClusterStatus) (interface{}, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return nil, err
	}

	var args = []interface{}{fmt.Sprintf("cluster:%s", clusterName)}
	args = append(args, "hash", status.Hash)
	args = append(args, "lastUpdated", status.LastUpdated)
	args = append(args, "totalResources", status.TotalResources)
	args = append(args, "maxQueryTime", status.MaxQueueTime)

	res, err := Execute("HMSET", args)
	return res, err
}
