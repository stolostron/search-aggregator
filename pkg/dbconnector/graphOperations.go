/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
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
	SourceUID, DestUID string
	EdgeType           string
}

// Represents the results of a chunked db operation
type ChunkedOperationResult struct {
	ResourceErrors      map[string]error // errors keyed by UID
	ConnectionError     error            // For when redis conn is down. If this is set, then ResourceErrors and SuccessfulResources are irrelevant.
	SuccessfulResources int              // Number that were successfully completed
}

// Deletes all resources for given cluster
func DeleteCluster(clusterName string) (QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return QueryResult{}, err
	}
	query := fmt.Sprintf("MATCH (n) WHERE n.cluster = '%s' DELETE n", clusterName)
	return Store.Query(query)
}

func TotalNodes(clusterName string) (QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return QueryResult{}, err
	}
	query := fmt.Sprintf("MATCH (n) WHERE n.cluster = '%s' RETURN count(n)", clusterName)
	return Store.Query(query)
}

// Returns a result set with all INTRA edges within the clusterName
func TotalIntraEdges(clusterName string) (QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return QueryResult{}, err
	}
	query := fmt.Sprintf("MATCH (s)-[e]->(d) WHERE s.cluster='%s' AND e._interCluster != true RETURN count(e)", clusterName)
	resp, err := Store.Query(query)
	return resp, err
}

func MergeDummyCluster(name string) (QueryResult, error) {
	kubeVersion := ""
	if config.ClusterClient != nil {
		clusterClientServerVersion, verr := config.ClusterClient.ServerVersion()
		if verr != nil {
			glog.Error("clusterClientServerVersion not found")
		} else {
			kubeVersion = clusterClientServerVersion.String()
		}
	} else {
		glog.Error("ClusterClient not initialized")
	}
	query := fmt.Sprintf("MERGE (c:Cluster {name: '%s', kind: 'cluster'}) SET c.status = 'OK', c.kubernetesVersion = '%s'", name, kubeVersion)
	return Store.Query(query)
}

func CheckClusterResource(clusterName string) (QueryResult, error) {
	err := ValidateClusterName(clusterName)
	if err != nil {
		return QueryResult{}, err
	}
	query := fmt.Sprintf("MATCH (c:Cluster {name: '%s'}) RETURN count(c)", clusterName)
	return Store.Query(query)
}
