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
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// SyncEvent - Object sent by the collector with the resources to change.
type SyncEvent struct {
	Hash            string `json:"hash,omitempty"`
	ClearAll        bool   `json:"clearAll,omitempty"`
	AddResources    []dbconnector.Resource
	UpdateResources []dbconnector.Resource
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
	Errors           []SyncError
}

// SyncError is used to respond whith errors.
type SyncError struct {
	ResourceUID string
	Message     string
}

type stats struct {
	resourcesAdded   int
	resourcesUpdated int
	resourcesDeleted int
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
		err := dbconnector.DeleteCluster(clusterName)
		if err != nil {
			glog.Error("Error deleting current resources for cluster.", err)
		}
	}

	stats := stats{
		resourcesAdded:   0,
		resourcesUpdated: 0,
		resourcesDeleted: 0,
	}
	addResources := syncEvent.AddResources
	updateResources := syncEvent.UpdateResources
	deleteResources := syncEvent.DeleteResources

	var syncErrors = make([]SyncError, 0)

	// ADD resources
	glog.Info("Adding ", len(addResources), " resources.")
	if len(addResources) > 0 {
		var resourcesChunk = make([]interface{}, 0)
		for _, resource := range addResources {
			resource.Cluster = clusterName
			// glog.Info("Adding resource: ", resource)

			resourcesChunk = append(resourcesChunk, resource)

			insertError := dbconnector.Insert(&resource)

			if insertError != nil {
				syncErrors = append(syncErrors, SyncError{
					ResourceUID: resource.UID,
					Message:     insertError.Error(),
				})
				continue
			}
			stats.resourcesAdded++

			// Commiting small chunks isn't the best for performance, but it's easier to debug problems and it
			// helps to prevent total failure in case we get a bad record. Will increase as we solidify our logic.
			if (stats.resourcesAdded+1)%10 == 0 {
				err := dbconnector.Flush()
				if err != nil {
					glog.Error("Error while commiting resources:")
					for i, res := range resourcesChunk {
						glog.Error("Resource[", i, "]\n", res)
					}
					panic("Error while commiting resources:")
				}
				resourcesChunk = make([]interface{}, 0)
			}
		}
	}

	// UPDATE resources
	if len(updateResources) > 0 {
		for _, resource := range updateResources {
			resource.Cluster = clusterName
			glog.Info("Updating resource: ", resource)

			updateError := dbconnector.Update(&resource)

			if updateError != nil {
				syncErrors = append(syncErrors, SyncError{
					ResourceUID: resource.UID,
					Message:     updateError.Error(),
				})
				continue
			}
			stats.resourcesUpdated++
		}
	}

	// DELETE resources
	if len(deleteResources) > 0 {
		for _, resource := range deleteResources {
			glog.Info("Deleting resource: ", resource)

			deleteError := dbconnector.Delete(resource.UID)
			if deleteError != nil {
				syncErrors = append(syncErrors, SyncError{
					ResourceUID: resource.UID,
					Message:     deleteError.Error(),
				})
				continue
			}
			stats.resourcesDeleted++
		}
	}

	// Only flush if any resources were added, updated, or deleted, otherwise it will cause an error.
	if stats.resourcesAdded > 0 || stats.resourcesUpdated > 0 || stats.resourcesDeleted > 0 {
		error := dbconnector.Flush()
		if error != nil {
			syncErrors = append(syncErrors, SyncError{
				ResourceUID: "UNKNOWN",
				Message:     "Error commiting resources to RedisGraph.",
			})
		}
	}

	glog.Info("Done commiting changes, preparing response.")

	// Updating cluster status in cache.
	updatedTimestamp := time.Now()
	totalResources, currentHash := computeHash(&dbConn.Graph, clusterName)
	status := dbconnector.ClusterStatus{
		Hash:           currentHash,
		LastUpdated:    updatedTimestamp.String(),
		TotalResources: totalResources,
	}

	dbconnector.SaveClusterStatus(clusterName, status)

	var response = SyncResponse{
		Hash:             currentHash,
		TotalAdded:       stats.resourcesAdded,
		TotalUpdated:     stats.resourcesUpdated,
		TotalDeleted:     stats.resourcesDeleted,
		TotalResources:   totalResources,
		UpdatedTimestamp: updatedTimestamp,
		Errors:           syncErrors,
	}
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		glog.Error("Error responding to SyncEvent:", encodeError, response)
	}
}
