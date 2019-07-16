/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"fmt"
	"strings"

	rg "github.com/redislabs/redisgraph-go"
)

// Recursive helper for InsertEdge. Takes a single chunk, and recursively attempts to insert that chunk, then the first and second halves of that chunk independently, and so on.
func chunkedInsertEdgeHelper(resources []Edge) ChunkedOperationResult {
	if len(resources) == 0 {
		return ChunkedOperationResult{} // No errors, and no SuccessfulResources
	}
	_, err := InsertEdge(resources) // We currently ignore encoding errors as they are always recoverable, may change in the future.
	if IsBadConnection(err) {       // this is false if err is nil
		return ChunkedOperationResult{
			ConnectionError: err,
		}
	}
	if err != nil {
		if len(resources) == 1 { // If this was a single resource
			uid := fmt.Sprintf("(%s)-[:%s]->(%s)", resources[0].SourceUID, resources[0].EdgeType, resources[0].DestUID)
			return ChunkedOperationResult{
				ResourceErrors: map[string]error{uid: err},
			}
		} else { // If this is multiple resources, we make a recursive call to find which half had the error.
			firstHalf := chunkedInsertEdgeHelper(resources[0 : len(resources)/2])
			secondHalf := chunkedInsertEdgeHelper(resources[len(resources)/2:])
			if firstHalf.ConnectionError != nil || secondHalf.ConnectionError != nil { // Again, if either one has a redis conn issue we just instantly bail
				return ChunkedOperationResult{
					ConnectionError: err,
				}
			}
			return ChunkedOperationResult{
				ResourceErrors:      mergeErrorMaps(firstHalf.ResourceErrors, secondHalf.ResourceErrors),
				SuccessfulResources: firstHalf.SuccessfulResources + secondHalf.SuccessfulResources, // These will be 0 if there were errs in the halves
			}
		}
	}
	// All clear, return that we got everything in
	return ChunkedOperationResult{
		SuccessfulResources: len(resources),
	}
}

// Inserts the given edges in the graph, does chunking for you and returns errors related to individual edges.
func ChunkedInsertEdge(resources []Edge) ChunkedOperationResult {
	var resourceErrors map[string]error
	totalSuccessful := 0
	for i := 0; i < len(resources); i += CHUNK_SIZE {
		endIndex := min(i+CHUNK_SIZE, len(resources))
		chunkResult := chunkedInsertEdgeHelper(resources[i:endIndex])
		if chunkResult.ConnectionError != nil {
			return chunkResult
		} else if chunkResult.ResourceErrors != nil {
			resourceErrors = mergeErrorMaps(resourceErrors, chunkResult.ResourceErrors) // if both are nil, this is still nil.
		}
		totalSuccessful += chunkResult.SuccessfulResources
	}
	return ChunkedOperationResult{
		ResourceErrors:      resourceErrors,
		SuccessfulResources: totalSuccessful,
	}
}

// Returns the result, any errors when encoding, and any error from the query itself.
func InsertEdge(edges []Edge) (rg.QueryResult, error) {
	query := insertEdgeQuery(edges) // Encoding errors are recoverable, but we still report them
	resp, err := Store.Query(query)
	return resp, err
}

// Returns a query used to draw an edge between 2 existing nodes.
// e.g. MATCH (s:{_uid:"abc"}), (d:{_uid:"def"}) CREATE (s)-[:Type]>(d)
func insertEdgeQuery(edges []Edge) string {
	if len(edges) == 0 {
		return ""
	}

	matchStrings := []string{}  // Build the MATCH portion
	createStrings := []string{} // Build the CREATE portion. Declare this at the same time so that we can do this in one pass.
	for i, edge := range edges {
		matchStrings = append(matchStrings, fmt.Sprintf("(s%d {_uid: '%s'}), (d%d {_uid: '%s'})", i, edge.SourceUID, i, edge.DestUID)) // e.g. (s0 {_uid: 'abc123'}), (d0 {_uid: 'abc321'})
		createStrings = append(createStrings, fmt.Sprintf("(s%d)-[:%s]->(d%[1]d)", i, edge.EdgeType))                                  // e.g. (s0)-[:type]->(d0)
	}

	queryString := fmt.Sprintf("%s%s", "MATCH "+strings.Join(matchStrings, ", "), " CREATE "+strings.Join(createStrings, ", "))
	return queryString
}
