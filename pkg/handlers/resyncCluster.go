/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

func resyncCluster(clusterName string, resources []*db.Resource, edges []db.Edge, metrics *SyncMetrics) (stats SyncResponse, err error) {
	startx := time.Now()
	glog.Info("Resync for cluster: ", clusterName)

	// First get the existing resources from the datastore for the cluster
	result, error := db.Store.Query(fmt.Sprintf("MATCH (n) WHERE n.cluster='%s' RETURN n", clusterName))

	if error != nil {
		glog.Error("Error getting existing resources for cluster ", clusterName)
		err = error // For return value.
	}

	glog.V(4).Infof("Total existing resources on RedisGraph for cluster %s, %d", clusterName, len(result.Results)-1)

	// Build a map to find the index of a given property from the RedisGraph results.
	var indexOfProp = make(map[string]int)
	for index, prop := range result.Results[0] {
		propName := strings.Split(prop, ".")[1]
		indexOfProp[propName] = index
	}

	// Build a map with all the current resources by UID.
	// Build a map of duplicated resources.
	var existingResources = make(map[string][]string)
	var duplicatedResources = make(map[string]int)
	for index, item := range result.Results {
		if index != 0 { // Skip the first item because it contains the headers, not a real resource.
			existingResourceUID := item[indexOfProp["_uid"]]
			if _, exists := existingResources[existingResourceUID]; exists {
				dupeCount, dupeExists := duplicatedResources[existingResourceUID]
				if !dupeExists {
					duplicatedResources[existingResourceUID] = 1
				} else {
					duplicatedResources[existingResourceUID] = dupeCount + 1
				}
			} else {
				existingResources[existingResourceUID] = item
			}
		}
	}

	// Delete duplicated records. We have to delete all records with the duplicated UID and recreate.
	if len(duplicatedResources) > 0 {
		glog.Warningf("RedisGraph contains duplicate records for some UIDs in cluster %s. Total uids duplicates: %d", clusterName, len(duplicatedResources))
		for dupeUID, dupeCount := range duplicatedResources {
			_, delError := db.Store.Query(fmt.Sprintf("MATCH (n) WHERE n._uid='%s' DELETE n", dupeUID))
			if delError != nil {
				glog.Error("Error deleting duplicates for ", dupeUID, delError)
			}
			glog.V(3).Infof("Deleted %d duplicates of UID %s", dupeCount, dupeUID)
			delete(existingResources, dupeUID) // Delete from existing resources.
		}
	}

	// Loop through incoming resources and check if each resource exist and if it needs to be updated.
	var resourcesToAdd = make([]*db.Resource, 0)
	var resourcesToUpdate = make([]*db.Resource, 0)
	for _, newResource := range resources {
		existingResource, exist := existingResources[newResource.UID]

		if !exist {
			// Resource needs to be added.
			resourcesToAdd = append(resourcesToAdd, newResource)
		} else {
			// Resource exists, but we need to check if it needs to be updated.
			newEncodedProperties, encodeError := newResource.EncodeProperties()
			if encodeError != nil {
				// Assume we need to update this resource if we hit an encoding error.
				glog.Warning("Error encoding properties of resource. ", encodeError)
				resourcesToUpdate = append(resourcesToUpdate, newResource)
			} else {
				for key, value := range newEncodedProperties {
					// Need to compare everything as strings because that's what we get from RedisGraph.
					var stringValue string
					switch typedVal := value.(type) {
					case int64:
						stringValue = strconv.FormatInt(typedVal, 10)
					default:
						stringValue = typedVal.(string)
					}

					if existingResource[indexOfProp[key]] != stringValue {
						resourcesToUpdate = append(resourcesToUpdate, newResource)
						break
					}
				}
			}
			// Remove the resource because it has been proccessed. Any resources remaining when we are done will need to be deleted.
			delete(existingResources, newResource.UID)
		}
	}

	// INSERT Resources

	metrics.NodeSyncStart = time.Now()
	insertResponse := db.ChunkedInsert(resourcesToAdd, clusterName)
	stats.TotalAdded = insertResponse.SuccessfulResources // could be 0
	if insertResponse.ConnectionError != nil {
		err = insertResponse.ConnectionError
	} else if len(insertResponse.ResourceErrors) != 0 {
		stats.AddErrors = processSyncErrors(insertResponse.ResourceErrors, "inserted")
	}

	// UPDATE Resources

	updateResponse := db.ChunkedUpdate(resourcesToUpdate)
	stats.TotalUpdated = updateResponse.SuccessfulResources // could be 0
	if updateResponse.ConnectionError != nil {
		err = updateResponse.ConnectionError
	} else if len(updateResponse.ResourceErrors) != 0 {
		stats.UpdateErrors = processSyncErrors(updateResponse.ResourceErrors, "updated")
	}

	// DELETE Resources

	deleteUIDS := make([]string, 0, len(existingResources))
	for _, resource := range existingResources {
		deleteUIDS = append(deleteUIDS, resource[indexOfProp["_uid"]])
	}
	deleteResponse := db.ChunkedDelete(deleteUIDS)
	stats.TotalDeleted = deleteResponse.SuccessfulResources // could be 0
	if deleteResponse.ConnectionError != nil {
		err = deleteResponse.ConnectionError
	} else if len(deleteResponse.ResourceErrors) != 0 {
		stats.DeleteErrors = processSyncErrors(deleteResponse.ResourceErrors, "deleted")
	}

	metrics.NodeSyncEnd = time.Now()

	// RE-SYNC Edges

	metrics.EdgeSyncStart = time.Now()
	currEdges, edgesError := db.Store.Query(fmt.Sprintf("MATCH (s)-[r]->(d) WHERE s.cluster='%s' AND d.cluster='%s' RETURN s._uid, type(r), d._uid", clusterName, clusterName))
	if edgesError != nil {
		glog.Warning("Error getting all existing edges for cluster ", clusterName, edgesError)
		err = edgesError
	}

	var existingEdges = make(map[string]db.Edge)
	var edgesToAdd = make([]db.Edge, 0)

	// Create a map with the existing edges.
	for index, e := range currEdges.Results {
		if index != 0 {
			existingEdges[fmt.Sprintf("%s-%s->%s", e[0], e[1], e[2])] = db.Edge{SourceUID: e[0], EdgeType: e[1], DestUID: e[2]}
		}
	}

	// Loop through incoming new edges and decide if each edge needs to be added.
	for _, e := range edges {
		if _, exists := existingEdges[fmt.Sprintf("%s-%s->%s", e.SourceUID, e.EdgeType, e.DestUID)]; exists {
			delete(existingEdges, fmt.Sprintf("%s-%s->%s", e.SourceUID, e.EdgeType, e.DestUID))
		} else {
			edgesToAdd = append(edgesToAdd, e)
		}
	}

	// Compute edges to delete. These are the remaining objects in existingEdges after processing all the incoming new edges.
	var edgesToDelete = make([]db.Edge, 0)
	for _, e := range existingEdges {
		edgesToDelete = append(edgesToDelete, e)
	}

	// INSERT Edges
	insertEdgeResponse := db.ChunkedInsertEdge(edgesToAdd)
	stats.TotalEdgesAdded = insertEdgeResponse.SuccessfulResources // could be 0
	if insertEdgeResponse.ConnectionError != nil {
		err = insertEdgeResponse.ConnectionError
	} else if len(insertEdgeResponse.ResourceErrors) != 0 {
		stats.AddEdgeErrors = processSyncErrors(insertEdgeResponse.ResourceErrors, "inserted by edge")
	}

	// DELETE Edges
	deleteEdgeResponse := db.ChunkedDeleteEdge(edgesToDelete)
	stats.TotalEdgesDeleted = deleteEdgeResponse.SuccessfulResources // could be 0
	if deleteEdgeResponse.ConnectionError != nil {
		err = deleteEdgeResponse.ConnectionError
	} else if len(deleteEdgeResponse.ResourceErrors) != 0 {
		stats.DeleteEdgeErrors = processSyncErrors(deleteEdgeResponse.ResourceErrors, "removed by edge")
	}

	// There's no need to UPDATE edges because edges don't have properties yet.
	elapsedx := time.Since(startx)
	glog.Infof("Cluster %s resync time %d", clusterName, elapsedx/1000)
	metrics.EdgeSyncEnd = time.Now()

	return stats, err
}
