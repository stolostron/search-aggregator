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

	dbConn := dbconnector.GetDatabaseClient()

	totalResources, currentHash := computeHash(&dbConn.Graph, clusterName)
	clusterStatus, _ := dbconnector.GetClusterStatus(clusterName)

	clusterStatus.TotalResources = totalResources
	clusterStatus.Hash = currentHash

	encodeError := json.NewEncoder(w).Encode(clusterStatus)
	if encodeError != nil {
		glog.Error("Error responding to GetClusterStatus:", encodeError, clusterStatus)
	}
}
