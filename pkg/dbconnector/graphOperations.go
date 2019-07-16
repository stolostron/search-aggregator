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

	rg "github.com/redislabs/redisgraph-go"
)

const CHUNK_SIZE = 40 // Used for the chunked operations in other files.

// Resource - Describes a resource (node)
type Resource struct {
	Kind           string `json:"kind,omitempty"`
	UID            string `json:"uid,omitempty"`
	Hash           string `json:"hash,omitempty"`
	ResourceString string `json:"resourceString,omitempty"`
	Properties     map[string]interface{}
}

// Edge Type Constants
type EdgeType string

// Describes a relationship between resources
type Edge struct {
	SourceUID, DestUID string
	EdgeType           EdgeType
}

// Represents the results of a chunked db operation
type ChunkedOperationResult struct {
	ResourceErrors      map[string]error // errors keyed by UID
	ConnectionError     error            // For when redis conn is down. If this is set, then ResourceErrors and SuccessfulResources are irrelevant.
	SuccessfulResources int              // Number that were successfully completed
}

var Store DBStore

// Interface for the DB dependency. Used for mocking rg.
type DBStore interface {
	Query(q string) (rg.QueryResult, error)
}

//Real redis graph, will hold a connection to RedisGraph
type RedisGraphStore struct{} // No properties. It just has the method

// Executes the given query against redisgraph.
// Called by the other functions in this file
func (RedisGraphStore) Query(q string) (rg.QueryResult, error) {
	// Get connection from the pool
	conn := Pool.Get() // This will block if there aren't any valid connections that are available.
	defer conn.Close()
	g := rg.Graph{
		Conn: conn,
		Name: GRAPH_NAME,
	}
	queryResult, err := g.Query(q)
	return queryResult, err
}

// Deletes all resources for given cluster
// First error is the error from encoding (only one possible), second is from the query.
func DeleteCluster(clusterName string) (rg.QueryResult, error, error) {
	query, badClusterNameErr := deleteClusterQuery(clusterName)
	if badClusterNameErr != nil {
		return rg.QueryResult{}, badClusterNameErr, nil
	}
	resp, err := Store.Query(query)
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
	resp, err := Store.Query(query)
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
