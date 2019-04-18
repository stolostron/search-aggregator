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

const CHUNK_SIZE = 20

// SyncEvent - Object sent by the collector with the resources to change.
type SyncEvent struct {
	Hash            string `json:"hash,omitempty"`
	ClearAll        bool   `json:"clearAll,omitempty"`
	AddResources    []db.Resource
	UpdateResources []db.Resource
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
	glog.Info("SyncResources() for cluster: ", clusterName)

	var syncEvent SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		glog.Error("Error decoding body of syncEvent: ", err)
	}

	// This usually indicates that something has gone wrong, basically that the collector detected we are out of sync and wants us to start over.
	if syncEvent.ClearAll {
		glog.Infof("Clearing previous data for cluster %s", clusterName)
		_, encodingErr, err := db.DeleteCluster(clusterName)
		if err != nil {
			glog.Error("Error deleting current resources for cluster: ", err)
			// TODO return 500
		}
		if encodingErr != nil {
			glog.Error("Invalid Cluster Name: ", clusterName)
			// TODO return 400
		}
	}

	stats := stats{
		resourcesAdded:   0,
		resourcesUpdated: 0,
		resourcesDeleted: 0,
	}

	var syncErrors = make([]SyncError, 0)

	// ADD resources
	glog.Info("Adding ", len(syncEvent.AddResources), " resources.")
	chunk := make([]*db.Resource, 0, CHUNK_SIZE)
	for i := range syncEvent.AddResources {
		if syncEvent.AddResources[i].Properties != nil {
			syncEvent.AddResources[i].Properties["cluster"] = clusterName

			chunk = append(chunk, &syncEvent.AddResources[i])
		} else { // This really shouldn't happen, it's suspected this happens because of a collector bug. Adding this for now while we figure that out.{
			glog.Warningf("Skipping resource with UID %s on cluster %s because of nil properties", syncEvent.AddResources[i].UID, clusterName)
		}

		// Commiting small chunks isn't the best for performance, but it's easier to debug problems and it
		// helps to prevent total failure in case we get a bad record. Will increase as we solidify our logic.
		if (i != 0 && (i+1)%CHUNK_SIZE == 0) || i == len(syncEvent.AddResources)-1 {
			badResources := 0
			_, _, err := db.Insert(chunk)
			if err != nil {
				glog.Errorf("Error inserting resources for cluster %s: %s", clusterName, err)
				// It's important that we know which resource actually had the problem - the approach here is to just try each member of the chunk individually. Can add more complex "binary chunking" later, but want to change how the DB package works slightly before that.
				glog.Error("Retrying chunk components individually")
				oneResourceSlice := make([]*db.Resource, 1) // This is to avoid a bunch of allocations of new slices - don't use it other than just to set oneResourceSlice[0] and pass to the db.
				for _, chunkMember := range chunk {
					oneResourceSlice[0] = chunkMember
					_, _, retryErr := db.Insert(oneResourceSlice)
					if retryErr != nil {
						glog.Errorf("Resource %s cannot be inserted: %s", chunkMember.UID, retryErr)
						syncErrors = append(syncErrors, SyncError{
							ResourceUID: chunkMember.UID,
							Message:     retryErr.Error(),
						})
						badResources++
					}
				}

				// TODO Return 400 if it was because of the request and a 500 if it was because we can't talk to redis
			} else {
				// glog.Infof("Successfully inserted %d resources", len(chunk))
				stats.resourcesAdded += len(chunk) - badResources
			}

			chunk = make([]*db.Resource, 0, CHUNK_SIZE)
		}
	}

	// UPDATE resources
	glog.Info("Updating ", len(syncEvent.UpdateResources), " resources.")
	// Don't need to reset chunk here as it was taken care of by either the last thing that was Added, or the actual declaration of the chunk slice (if Add didn't have anything to run).
	for i := range syncEvent.UpdateResources {

		if syncEvent.UpdateResources[i].Properties != nil {
			syncEvent.UpdateResources[i].Properties["cluster"] = clusterName

			chunk = append(chunk, &syncEvent.UpdateResources[i])
		} else { // This really shouldn't happen, it's suspected this happens because of a collector bug. Adding this for now while we figure that out.{
			glog.Warningf("Skipping resource with UID %s on cluster %s because of nil properties", syncEvent.UpdateResources[i].UID, clusterName)
		}

		// Commiting small chunks isn't the best for performance, but it's easier to debug problems and it
		// helps to prevent total failure in case we get a bad record. Will increase as we solidify our logic.
		if (i != 0 && (i+1)%CHUNK_SIZE == 0) || i == len(syncEvent.UpdateResources)-1 {
			badResources := 0

			_, _, err := db.Update(chunk)
			if err != nil {
				glog.Errorf("Error updating resources for cluster %s: %s", clusterName, err)
				// It's important that we know which resource actually had the problem - the approach here is to just try each member of the chunk individually. Can add more complex "binary chunking" later, but want to change how the DB package works slightly before that.
				glog.Error("Retrying chunk components individually")
				oneResourceSlice := make([]*db.Resource, 1) // This is to avoid a bunch of allocations of new slices - don't use it other than just to set oneResourceSlice[0] and pass to the db.
				for _, chunkMember := range chunk {
					oneResourceSlice[0] = chunkMember
					_, _, retryErr := db.Update(oneResourceSlice)
					if retryErr != nil {
						glog.Errorf("Resource %s cannot be updated: %s", chunkMember.UID, retryErr)
						syncErrors = append(syncErrors, SyncError{
							ResourceUID: chunkMember.UID,
							Message:     retryErr.Error(),
						})
						badResources++
					}
				}
				// TODO Return 400 if it was because of the request and a 500 if it was because we can't talk to redis
			} else {
				stats.resourcesUpdated += len(chunk) - badResources
			}

			chunk = make([]*db.Resource, 0, CHUNK_SIZE)
		}
	}

	// DELETE resources
	// This is a bit different because syncEvent.DeleteResources is a different type to the other two, see the declaration above.
	glog.Info("Deleting ", len(syncEvent.DeleteResources), " resources.")
	deleteChunk := make([]string, 0, CHUNK_SIZE) // Different chunk for deletions because it's just the UIDs
	// Don't need to reset chunk here as it was taken care of by either the last thing that was Added, or the actual declaration of the chunk slice (if Add didn't have anything to run).
	for i, deleteEvent := range syncEvent.DeleteResources {
		deleteChunk = append(deleteChunk, deleteEvent.UID)

		// Commiting small chunks isn't the best for performance, but it's easier to debug problems and it
		// helps to prevent total failure in case we get a bad record. Will increase as we solidify our logic.
		if (i != 0 && (i+1)%CHUNK_SIZE == 0) || i == len(syncEvent.DeleteResources)-1 {
			badResources := 0
			_, err := db.Delete(deleteChunk)
			if err != nil {
				glog.Errorf("Error deleting resources for cluster %s: %s", clusterName, err)
				// It's important that we know which resource actually had the problem - the approach here is to just try each member of the chunk individually. Can add more complex "binary chunking" later, but want to change how the DB package works slightly before that.
				glog.Error("Retrying chunk components individually")
				oneResourceSlice := make([]string, 1) // This is to avoid a bunch of allocations of new slices - don't use it other than just to set oneResourceSlice[0] and pass to the db.
				for _, chunkMember := range deleteChunk {
					oneResourceSlice[0] = chunkMember
					_, retryErr := db.Delete(oneResourceSlice)
					if retryErr != nil {
						glog.Errorf("Resource %s cannot be deleted: %s", chunkMember, retryErr)
						syncErrors = append(syncErrors, SyncError{
							ResourceUID: chunkMember,
							Message:     retryErr.Error(),
						})
						badResources++
					}
				}
				// TODO Return 400 if it was because of the request and a 500 if it was because we can't talk to redis
			} else {
				stats.resourcesDeleted += len(deleteChunk) - badResources
			}

			deleteChunk = make([]string, CHUNK_SIZE) // Reset chunk
		}
	}

	glog.Infof("Done commiting changes for cluster %s, preparing response", clusterName)

	// Updating cluster status in cache.
	updatedTimestamp := time.Now()
	totalResources, currentHash := computeHash(clusterName) // This goes out to the DB through a work order, so it can take a second
	status := db.ClusterStatus{
		Hash:           currentHash,
		LastUpdated:    updatedTimestamp.String(),
		TotalResources: totalResources,
	}

	_, clusterNameErr, err := db.SaveClusterStatus(clusterName, status)
	if clusterNameErr != nil {
		glog.Errorf("Could not save cluster status because of invalid cluster name: %s", clusterName)
	}
	if err != nil {
		glog.Errorf("Failed to save cluster status for cluster %s: %s", clusterName, err)
		// TODO return 500
	}

	glog.Info("Sync Completed for Cluster ", clusterName, ": ", stats)

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
