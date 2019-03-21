package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	rg "github.com/redislabs/redisgraph-go"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// SyncEvent - Object sent by the collector with the resources to change.
type SyncEvent struct {
	Hash            string `json:"hash,omitempty"`
	ClearAll        bool
	AddResources    []AddResourceEvent
	UpdateResources []UpdateResourceEvent
	DeleteResources []DeleteResourceEvent
	// TODO: AddEdges, DeleteEdges
}

// AddResourceEvent - Contains the information needed to add a new resource.
type AddResourceEvent struct {
	Kind       string `json:"kind,omitempty"`
	UID        string `json:"uid,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Properties map[string]interface{}
}

// UpdateResourceEvent - Contains the information needed to update an existing resource.
type UpdateResourceEvent struct {
	Kind       string `json:"kind,omitempty"`
	UID        string `json:"uid,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Properties map[string]interface{}
}

// DeleteResourceEvent - Contains the information needed to delete an existing resource.
type DeleteResourceEvent struct {
	UID string `json:"uid,omitempty"`
}

// SyncResponse - Response to a SyncEvent
type SyncResponse struct {
	Hash             string
	TotalAdded       int
	TotalChanged     int
	TotalDeleted     int
	TotalResources   int
	UpdatedTimestamp time.Time
	Message          string
}

// SyncResources - Process Add, Update, and Delete events.
func SyncResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	glog.Info("SyncResources() for cluster:", clusterName)

	dbConn := dbconnector.GetDatabaseClient()

	var syncEvent SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		glog.Error("Error decoding body of syncEvent:", err)
	}

	if syncEvent.ClearAll {
		query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' DELETE n"
		_, err := dbConn.Graph.Query(query)
		if err != nil {
			glog.Error("Error running RedisGraph delete query:", err, query)
		}
		glog.Info("!!! Deleted all previous resources for cluster:", clusterName)
	}

	addResources := syncEvent.AddResources
	updateResources := syncEvent.UpdateResources
	deleteResources := syncEvent.DeleteResources

	// ADD resources
	for _, resource := range addResources {
		glog.Info("Adding resource: ", resource)

		// TODO: Enforce required values (Kind, UID, Hash)
		// TODO: Do I need to sanitize inputs?
		// TODO: Need special processing for lists (labels and roles).

		resource.Properties["kind"] = resource.Kind
		resource.Properties["cluster"] = clusterName
		resource.Properties["_uid"] = resource.UID
		resource.Properties["_hash"] = resource.Hash
		resource.Properties["_rbac"] = "UNKNOWN" // TODO: This must be the namespace of the cluster.

		err := dbConn.Graph.AddNode(&rg.Node{
			ID:         resource.UID, // FIXME: This is supported by RedisGraph but doesn't work in the redisgraph-go client.
			Label:      resource.Kind,
			Properties: resource.Properties,
		})
		if err != nil {
			glog.Error("Error adding resource node:", err, resource)
		}
	}
	_, error := dbConn.Graph.Flush()
	if error != nil {
		glog.Error("Error adding nodes in RedisGraph.", error)
		// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
	}

	// UPDATE resources
	for _, resource := range updateResources {
		glog.Info("Updating resource: ", resource)
		// FIXME: Properly update resource. Deleting and recreating is very lazy.
		query := "MATCH (n) WHERE n._uid = '" + resource.UID + "' DELETE n"

		_, err := dbConn.Graph.Query(query)
		if err != nil {
			glog.Error("Error executing query:", err, query)
		}
		resource.Properties["kind"] = resource.Kind
		resource.Properties["cluster"] = clusterName
		resource.Properties["_uid"] = resource.UID
		resource.Properties["_hash"] = resource.Hash
		resource.Properties["_rbac"] = "UNKNOWN" // TODO: This must be the namespace of the cluster.

		error := dbConn.Graph.AddNode(&rg.Node{
			ID:         resource.UID, // FIXME: This doesn't work in the redisgraph-go client.
			Label:      resource.Kind,
			Properties: resource.Properties,
		})
		if error != nil {
			glog.Error("Error updating resource node:", error, resource)
		}
	}
	_, updateErr := dbConn.Graph.Flush()
	if updateErr != nil {
		glog.Error("Error updating nodes in RedisGraph.", updateErr)
		// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
	}

	// DELETE resources
	for _, resource := range deleteResources {
		glog.Info("Deleting resource: ", resource)
		query := "MATCH (n) WHERE n._uid = '" + resource.UID + "' DELETE n"
		_, deleteErr := dbConn.Graph.Query(query)
		if deleteErr != nil {
			glog.Error("Error deleting nodes in RedisGraph.", deleteErr)
			// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
		}
	}

	// Updating cluster status in cache.
	updatedTimestamp := time.Now()
	totalResources, currentHash := computeHash(&dbConn.Graph, clusterName)
	var clusterStatus = []interface{}{fmt.Sprintf("cluster:%s", clusterName)} // TODO: I'll worry about strings later.
	clusterStatus = append(clusterStatus, "hash", currentHash)
	clusterStatus = append(clusterStatus, "lastUpdated", updatedTimestamp)

	_, deleteErr := dbConn.Conn.Do("HMSET", clusterStatus...)

	if deleteErr != nil {
		glog.Error("Error deleting nodes in RedisGraph.", deleteErr)
		// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
	}

	var response = SyncResponse{
		Hash:             currentHash,
		TotalAdded:       len(addResources),
		TotalChanged:     len(updateResources),
		TotalDeleted:     len(deleteResources),
		TotalResources:   totalResources,
		UpdatedTimestamp: updatedTimestamp,
		Message:          "TODO: Maybe we don't need this message field.",
	}
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		glog.Error("Error responding to SyncEvent:", encodeError, response)
	}
}
