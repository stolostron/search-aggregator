/*
 * (C) Copyright IBM Corporation 2019 All Rights Reserved
 * Copyright Contributors to the Open Cluster Management project
 */

package handlers

import (
	"testing"
)

//Checks that multiple clusters can update simultaneously without causing a concurrent map write error.
func TestHandlerMetrics(t *testing.T) {

	go InitClusterMetrics("local-cluster")
	go InitClusterMetrics("remote-cluster")
	t.Log("SyncMetrics completed successfully")
}

// Function to call InitSyncMetrics multiple times to make multiple updates to the map
func InitClusterMetrics(clusterName string) {
	i := 1
	for i < 500 {
		metrics := InitSyncMetrics(clusterName)
		metrics.CompleteSyncEvent()
		i++
	}
}
