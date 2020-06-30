/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
Copyright (c) 2020 Red Hat, Inc.
*/

package dbconnector

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"
	rg2 "github.com/redislabs/redisgraph-go"
)

// Recursive helper for ChunkedUpdate. Takes a single chunk, and recursively attempts to insert that chunk, then the first and second halves of that chunk independently, and so on.
func chunkedUpdateHelper(resources []*Resource) ChunkedOperationResult {
	if len(resources) == 0 {
		return ChunkedOperationResult{} // No errors, and no SuccessfulResources
	}
	_, _, err := Update(resources) // We currently ignore encoding errors as they are always recoverable, may change in the future.
	if IsBadConnection(err) {      // this is false if err is nil
		return ChunkedOperationResult{
			ConnectionError: err,
		}
	}
	if err != nil {
		if len(resources) == 1 { // If this was a single resource
			return ChunkedOperationResult{
				ResourceErrors: map[string]error{resources[0].UID: err},
			}
		} else { // If this is multiple resources, we make a recursive call to find which half had the error.
			firstHalf := chunkedUpdateHelper(resources[0 : len(resources)/2])
			secondHalf := chunkedUpdateHelper(resources[len(resources)/2:])
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

// Updates the given resources in the graph, does chunking for you and returns errors related to individual resources.
func ChunkedUpdate(resources []*Resource) ChunkedOperationResult {
	var resourceErrors map[string]error
	totalSuccessful := 0
	for i := 0; i < len(resources); i += CHUNK_SIZE {
		endIndex := min(i+CHUNK_SIZE, len(resources))
		chunkResult := chunkedUpdateHelper(resources[i:endIndex])
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

// Updates given resources into graph, transparently builds query for you and returns the reponse and errors given by redisgraph.
// Returns the result, any errors when encoding, and any error from the query itself.
func Update(resources []*Resource) (*rg2.QueryResult, map[string]error, error) {
	query, encodingErrors := updateQuery(resources) // Encoding errors are recoverable, but we still report them
	resp, err := Store.Query(query)
	return resp, encodingErrors, err
}

// Given a set of resources, returns Query string for replacing the existing versions of them in redisgraph with the given ones.
// Will not delete old properties.
func updateQuery(resources []*Resource) (string, map[string]error) {

	if len(resources) == 0 {
		return "", nil
	}
	encodingErrors := make(map[string]error)

	// Form query string with MATCH and SET to update all the resources at once.
	// Useful doc: https://oss.redislabs.com/redisgraph/commands/#set
	matchStrings := []string{} // Build the MATCH portion
	setStrings := []string{}   // Build the SET portion. Declare this at the same time so that we can do this in one pass.
	for i, resource := range resources {
		resource.addRbacProperty()
		matchStrings = append(matchStrings, fmt.Sprintf("(n%d:%s {_uid: '%s'})", i, resource.Properties["kind"], resource.UID)) // e.g. (n0:Pod {_uid: 'abc123'})
		encodedProps, err := resource.EncodeProperties()
		if err != nil {
			glog.Error("Cannot encode resource ", resource.UID, ", excluding it from update: ", err)
			encodingErrors[resource.UID] = err
			continue
		}
		for k, v := range encodedProps {
			switch typed := v.(type) { // At this point it's either string or int64. Need to wrap in quotes if it's string
			case int64:
				setStrings = append(setStrings, fmt.Sprintf("n%d.%s=%d", i, k, typed)) // e.g. n0.<key>=<value>
			default:
				setStrings = append(setStrings, fmt.Sprintf("n%d.%s='%s'", i, k, typed)) // e.g. n0.<key>=<value>
			}
		}
	}

	queryString := fmt.Sprintf("%s%s", "MATCH "+strings.Join(matchStrings, ", "), " SET "+strings.Join(setStrings, ", "))

	return queryString, encodingErrors
}

func UpdateByName(resource Resource) (*rg2.QueryResult, error, bool) {
	resource.addRbacProperty()
	encodedProps, err := resource.EncodeProperties()
	if err != nil {
		glog.Error("Cannot encode resource ", resource.UID, ", excluding it from update: ", err)
		return &rg2.QueryResult{}, err, false
	}

	// we need to add the uid to the encoded props so if we update a dummy node we can attach a UID to it
	encodedProps["_uid"] = resource.UID

	// Decide if we want to SET again in REDIS -> if we keep setting values Redis is going OOM .
	// We check if REDIS is responding , if not we will clean our memory cache so that we write to redis
	// Check if this is from Update Intent and Map is not nil , we can check if the same values are already there in redis and return
	// with out performing a SET . This will help alleviate the OOM situation
	if isKeyClustersCache(resource.UID) {
		mapInRG := getClustersCache(resource.UID)
		if reflect.DeepEqual(mapInRG, encodedProps) {
			glog.V(3).Infof("No updates performed as the Object values have not changed")
			/*RG3 return QueryResult{Results: nil, Statistics: []string{"Update Not Required"}}, err RG3*/
			return &rg2.QueryResult{}, err, true
		}

	}
	setStrings := []string{} // Build the SET portion.
	for k, v := range encodedProps {
		switch typed := v.(type) { // At this point it's either string or int64. Need to wrap in quotes if it's string
		case int64:
			setStrings = append(setStrings, fmt.Sprintf("n.%s=%d", k, typed)) // e.g. n.<key>=<value>
		default:
			setStrings = append(setStrings, fmt.Sprintf("n.%s='%s'", k, typed)) // e.g. n.<key>=<value>
		}
	}

	// e.g. "MATCH (n:Cluster {name: 'abc123'}) SET n.foo=4"
	queryString := fmt.Sprintf("MATCH (n:%s {name: '%s'}) SET %s", resource.Properties["kind"], resource.Properties["name"], strings.Join(setStrings, ", "))
	glog.Info("SET Query ", queryString)
	resp, err := Store.Query(queryString)
	glog.Info("SET  resp.PropertiesSet()", resp.PropertiesSet())
	resp.PrettyPrint()
	//if there is no error store the Map in Global encodedPropsMap
	if err == nil {
		if isClustersCacheNil() {
			glog.V(3).Infof("Creating  new cluster cache")
			createClustersCache(resource.UID, encodedProps)
		} else {
			setClustersCache(resource.UID, encodedProps)

		}

	}

	return resp, err, false
}
