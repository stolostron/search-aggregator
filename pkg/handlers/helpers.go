/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package handlers

import (
	"crypto/sha1"
	"fmt"

	"github.com/golang/glog"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// computeHash computes a new hash using the hashes from all the current resources, and returns that and the total number of resources.
func computeHash(clusterName string) (totalResources int, hash string) {

	resp, clusterNameErr, err := db.Hashes(clusterName)
	if clusterNameErr != nil {
		glog.Errorf("Cannot encode clusterName %s: %s", clusterName, clusterNameErr)
	}
	if err != nil {
		glog.Errorf("Error fetching hashes for cluster %s: %s", clusterName, err)
	}

	if len(resp.Results) <= 1 { // Just 1 would be just the header
		glog.Info("Cluster ", clusterName, " doesn't have any resources")
		return 0, ""
	}

	allHashes := resp.Results[1:] // Start at index 1 because index 0 has the header.

	h := sha1.New()
	_, err = h.Write([]byte(fmt.Sprintf("%x", allHashes))) // TODO: I'll worry about strings later.
	if err != nil {
		glog.Error("Error generating hash.")
	}
	bs := h.Sum(nil)

	totalResources = len(allHashes)
	hash = fmt.Sprintf("%x", bs)

	return // Returns named returns in header
}
