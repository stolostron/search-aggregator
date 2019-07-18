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
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// SyncEvent - Object sent by the collector with the resources to change.
type SyncEvent struct {
	ClearAll bool `json:"clearAll,omitempty"`

	AddResources    []*db.Resource
	UpdateResources []*db.Resource
	DeleteResources []DeleteResourceEvent

	AddEdges    []db.Edge
	DeleteEdges []db.Edge
}

// DeleteResourceEvent - Contains the information needed to delete an existing resource.
type DeleteResourceEvent struct {
	UID string `json:"uid,omitempty"`
}

// SyncResponse - Response to a SyncEvent
type SyncResponse struct {
	TotalAdded        int
	TotalUpdated      int
	TotalDeleted      int
	TotalResources    int
	TotalEdgesAdded   int
	TotalEdgesDeleted int
	TotalEdges        int
	AddErrors         []SyncError
	UpdateErrors      []SyncError
	DeleteErrors      []SyncError
	AddEdgeErrors     []SyncError
	DeleteEdgeErrors  []SyncError
}

// SyncError is used to respond with errors.
type SyncError struct {
	ResourceUID string
	Message     string // Often comes out of a golang error using .Error()
}

// SyncResources - Process Add, Update, and Delete events.
func SyncResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	glog.V(2).Info("SyncResources() for cluster: ", clusterName)

	response := SyncResponse{}

	// Function that sends the current response and the given status code.
	// If you want to bail out early, make sure to call return right after.
	respond := func(status int) {
		statusMessage := fmt.Sprintf(
			"Responding to cluster %s with status %d, stats: {Added: %d, Updated: %d, Deleted: %d, Edges Added: %d, Edges Deleted: %d, Total Resources: %d}",
			clusterName,
			status,
			response.TotalAdded,
			response.TotalUpdated,
			response.TotalDeleted,
			response.TotalEdgesAdded,
			response.TotalEdgesDeleted,
			response.TotalResources,
		)
		if status == http.StatusOK {
			glog.Infof(statusMessage)
		} else {
			glog.Errorf(statusMessage)
		}
		w.WriteHeader(status)
		encodeError := json.NewEncoder(w).Encode(response)
		if encodeError != nil {
			glog.Error("Error responding to SyncEvent:", encodeError, response)
		}
	}

	var syncEvent SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		glog.Error("Error decoding body of syncEvent: ", err)
		respond(http.StatusBadRequest)
		return
	}

	err = db.ValidateClusterName(clusterName)
	if err != nil {
		glog.Error("Invalid Cluster Name: ", clusterName)
		respond(http.StatusBadRequest)
		return
	}

	// This usually indicates that something has gone wrong, basically that the collector detected we are out of sync and wants us to start over.
	if syncEvent.ClearAll {
		glog.Infof("Clearing previous data for cluster %s", clusterName)
		_, err := db.DeleteCluster(clusterName)
		if err != nil {
			glog.Error("Error deleting current resources for cluster: ", err)
			if db.IsBadConnection(err) {
				respond(http.StatusServiceUnavailable)
				return
			} else if !db.IsGraphMissing(err) {
				respond(http.StatusBadRequest)
				return
			}
		}
	}

	// INSERT Resources

	// add cluster fields
	for i := range syncEvent.AddResources {
		syncEvent.AddResources[i].Properties["cluster"] = clusterName
	}

	insertResponse := db.ChunkedInsert(syncEvent.AddResources)
	response.TotalAdded = insertResponse.SuccessfulResources // could be 0
	if insertResponse.ConnectionError != nil {
		respond(http.StatusServiceUnavailable)
		return
	} else if insertResponse.ResourceErrors != nil {
		response.AddErrors = processSyncErrors(insertResponse.ResourceErrors, "inserted")
		respond(http.StatusBadRequest)
		return
	}

	// UPDATE Resources

	// add cluster fields
	for i := range syncEvent.UpdateResources {
		syncEvent.UpdateResources[i].Properties["cluster"] = clusterName
	}
	updateResponse := db.ChunkedUpdate(syncEvent.UpdateResources)
	response.TotalUpdated = updateResponse.SuccessfulResources // could be 0
	if updateResponse.ConnectionError != nil {
		respond(http.StatusServiceUnavailable)
		return
	} else if updateResponse.ResourceErrors != nil {
		response.UpdateErrors = processSyncErrors(updateResponse.ResourceErrors, "updated")
		respond(http.StatusBadRequest)
		return
	}

	// DELETE Resources

	// reformat to []string
	deleteUIDS := make([]string, 0, len(syncEvent.DeleteResources))
	for _, de := range syncEvent.DeleteResources {
		deleteUIDS = append(deleteUIDS, de.UID)
	}

	deleteResponse := db.ChunkedDelete(deleteUIDS)
	response.TotalDeleted = deleteResponse.SuccessfulResources // could be 0
	if deleteResponse.ConnectionError != nil {
		respond(http.StatusServiceUnavailable)
		return
	} else if deleteResponse.ResourceErrors != nil {
		response.DeleteErrors = processSyncErrors(deleteResponse.ResourceErrors, "deleted")
		respond(http.StatusBadRequest)
		return
	}

	// Insert Edges
	insertEdgeResponse := db.ChunkedInsertEdge(syncEvent.AddEdges)
	response.TotalEdgesAdded = insertEdgeResponse.SuccessfulResources // could be 0
	if insertEdgeResponse.ConnectionError != nil {
		respond(http.StatusServiceUnavailable)
		return
	} else if insertEdgeResponse.ResourceErrors != nil {
		response.AddEdgeErrors = processSyncErrors(insertEdgeResponse.ResourceErrors, "inserted by edge")
		respond(http.StatusBadRequest)
		return
	}

	// Delete Edges
	deleteEdgeResponse := db.ChunkedDeleteEdge(syncEvent.DeleteEdges)
	response.TotalEdgesDeleted = deleteEdgeResponse.SuccessfulResources // could be 0
	if deleteEdgeResponse.ConnectionError != nil {
		respond(http.StatusServiceUnavailable)
		return
	} else if deleteEdgeResponse.ResourceErrors != nil {
		response.DeleteEdgeErrors = processSyncErrors(deleteEdgeResponse.ResourceErrors, "removed by edge")
		respond(http.StatusBadRequest)
		return
	}

	glog.V(2).Infof("Done updating resources for cluster %s, preparing response", clusterName)
	response.TotalResources = computeNodeCount(clusterName) // This goes out to the DB through a work order, so it can take a second
	response.TotalEdges = computeIntraEdges(clusterName)
	respond(http.StatusOK)
}

// internal function to inline the errors
func processSyncErrors(re map[string]error, verb string) []SyncError {
	ret := []SyncError{}
	for uid, e := range re {
		glog.Errorf("Resource %s cannot be %s: %s", uid, verb, e)
		ret = append(ret, SyncError{
			ResourceUID: uid,
			Message:     e.Error(),
		})
	}

	return ret
}
