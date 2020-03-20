/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"fmt"
	"sort"
	"strings"
)

// Inserts the given edges grouped by source
func ChunkedInsertEdge(resources []Edge) ChunkedOperationResult {
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

	var whereClause strings.Builder
	for i := range resources {
		// add dest uid for each node in the group to where clause
		if whereClause.Len() == 0 {
			fmt.Fprintf(&whereClause, "WHERE d._uid='%s'", resources[i].DestUID)
		} else {
			fmt.Fprintf(&whereClause, " OR d._uid='%s'", resources[i].DestUID)
		}

		currentLength++

		//look ahead to see if we are in a differnet group or if at max chuck size
		if currentLength == CHUNK_SIZE || (i < len(resources)-1 && (resources[i+1].SourceUID != resources[i].SourceUID || resources[i+1].EdgeType != resources[i].EdgeType)) {
			_, err := insertEdge(resources[i], whereClause.String())
			if err != nil {
				// saving JUST the source as the key to the map
				resourceErrors[resources[i].SourceUID] = err
			} else {
				totalAdded += currentLength
			}
			whereClause.Reset()
			currentLength = 0
		}
	}

	// commit the last edge string to the db
	_, err := insertEdge(resources[len(resources)-1], whereClause.String())
	if err != nil {
		// saving JUST the source as the key to the map
		resourceErrors[resources[len(resources)-1].SourceUID] = err
	} else {
		totalAdded += currentLength
	}

	return ChunkedOperationResult{
		ResourceErrors:      resourceErrors,
		SuccessfulResources: totalAdded,
	}
}

// e.g. MATCH (s:{_uid:'abc'}), (d) WHERE d._uid='def' OR d._uid='ghi' CREATE (s)-[:Type]>(d)
func insertEdge(edge Edge, whereClause string) (QueryResult, error) {
	query := ""
	if edge.SourceLabel != "" && edge.DestLabel != "" {
		query = fmt.Sprintf("MATCH (s:%s {_uid: '%s'}), (d:%s) %s CREATE (s)-[:%s]->(d)", edge.SourceLabel, edge.SourceUID, edge.DestLabel, whereClause, edge.EdgeType)
	} else {
		query = fmt.Sprintf("MATCH (s {_uid: '%s'}), (d) %s CREATE (s)-[:%s]->(d)", edge.SourceUID, whereClause, edge.EdgeType)
	}
	//glog.Info(query)
	resp, err := Store.Query(query)
	return resp, err
}
