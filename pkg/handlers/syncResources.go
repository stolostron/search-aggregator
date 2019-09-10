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
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/config"
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
	RequestId   int
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
	Version           string
	RequestId         int
}

// SyncError is used to respond with errors.
type SyncError struct {
	ResourceUID string
	Message     string // Often comes out of a golang error using .Error()
}

// SyncResources - Process Add, Update, and Delete events.
func SyncResources(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	glog.V(2).Info("SyncResources() for cluster: ", clusterName)
	interClusterUpdated := false                // flag to decide the time when last suscription was changed
	subscriptionUIDMap := make(map[string]bool) // map to hold exisiting subscription uids

	response := SyncResponse{Version: config.AGGREGATOR_API_VERSION}

	// Function that sends the current response and the given status code.
	// If you want to bail out early, make sure to call return right after.
	respond := func(status int) {
		statusMessage := fmt.Sprintf(
			"Responding to cluster %s with requestId %d, status %d, stats: {Added: %d, Updated: %d, Deleted: %d, Edges Added: %d, Edges Deleted: %d, Total Resources: %d, Total Edges: %d}",
			clusterName,
			response.RequestId,
			status,
			response.TotalAdded,
			response.TotalUpdated,
			response.TotalDeleted,
			response.TotalEdgesAdded,
			response.TotalEdgesDeleted,
			response.TotalResources,
			response.TotalEdges,
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
	response.RequestId = syncEvent.RequestId
	glog.V(3).Infof("Processing Request with { request: %d, add: %d, update: %d, delete: %d edge add: %d edge delete: %d }", syncEvent.RequestId, len(syncEvent.AddResources), len(syncEvent.UpdateResources), len(syncEvent.DeleteResources), len(syncEvent.AddEdges), len(syncEvent.DeleteEdges))

	err = db.ValidateClusterName(clusterName)
	if err != nil {
		glog.Warning("Invalid Cluster Name: ", clusterName)
		respond(http.StatusBadRequest)
		return
	}

	// Validate that we have a Cluster CRD so we can build edges on create
	if !assertClusterNode(clusterName) {
		glog.Warningf("Warning, couldn't to find a Cluster resource with name: %s. This means that the sync request came from a remote cluster that hasn’t joined MCM. Rejecting the incoming sync request.", clusterName)
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
	// let us store the Current Subscription Uids in a map [String] -> boolean
	uidresults, uiderr := getUIDsForSubscriptions()
	if uiderr == nil {
		if len(uidresults.Results) > 1 {
			for _, uid := range uidresults.Results[1:] {
				subscriptionUIDMap[uid[0]] = true
			}
		}

	} else {
		glog.Warningf("Error Fetching Subscriptions %s", uiderr)
	}
	glog.V(3).Infof("Current Subscriptions found %d", len(subscriptionUIDMap))

	// INSERT Resources

	// add cluster fields
	for i := range syncEvent.AddResources {
		syncEvent.AddResources[i].Properties["cluster"] = clusterName
	}

	insertResponse := db.ChunkedInsert(syncEvent.AddResources, clusterName)
	response.TotalAdded = insertResponse.SuccessfulResources // could be 0
	if insertResponse.ConnectionError != nil {
		respond(http.StatusServiceUnavailable)
		return
	} else if len(insertResponse.ResourceErrors) != 0 {
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
	} else if len(updateResponse.ResourceErrors) != 0 {
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
	} else if len(deleteResponse.ResourceErrors) != 0 {
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
	} else if len(insertEdgeResponse.ResourceErrors) != 0 {
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
	} else if len(deleteEdgeResponse.ResourceErrors) != 0 {
		response.DeleteEdgeErrors = processSyncErrors(deleteEdgeResponse.ResourceErrors, "removed by edge")
		respond(http.StatusBadRequest)
		return
	}

	glog.V(2).Infof("Done updating resources for cluster %s, preparing response", clusterName)
	response.TotalResources = computeNodeCount(clusterName) // This goes out to the DB through a work order, so it can take a second
	response.TotalEdges = computeIntraEdges(clusterName)
	elapsed := time.Since(start)
	if int(elapsed.Seconds()) > 5 {
		glog.Warningf("SyncResources from %s took %s", clusterName, elapsed)
		glog.Warningf("Increased Processing time with { request: %d, add: %d, update: %d, delete: %d edge add: %d edge delete: %d }", syncEvent.RequestId, len(syncEvent.AddResources), len(syncEvent.UpdateResources), len(syncEvent.DeleteResources), len(syncEvent.AddEdges), len(syncEvent.DeleteEdges))
	} else {
		glog.V(4).Infof("SyncResources from %s took %s", clusterName, elapsed)
	}
	respond(http.StatusOK)

	// update the timestamp if we made any changes Kind = Subscription OR
	// An Edge which connect from/to a Node (Kind = Subscription)

	// if any Node with kind Subscription Added then interClusterUpdated
	for i := range syncEvent.AddResources {
		if syncEvent.AddResources[i].Properties["kind"] == "Subscription" || syncEvent.AddResources[i].Properties["kind"] == "Application" {
			glog.V(3).Infof("Will trigger Intercluster - Added  Node %s ", syncEvent.AddResources[i].Properties["name"])
			interClusterUpdated = true
			break
		}

	}
	// if interClusterUpdated == false check any updates to Node with Kind = Subscription
	if !interClusterUpdated {
		for i := range syncEvent.UpdateResources {
			if syncEvent.UpdateResources[i].Properties["kind"] == "Subscription" || syncEvent.UpdateResources[i].Properties["kind"] == "Application" {
				glog.V(3).Infof("Will trigger Intercluster - Updated  Node %s ", syncEvent.UpdateResources[i].Properties["name"])
				interClusterUpdated = true
				break
			}

		}

	}

	// if interClusterUpdated == false check Updated and Deleted edges
	if !interClusterUpdated {
		// Check Added Edges - Do they have a UID  which is present in  Subscriptions Map we created
		for i := range syncEvent.AddEdges {
			if subscriptionUIDMap[syncEvent.AddEdges[i].SourceUID] || subscriptionUIDMap[syncEvent.AddEdges[i].DestUID] {
				glog.V(3).Infof("Will trigger Intercluster Added Edge %s -> %s ", syncEvent.AddEdges[i].SourceUID, syncEvent.AddEdges[i].DestUID)
				interClusterUpdated = true
				break
			}

		}
		//if we have interClusterUpdated = false , check for the deleted edges if they had connection to Kind = Subscription
		if !interClusterUpdated {
			for i := range syncEvent.DeleteEdges {
				if subscriptionUIDMap[syncEvent.DeleteEdges[i].SourceUID] || subscriptionUIDMap[syncEvent.DeleteEdges[i].DestUID] {
					glog.V(3).Infof("Will trigger Intercluster Deleted Edge %s -> %s ", syncEvent.DeleteEdges[i].SourceUID, syncEvent.DeleteEdges[i].DestUID)
					interClusterUpdated = true
					break
				}

			}

		}

	}

	if interClusterUpdated {
		LastUpdated = time.Now()
	}
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
