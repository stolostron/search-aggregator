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

	"github.com/golang/glog"
	rg "github.com/redislabs/redisgraph-go"
)

// Recursive helper for ChunkedInsert. Takes a single chunk, and recursively attempts to insert that chunk, then the first and second halves of that chunk independently, and so on.
func chunkedInsertHelper(resources []*Resource) ChunkedOperationResult {

	if len(resources) == 0 {
		return ChunkedOperationResult{} // No errors, and no SuccessfulResources
	}

	_, _, err := Insert(resources) // We currently ignore encoding errors as they are always recoverable, may change in the future.
	if IsBadConnection(err) {      // this is false if err is nil
		return ChunkedOperationResult{
			ConnectionError: err,
		}
	}

	if err != nil {
		if len(resources) == 1 { // If this was a single resource
			glog.Warningf("Rejecting Resource %s: %s", resources[0].UID, err)
			return ChunkedOperationResult{
				ResourceErrors: map[string]error{resources[0].UID: err},
			}
		} else { // If this is multiple resources, we make a recursive call to find which half had the error.
			firstHalf := chunkedInsertHelper(resources[0 : len(resources)/2])
			secondHalf := chunkedInsertHelper(resources[len(resources)/2:])
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

// Inserts the given resources into the graph, does chunking for you and returns errors related to individual resources.
func ChunkedInsert(resources []*Resource) ChunkedOperationResult {
	var resourceErrors map[string]error
	totalSuccessful := 0
	for i := 0; i < len(resources); i += CHUNK_SIZE {
		endIndex := min(i+CHUNK_SIZE, len(resources))
		chunkResult := chunkedInsertHelper(resources[i:endIndex])
		if chunkResult.ConnectionError != nil {
			return chunkResult
		} else if chunkResult.ResourceErrors != nil {
			resourceErrors = mergeErrorMaps(resourceErrors, chunkResult.ResourceErrors) // if both are nil, this is still nil.
		}
		totalSuccessful += chunkResult.SuccessfulResources
	}
	ret := ChunkedOperationResult{
		ResourceErrors:      resourceErrors,
		SuccessfulResources: totalSuccessful,
	}
	return ret
}

// Inserts given resources into graph, transparently builds query for you and returns the reponse and errors given by redisgraph.
// Returns the result, any errors when encoding, and any error from the query itself.
func Insert(resources []*Resource) (rg.QueryResult, map[string]error, error) {
	query, encodingErrors := insertQuery(resources) // Encoding errors are recoverable, but we still report them
	resp, err := Query(query)
	return resp, encodingErrors, err
}

// Given a set of Resources, returns Query for inserting them into redisgraph.
func insertQuery(resources []*Resource) (string, map[string]error) {

	if len(resources) == 0 {
		return "", nil
	}

	encodingErrors := make(map[string]error)

	resourceStrings := []string{} // Build the query string piece by piece.
	for _, resource := range resources {
		resource.addRbacProperty()
		encodedProps, err := resource.encodeProperties()
		if err != nil {
			glog.Error("Cannot encode resource ", resource.UID, ", excluding it from insertion: ", err)
			encodingErrors[resource.UID] = err
			continue
		}
		propStrings := []string{}
		for k, v := range encodedProps {
			switch typed := v.(type) { // At this point it's either string or int64. Need to wrap in quotes if it's string
			case int64:
				propStrings = append(propStrings, fmt.Sprintf("%s:%d", k, typed)) // e.g. key>:<value>
			default:
				propStrings = append(propStrings, fmt.Sprintf("%s:'%s'", k, typed)) // e.g. <key>:'<value>'
			}
		}
		resourceStrings = append(resourceStrings, fmt.Sprintf("(:%s {_uid:'%s', %s})", resource.Properties["kind"], resource.UID, strings.Join(propStrings, ", "))) // e.g. (:Pod {_uid: 'abc123', prop1:5, prop2:'cheese'})
	}

	queryString := fmt.Sprintf("%s %s", "CREATE", strings.Join(resourceStrings, ", ")) // e.g. CREATE (:Pod {_uid: 'abc123', prop1:5, prop2:'cheese'}), (:Pod {_uid: 'def456', prop1:4, prop2:'water'})

	return queryString, encodingErrors
}