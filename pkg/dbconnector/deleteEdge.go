/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"fmt"
	"strings"
)

// Recursive helper for DeleteEdge. Takes a single chunk, and recursively attempts to delete that chunk, then the first and second halves of that chunk independently, and so on.
func chunkedDeleteEdgeHelper(resources []Edge) ChunkedOperationResult {
	if len(resources) == 0 {
		return ChunkedOperationResult{} // No errors, and no SuccessfulResources
	}
	_, err := DeleteEdge(resources) // We currently ignore encoding errors as they are always recoverable, may change in the future.
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
			firstHalf := chunkedDeleteEdgeHelper(resources[0 : len(resources)/2])
			secondHalf := chunkedDeleteEdgeHelper(resources[len(resources)/2:])
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

// Updates the given resources in the graph, does chunking for you and returns errors related to individual edges.
func ChunkedDeleteEdge(resources []Edge) ChunkedOperationResult {
	var resourceErrors map[string]error
	totalSuccessful := 0
	for i := 0; i < len(resources); i += CHUNK_SIZE {
		endIndex := min(i+CHUNK_SIZE, len(resources))
		chunkResult := chunkedDeleteEdgeHelper(resources[i:endIndex])
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
func DeleteEdge(edges []Edge) (QueryResult, error) {
	query := deleteEdgeQuery(edges) // Encoding errors are recoverable, but we still report them
	resp, err := Store.Query(query)
	return resp, err
}

// Returns a query used to delete an edge between 2 existing nodes.
// e.g. MATCH (s {_uid: 'abc'})-[e:Type]->(d {_uid: 'def'}) DELETE e
func deleteEdgeQuery(edges []Edge) string {
	if len(edges) == 0 {
		return ""
	}

	matchStrings := []string{}  // Build the MATCH portion
	deleteStrings := []string{} // Build the DELETE portion. Declare this at the same time so that we can do this in one pass.
	for i, edge := range edges {
		// e.g. MATCH (s {_uid: 'abc'})-[e:Type]->(d {_uid: 'def'})
		if edge.SourceKind != "" && edge.DestKind != "" {
			matchStrings = append(matchStrings, SanitizeQuery("(s%d:%[5]s {_uid: '%s'})-[e%[1]d:%[3]s]->(d%[1]d:%[6]s {_uid: '%[4]s'})", i, edge.SourceUID, edge.EdgeType, edge.DestUID, edge.SourceKind, edge.DestKind))
		} else {
			matchStrings = append(matchStrings, SanitizeQuery("(s%d {_uid: '%s'})-[e%[1]d:%[3]s]->(d%[1]d {_uid: '%[4]s'})", i, edge.SourceUID, edge.EdgeType, edge.DestUID))
		}
		deleteStrings = append(deleteStrings, fmt.Sprintf("e%d", i)) // e.g. e0
	}

	/* #nosec G201 - Input sanitized above. */
	queryString := fmt.Sprintf("MATCH %s DELETE %s", strings.Join(matchStrings, ", "), strings.Join(deleteStrings, ", "))

	return queryString
}
