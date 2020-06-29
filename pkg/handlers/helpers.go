/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package handlers

import (
	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// returns the total number of nodes on cluster
func computeNodeCount(clusterName string) int {
	resp, err := db.TotalNodes(clusterName)
	if err != nil {
		glog.Errorf("Error node count for cluster %s: %s", clusterName, err)
		return 0
	}

	if resp.Empty() { // Just 1 would be just the header
		glog.Info("Cluster ", clusterName, " doesn't have any nodes")
		return 0
	}
	//Iterating to next record to get count - count is in the first index(0) of the first record
	for resp.Next() {
		record := resp.Record()
		countInterface := record.GetByIndex(0)
		if count, ok := countInterface.(int); ok {
			return count
		} else {
			glog.Errorf("Could not parse node count results for cluster %s", clusterName)
		}
	}
	return 0
}

// computeIntraEdges counts the nubmer of intra edges returned form db
func computeIntraEdges(clusterName string) int {
	resp, err := db.TotalIntraEdges(clusterName)
	if err != nil {
		glog.Errorf("Error fetching edge count for cluster %s: %s", clusterName, err)
		return 0
	}

	if resp.Empty() { // Just 1 would be just the header
		glog.Info("Cluster ", clusterName, " doesn't have any edges")
		return 0
	}
	//Iterating to next record to get count - count is in the first index(0) of the first record
	for resp.Next() {
		record := resp.Record()
		countInterface := record.GetByIndex(0)
		if count, ok := countInterface.(int); ok {
			return count
		} else {
			glog.Errorf("Could not parse edge count results for cluster %s", clusterName)
		}
	}

	return 0
}

func assertClusterNode(clusterName string) bool {
	if clusterName == "local-cluster" || config.Cfg.SkipClusterValidation == "true" {
		_, err := db.MergeDummyCluster(clusterName)
		if err != nil {
			glog.Error("Could not merge local cluster Cluster resource: ", err)
			return false
		}
	} else {
		resp, err := db.CheckClusterResource(clusterName)
		if err != nil {
			glog.Error("Could not check cluster resource by name: ", err)
			return false
		}
		//Iterating to next record to get count - count is in the first index(0) of the first record
		for resp.Next() {
			record := resp.Record()
			countInterface := record.GetByIndex(0)
			if count, ok := countInterface.(int); ok {
				if count <= 0 {
					return false
				}
			} else {
				glog.Errorf("Could not parse Cluster count results for cluster %s", clusterName)
			}
		}
	}

	return true
}
