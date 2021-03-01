/*
 * (C) Copyright IBM Corporation 2019 All Rights Reserved
 * Copyright (c) 2020 Red Hat, Inc.
 * Copyright Contributors to the Open Cluster Management project
*/

package dbconnector

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"
	rg2 "github.com/redislabs/redisgraph-go"
)

// Inserts the given edges grouped by source
func ChunkedInsertEdge(resources []Edge, clusterName string) ChunkedOperationResult {
	glog.V(4).Info("For cluster ", clusterName, ": Number of edges received in ChunkedInsertEdge: ", len(resources))
	var insertEdgeCount int
	if len(resources) == 0 {
		return ChunkedOperationResult{
			ResourceErrors:      nil,
			SuccessfulResources: 0,
		}
	}

	// sort our slice addessending by combination source/type to build efficient queries
	sort.Slice(resources, func(i, j int) bool {
		// https://stackoverflow.com/questions/4576714/sort-by-two-values-prioritizing-on-one-of-them
		return resources[i].SourceUID < resources[j].SourceUID || resources[i].EdgeType < resources[j].EdgeType
	})

	// status to return in ChunkedOperationResult
	resourceErrors := make(map[string]error)
	totalAdded := 0
	currentLength := 0

	newWhereClause := true
	var whereClause strings.Builder
	for i := range resources {
		newWhereClause = true
		// add dest uid for each node in the group to where clause
		if whereClause.Len() == 0 {
			fmt.Fprint(&whereClause, SanitizeQuery("WHERE d._uid='%s'", resources[i].DestUID))
		} else {
			fmt.Fprint(&whereClause, SanitizeQuery(" OR d._uid='%s'", resources[i].DestUID))
		}

		currentLength++

		//look ahead to see if we are in a differnet group or if at max chuck size
		if currentLength == CHUNK_SIZE || (i < len(resources)-1 &&
			(resources[i+1].SourceUID != resources[i].SourceUID || resources[i+1].EdgeType != resources[i].EdgeType)) {
			resp, err := insertEdge(resources[i], whereClause.String())
			newWhereClause = false
			if err != nil {
				// saving JUST the source as the key to the map
				resourceErrors[resources[i].SourceUID] = err
			} else {
				totalAdded += currentLength
				insertEdgeCount += resp.RelationshipsCreated()
			}
			whereClause.Reset()
			currentLength = 0
		}
	}

	if newWhereClause {
		// commit the last edge string to the db
		resp, err := insertEdge(resources[len(resources)-1], whereClause.String())
		if err != nil {
			// saving JUST the source as the key to the map
			resourceErrors[resources[len(resources)-1].SourceUID] = err
		} else {
			totalAdded += currentLength
			insertEdgeCount += resp.RelationshipsCreated()
		}
	}
	glog.V(4).Info("ChunkedInsertEdge: For cluster, ", clusterName, ": Number of edges inserted: ", insertEdgeCount)

	return ChunkedOperationResult{
		ResourceErrors:      resourceErrors,
		SuccessfulResources: totalAdded,
		EdgesAdded:          insertEdgeCount,
	}
}

// e.g. MATCH (s:{_uid:'abc'}), (d) WHERE d._uid='def' OR d._uid='ghi' CREATE (s)-[:Type]>(d)
func insertEdge(edge Edge, whereClause string) (*rg2.QueryResult, error) {
	//This is the basic insert query without using node labels
	query := fmt.Sprintf("MATCH (s {_uid: '%s'}), (d) %s CREATE (s)-[:%s]->(d)",
		edge.SourceUID, whereClause, edge.EdgeType)

	// If OR d_uid= is present in whereClause, multiple edges are inserted. So, filter by destKind label cannot be used
	if strings.Contains(whereClause, " OR d._uid=") {
		if edge.SourceKind != "" {
			query = fmt.Sprintf("MATCH (s:%s {_uid: '%s'}), (d) %s CREATE (s)-[:%s]->(d)",
				edge.SourceKind, edge.SourceUID, whereClause, edge.EdgeType)
		}
	} else { //insert only single edge
		//Insert with node labels if only one edge is inserted at a time.
		if edge.SourceKind != "" && edge.DestKind != "" { // check if both source and dest labels are present
			query = fmt.Sprintf("MATCH (s:%s {_uid: '%s'}), (d:%s) %s CREATE (s)-[:%s]->(d)",
				edge.SourceKind, edge.SourceUID, edge.DestKind, whereClause, edge.EdgeType)
		}
	}
	glog.V(4).Info("Insert query: ", query)
	resp, err := Store.Query(query)
	if err == nil {
		glog.V(4).Info("Relationships created: ", resp.RelationshipsCreated())
	}
	return resp, err
}
