/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2020 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
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
