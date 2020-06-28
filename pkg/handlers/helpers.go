/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package handlers

import (
	"strconv"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// returns the total number of nodes on cluster
func computeNodeCount(clusterName string) int {
	resp, err := db.TotalNodes(clusterName)
	if err != nil {
		glog.Errorf("Error node count for cluster %s: %s", clusterName, err)
	}

	if len(resp.Results) <= 1 { // Just 1 would be just the header
		return 0
	}

	// headers are at the top of table - count is in second row
	countString := resp.Results[1][0]
	count, err := strconv.Atoi(countString)

	if err != nil {
		glog.Errorf("Could not parse node count string for cluster %s: %s", clusterName, countString)
	}

	return count
}

// computeIntraEdges counts the nubmer of intra edges returned form db
func computeIntraEdges(clusterName string) int {
	resp, err := db.TotalIntraEdges(clusterName)
	if err != nil {
		glog.Errorf("Error fetching edge count for cluster %s: %s", clusterName, err)
	}

	if len(resp.Results) <= 1 { // Just 1 would be just the header
		glog.Info("Cluster ", clusterName, " doesn't have any edges")
		return 0
	}

	// headers are at the top of table - count is in second row
	countString := resp.Results[1][0]
	count, err := strconv.Atoi(countString)

	if err != nil {
		glog.Errorf("Could not parse edge count string for cluster %s: %s", clusterName, countString)
	}

	return count
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

		if len(resp.Results) <= 1 { // Just 1 would be just the header
			glog.Infof("Cluster %s does not exist.", clusterName)
			return false
		}

		// headers are at the top of table - count is in second row
		countString := resp.Results[1][0]
		count, err := strconv.Atoi(countString)

		if err != nil {
			glog.Errorf("Could not parse Cluster count string for cluster %s: %s", clusterName, countString)
			return false
		}

		if count <= 0 {
			return false
		}
	}

	return true
}
