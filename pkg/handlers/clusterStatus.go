package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"

	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// ClusterStatus - Status of a single cluster.
type ClusterStatus struct {
	Hash           string
	LastUpdated    string
	TotalResources int
	MaxQueueTime   int
	Message        string
}

// GetClusterStatus responds with the cluster status.
func GetClusterStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	glog.Info("GetClusterStatus() for cluster:", clusterName)

	dbConn := dbconnector.GetDatabaseClient()

	clusterStatus, err := dbConn.Conn.Do("HGETALL", fmt.Sprintf("cluster:%s", clusterName)) // TODO: I'll worry about strings later.
	if err != nil {
		glog.Error("Error getting status of cluster"+clusterName+" from Redis.", err)
	}
	var status = []interface{}{clusterStatus}
	glog.Info("Cluster status: ", status[0])

	totalResources, currentHash := computeHash(&dbConn.Graph, clusterName)

	var response = ClusterStatus{
		Hash:           currentHash,
		Message:        "ClusterStatus",
		LastUpdated:    "TODO: Get lastUpdated timestamp from Redis.",
		TotalResources: totalResources,
	}
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		glog.Error("Error responding to GetClusterStatus:", encodeError, response)
	}
}
