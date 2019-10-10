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
)

// Recursive helper for ChunkedDelete. Takes a single chunk, and recursively attempts to insert that chunk, then the first and second halves of that chunk independently, and so on.
func chunkedDeleteHelper(uids []string) ChunkedOperationResult {
	if len(uids) == 0 {
		return ChunkedOperationResult{} // No errors, and no SuccessfulResources
	}
	_, err := Delete(uids)
	if IsBadConnection(err) { // this is false if err is nil
		return ChunkedOperationResult{
			ConnectionError: err,
		}
	}
	if err != nil {
		if len(uids) == 1 { // If this was a single resource
			return ChunkedOperationResult{
				ResourceErrors: map[string]error{uids[0]: err},
			}
		} else { // If this is multiple resources, we make a recursive call to find which half had the error.
			firstHalf := chunkedDeleteHelper(uids[0 : len(uids)/2])
			secondHalf := chunkedDeleteHelper(uids[len(uids)/2:])
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
		SuccessfulResources: len(uids),
	}
}

// Deletes the given resources from the graph, does chunking for you and returns errors related to individual resources.
func ChunkedDelete(resources []string) ChunkedOperationResult {
	var resourceErrors map[string]error
	totalSuccessful := 0
	for i := 0; i < len(resources); i += CHUNK_SIZE {
		endIndex := min(i+CHUNK_SIZE, len(resources))
		chunkResult := chunkedDeleteHelper(resources[i:endIndex])
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

// Deletes resources with the given UIDs, transparently builds query for you and returns the reponse and errors given by redisgraph.
// No encoding errors possible with this operation.
func Delete(uids []string) (QueryResult, error) {
	query := deleteQuery(uids)
	resp, err := Store.Query(query)
	return resp, err
}

// This has the error in the returns so that it matches the interface, it isn't used. Thought that was nicer than having 2 interfaces.
func deleteQuery(uids []string) string {
	if len(uids) == 0 {
		return ""
	}

	clauseStrings := []string{} // Build the clauses to filter down to only the ones we want.
	for _, uid := range uids {
		clauseStrings = append(clauseStrings, fmt.Sprintf("n._uid='%s'", uid))
	}

	queryString := fmt.Sprintf("MATCH (n) WHERE (%s) DELETE n", strings.Join(clauseStrings, " OR ")) // e.g. MATCH (n) WHERE (n._uid='uid1' OR n._uid='uid2') DELETE n

	return queryString
}
