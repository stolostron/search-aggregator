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
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// SyncEvent - Object sent by the collector with the resources to change.
type SyncEvent struct {
	Hash            string `json:"hash,omitempty"`
	ClearAll        bool   `json:"clearAll,omitempty"`
	AddResources    []*db.Resource
	UpdateResources []*db.Resource
	DeleteResources []DeleteResourceEvent
	// TODO: AddEdges, DeleteEdges
}

// DeleteResourceEvent - Contains the information needed to delete an existing resource.
type DeleteResourceEvent struct {
	UID string `json:"uid,omitempty"`
}

// SyncResponse - Response to a SyncEvent
type SyncResponse struct {
	Hash             string
	TotalAdded       int
	TotalUpdated     int
	TotalDeleted     int
	TotalResources   int
	UpdatedTimestamp time.Time
	AddErrors        []SyncError
	UpdateErrors     []SyncError
	DeleteErrors     []SyncError
}

// SyncError is used to respond whith errors.
type SyncError struct {
	ResourceUID string
	Message     string // Often comes out of a golang error using .Error()
}

// SyncResources - Process Add, Update, and Delete events.
func SyncResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	glog.Info("SyncResources() for cluster: ", clusterName)

	response := SyncResponse{}

	// Function that sends the current response and the given status code.
	// If you want to bail out early, make sure to call return right after.
	respond := func(status int) {
		if status == http.StatusOK {
			glog.Infof("Responding to cluster %s with status %d, stats: {Added: %d, Updated: %d, Deleted: %d, Total: %d}", clusterName, status, response.TotalAdded, response.TotalUpdated, response.TotalDeleted, response.TotalResources)
		} else {
			glog.Errorf("Responding to cluster %s with status %d, stats: {Added: %d, Updated: %d, Deleted: %d, Total: %d}", clusterName, status, response.TotalAdded, response.TotalUpdated, response.TotalDeleted, response.TotalResources)
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

	// This usually indicates that something has gone wrong, basically that the collector detected we are out of sync and wants us to start over.
	if syncEvent.ClearAll {
		glog.Infof("Clearing previous data for cluster %s", clusterName)
		_, encodingErr, err := db.DeleteCluster(clusterName)
		if err != nil {
			glog.Error("Error deleting current resources for cluster: ", err)
			if db.IsBadConnection(err) {
				respond(http.StatusServiceUnavailable)
				return
			} else {
				respond(http.StatusBadRequest)
				return
			}
		}
		if encodingErr != nil {
			glog.Error("Invalid Cluster Name: ", clusterName)
			respond(http.StatusBadRequest)
			return
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
		for uid, e := range insertResponse.ResourceErrors {
			glog.Errorf("Resource %s cannot be inserted: %s", uid, e)
			response.AddErrors = append(response.AddErrors, SyncError{
				ResourceUID: uid,
				Message:     e.Error(),
			})
		}
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
		for uid, e := range updateResponse.ResourceErrors {
			glog.Errorf("Resource %s cannot be updated: %s", uid, e)
			response.UpdateErrors = append(response.UpdateErrors, SyncError{
				ResourceUID: uid,
				Message:     e.Error(),
			})
		}
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
		for uid, e := range deleteResponse.ResourceErrors {
			glog.Errorf("Resource %s cannot be deleted: %s", uid, e)
			response.DeleteErrors = append(response.DeleteErrors, SyncError{
				ResourceUID: uid,
				Message:     e.Error(),
			})
		}
		respond(http.StatusBadRequest)
		return
	}

	glog.Infof("Done commiting changes for cluster %s, preparing response", clusterName)

	// Updating cluster status in cache.
	response.UpdatedTimestamp = time.Now()
	response.TotalResources, response.Hash = computeHash(clusterName) // This goes out to the DB through a work order, so it can take a second
	status := db.ClusterStatus{
		Hash:           response.Hash,
		LastUpdated:    response.UpdatedTimestamp.String(),
		TotalResources: response.TotalResources,
	}

	_, clusterNameErr, err := db.SaveClusterStatus(clusterName, status)
	if clusterNameErr != nil {
		glog.Errorf("Could not save cluster status because of invalid cluster name: %s", clusterName)
		respond(http.StatusBadRequest)
		return
	}

	if err != nil {
		glog.Errorf("Failed to save cluster status for cluster %s: %s", clusterName, err)
		if db.IsBadConnection(err) {
			respond(http.StatusServiceUnavailable)
			return
		} else {
			respond(http.StatusBadRequest)
			return
		}
	}

	respond(http.StatusOK)
}
