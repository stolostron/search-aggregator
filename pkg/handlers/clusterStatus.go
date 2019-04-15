/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/golang/glog"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// GetClusterStatus responds with the cluster status.
func GetClusterStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	glog.Info("GetClusterStatus() for cluster:", clusterName)

	totalResources, currentHash := computeHash(clusterName)
	resp, clusterNameErr, err := db.GetClusterStatus(clusterName)
	if clusterNameErr != nil {
		glog.Errorf("Invalid Cluster Name %s: %s", clusterName, clusterNameErr)
		// TODO return 400
	}
	if err != nil {
		glog.Errorf("Error getting cluster status for cluster %s: %s", clusterName, err)
		// TODO return 500
	}

	// TODO this has a couple things in it that are redundant with what comes out of computeHash - unsure what should be done here but left the old behavior for now
	status, stringMapErr := redis.StringMap(resp, err)
	if stringMapErr != nil {
		glog.Errorf("Error getting cluster status for cluster %s: %s", clusterName, stringMapErr)
	}

	maxQueryTime, _ := strconv.ParseInt(status["maxQueryTime"], 10, 32)

	clusterStatus := db.ClusterStatus{
		Hash:           currentHash,
		LastUpdated:    status["lastUpdated"],
		TotalResources: int(totalResources),
		MaxQueueTime:   int(maxQueryTime),
	}

	encodeError := json.NewEncoder(w).Encode(clusterStatus)
	if encodeError != nil {
		glog.Error("Error responding to GetClusterStatus:", encodeError, clusterStatus)
	}
}
