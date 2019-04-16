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

// This file contains Arg getter functions used for getting redis args for graph queries.

// Resource - Describes a resource
type Resource struct {
	Kind       string `json:"kind,omitempty"`
	UID        string `json:"uid,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Properties map[string]interface{}
}

// Executes the given query against redisgraph.
// Called by the other functions in this file
func Query(q string) (rg.QueryResult, error) {
	// Get connection from the pool
	// TODO may need a timeout here.
	conn := Pool.Get() // This will block if there aren't any valid connections that are available.
	defer conn.Close()
	g := rg.Graph{
		Conn: conn,
		Name: GRAPH_NAME,
	}
	queryResult, err := g.Query(q)
	return queryResult, err
}

// Inserts given resources into graph.
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
		glog.Warning("Appending: ", resource.UID) // RM
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

	// glog.Warning("INSERT QUERY: ", queryString) //RM

	return queryString, encodingErrors
}

// Updates given resources into graph.
// Returns the result, any errors when encoding, and any error from the query itself.
//FIXME this function is screwed way up for now while waiting on a bugfix
func Update(resources []*Resource) (rg.QueryResult, map[string]error, error) {
	query, encodingErrors := updateQuery(resources) // Encoding errors are recoverable, but we still report them
	resp, err := Query(query)
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
		matchStrings = append(matchStrings, fmt.Sprintf("(n%d:%s {_uid: '%s'})", i, resource.Properties["kind"], resource.UID)) // e.g. (n0:Pod {_uid: 'abc123'})
		encodedProps, err := resource.encodeProperties()
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

// Deletes resources with the given UIDs.
// No encoding errors possible with this operation.
func Delete(uids []string) (rg.QueryResult, error) {
	query := deleteQuery(uids)
	resp, err := Query(query)
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

// Deletes all resources for given cluster
// First error is the error from encoding (only one possible), second is from the query.
func DeleteCluster(clusterName string) (rg.QueryResult, error, error) {
	query, badClusterNameErr := deleteClusterQuery(clusterName)
	if badClusterNameErr != nil {
		return rg.QueryResult{}, badClusterNameErr, nil
	}
	resp, err := Query(query)
	return resp, badClusterNameErr, err
}

// Returns query to delete all graph data for a given cluster.
func deleteClusterQuery(clusterName string) (string, error) {
	err := validateClusterName(clusterName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("MATCH (n) WHERE n.cluster = '%s' DELETE n", clusterName), nil
}

func Hashes(clusterName string) (rg.QueryResult, error, error) {
	query, badClusterNameErr := hashesQuery(clusterName)
	if badClusterNameErr != nil {
		return rg.QueryResult{}, badClusterNameErr, nil
	}
	resp, err := Query(query)
	return resp, badClusterNameErr, err
}

// Returns query to get hashes for given cluster's resources in redisgraph.
func hashesQuery(clusterName string) (string, error) {
	err := validateClusterName(clusterName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("MATCH (n) WHERE n.cluster = '%s' RETURN n._hash ORDER BY n._hash ASC", clusterName), nil
}
