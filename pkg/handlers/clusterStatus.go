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

	"github.com/golang/glog"
	"github.com/gorilla/mux"

	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// GetClusterStatus responds with the cluster status.
func GetClusterStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	glog.Info("GetClusterStatus() for cluster:", clusterName)

	dbConn, err := dbconnector.GetDatabaseClient()
	if err != nil {
		glog.Error("Error getting redis client: ", err)
	}

	totalResources, currentHash := computeHash(&dbConn.Graph, clusterName)
	clusterStatus, _ := dbconnector.GetClusterStatus(clusterName)

	clusterStatus.TotalResources = totalResources
	clusterStatus.Hash = currentHash

	encodeError := json.NewEncoder(w).Encode(clusterStatus)
	if encodeError != nil {
		glog.Error("Error responding to GetClusterStatus:", encodeError, clusterStatus)
	}
}
