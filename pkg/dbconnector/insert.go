/*
 * (C) Copyright IBM Corporation 2019 All Rights Reserved
 * Copyright (c) 2020 Red Hat, Inc.
 * Copyright Contributors to the Open Cluster Management project
 */

package dbconnector

import (
	"fmt"
	"strings"
	"sync"

	"github.com/golang/glog"
	rg2 "github.com/redislabs/redisgraph-go"
)

// Recursive helper for ChunkedInsert. Takes a single chunk, and recursively attempts to insert that chunk,
// then the first and second halves of that chunk independently, and so on.
func chunkedInsertHelper(resources []*Resource, clusterName string) ChunkedOperationResult {

	if len(resources) == 0 {
		return ChunkedOperationResult{} // No errors, and no SuccessfulResources
	}

	_, _, err := Insert(resources, clusterName) // We ignore encoding errors as they are always recoverable.
	if IsBadConnection(err) {                   // this is false if err is nil
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
			firstHalf := chunkedInsertHelper(resources[0:len(resources)/2], clusterName)
			secondHalf := chunkedInsertHelper(resources[len(resources)/2:], clusterName)
			if firstHalf.ConnectionError != nil || secondHalf.ConnectionError != nil {
				// Again, if either one has a redis conn issue we just instantly bail
				return ChunkedOperationResult{
					ConnectionError: err,
				}
			}
			return ChunkedOperationResult{
				ResourceErrors: mergeErrorMaps(firstHalf.ResourceErrors, secondHalf.ResourceErrors),
				// These will be 0 if there were errs in the halves
				SuccessfulResources: firstHalf.SuccessfulResources + secondHalf.SuccessfulResources,
			}
		}
	}
	// All clear, return that we got everything in
	return ChunkedOperationResult{
		SuccessfulResources: len(resources),
	}
}

// Insert the given resources into the graph, does chunking for you and returns errors related to individual resources.
func ChunkedInsert(resources []*Resource, clusterName string) ChunkedOperationResult {
	var resourceErrors map[string]error
	totalSuccessful := 0
	var ExistingIndexMapMutex = sync.RWMutex{}

	kindMap := make(map[string]struct{})
	for _, res := range resources {
		kindMap[res.Properties["kind"].(string)] = struct{}{}
	}

	for i := 0; i < len(resources); i += CHUNK_SIZE {
		endIndex := min(i+CHUNK_SIZE, len(resources))
		chunkResult := chunkedInsertHelper(resources[i:endIndex], clusterName)
		if chunkResult.ConnectionError != nil {
			return chunkResult
		} else if chunkResult.ResourceErrors != nil {
			resourceErrors = mergeErrorMaps(resourceErrors, chunkResult.ResourceErrors) // if both are nil, this is nil
		}
		totalSuccessful += chunkResult.SuccessfulResources
	}
	ret := ChunkedOperationResult{
		ResourceErrors:      resourceErrors,
		SuccessfulResources: totalSuccessful,
	}
	for kind := range kindMap {
		ExistingIndexMapMutex.RLock()
		exists := ExistingIndexMap[kind]
		ExistingIndexMapMutex.RUnlock()
		if !exists {
			insertErr := insertIndex(kind, "_uid")
			if insertErr == nil {
				ExistingIndexMapMutex.Lock() // Lock map before writing
				ExistingIndexMap[kind] = true
				ExistingIndexMapMutex.Unlock() // Unlock map after writing
			}
		}
	}
	return ret
}

// Inserts given resources into graph, transparently builds query for you and
// returns the response and errors given by redisgraph.
// Returns the result, any errors when encoding, and any error from the query itself.
func Insert(resources []*Resource, clusterName string) (*rg2.QueryResult, map[string]error, error) {
	query, encodingErrors := insertQuery(resources, clusterName) // Encoding errors are recoverable, but we report them
	resp, err := Store.Query(query)
	return resp, encodingErrors, err
}

// Given a set of Resources, returns Query for inserting them into redisgraph.
func insertQuery(resources []*Resource, clusterName string) (string, map[string]error) {

	if len(resources) == 0 {
		return "", nil
	}

	encodingErrors := make(map[string]error)

	resourceStrings := []string{} // Build the query string piece by piece.
	for _, resource := range resources {
		resource.addRbacProperty()
		encodedProps, err := resource.EncodeProperties()
		if err != nil {
			glog.Error("Cannot encode resource ", resource.UID, ", excluding it from insertion: ", err)
			encodingErrors[resource.UID] = err
			continue
		}
		propStrings := []string{}
		for k, v := range encodedProps {
			switch typed := v.(type) { // This is either string or int64 with base type string or []interface
			//Need to wrap in quotes if it's string
			case int64:
				propStrings = append(propStrings, fmt.Sprintf("%s:%d", k, typed)) // e.g. key>:<value>
			case []interface{}, map[string]interface{}: // Values are individually quoted already in encodeProperty
				propStrings = append(propStrings, fmt.Sprintf("%s:%s", k, typed)) // e.g. <key>:<value>
			default:
				propStrings = append(propStrings, fmt.Sprintf("%s:'%s'", k, typed)) // e.g. <key>:'<value>'
			}
		}
		// e.g. (:Pod {_uid: 'abc123', prop1:5, prop2:'cheese'})
		resource := fmt.Sprintf("(:%s {_uid:'%s', %s})",
			resource.Properties["kind"], resource.UID, strings.Join(propStrings, ", "))

		// if a clusterName was passed in then we should connect the resource to the cluster node
		if clusterName != "" {
			resource += "-[:inCluster {_interCluster: true}]->(c)"
		}

		resourceStrings = append(resourceStrings, resource)
	}

	// e.g. CREATE (:Pod {_uid: 'abc123', prop1:5, prop2:'cheese'}), (:Pod {_uid: 'def456', prop1:4, prop2:'water'})
	queryString := fmt.Sprintf("%s %s", "CREATE", strings.Join(resourceStrings, ", "))

	// need to match the cluster node so we can reference it
	if clusterName != "" {
		queryString = SanitizeQuery("MATCH (c:Cluster {name: '%s'})", clusterName) + queryString
	}

	return queryString, encodingErrors
}
