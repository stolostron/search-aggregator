/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.
*/
// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package dbconnector

import (
	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	rg2 "github.com/redislabs/redisgraph-go"
)

const CHUNK_SIZE = 40 // Used for the chunked operations in other files.

// Resource - Describes a resource (node)
type Resource struct {
	Kind           string `json:"kind,omitempty"`
	UID            string `json:"uid,omitempty"`
	ResourceString string `json:"resourceString,omitempty"`
	Properties     map[string]interface{}
}

// Describes a relationship between resources
type Edge struct {
	SourceUID, DestUID   string
	EdgeType             string
	SourceKind, DestKind string
}

// Represents the results of a chunked db operation
type ChunkedOperationResult struct {
	ResourceErrors      map[string]error // errors keyed by UID
	ConnectionError     error            // For when db conn is down. Supersedes ResourceErrors and SuccessfulResources
	SuccessfulResources int              // Number that were successfully completed
	EdgesAdded          int
	EdgesDeleted        int
}

// Deletes all resources for given cluster
func DeleteCluster(clusterName string) (*rg2.QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return &rg2.QueryResult{}, err
	}
	query := SanitizeQuery("MATCH (n {cluster:'%s'}) DELETE n", clusterName)
	return Store.Query(query)
}

func TotalNodes(clusterName string) (*rg2.QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return &rg2.QueryResult{}, err
	}
	query := SanitizeQuery("MATCH (n {cluster:'%s'}) RETURN count(n)", clusterName)
	return Store.Query(query)
}

// Returns a result set with all INTRA edges within the clusterName
func TotalIntraEdges(clusterName string) (*rg2.QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return &rg2.QueryResult{}, err
	}
	query := SanitizeQuery("MATCH (s {cluster:'%s'})-[e]->(d {cluster:'%s'}) WHERE (e._interCluster <> true) OR (e._interCluster IS NULL) RETURN count(e)", clusterName, clusterName)
	resp, err := Store.Query(query)
	return resp, err
}

func MergeDummyCluster(name string) (*rg2.QueryResult, error) {
	kubeVersion := ""
	discoveryClient := config.GetDiscoveryClient()

	if discoveryClient != nil {
		clusterClientServerVersion, err := discoveryClient.ServerVersion()
		if err != nil {
			glog.Error("Error getting hub kubernetes version. clusterClientServerVersion was not found.")
		} else {
			kubeVersion = clusterClientServerVersion.String()
		}
	}

	query := SanitizeQuery(
		"MERGE (c:Cluster {name: '%s', kind: 'cluster'}) SET c.status = 'OK', c.kubernetesVersion = '%s'",
		name, kubeVersion)
	return Store.Query(query)
}

func CheckClusterResource(clusterName string) (*rg2.QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		glog.Warning("Error validating cluster:", clusterName)
		return &rg2.QueryResult{}, err
	}
	query := SanitizeQuery("MATCH (c:Cluster {name: '%s'}) RETURN count(c)", clusterName)
	return Store.Query(query)
}
