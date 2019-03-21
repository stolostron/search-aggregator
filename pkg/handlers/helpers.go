package handlers

import (
	"crypto/sha1"
	"fmt"

	"github.com/golang/glog"
	rg "github.com/redislabs/redisgraph-go"
)

// ComputeHash computes a new hash using the hashes from all the current resources.
func computeHash(graph *rg.Graph, clusterName string) (totalResources int, hash string) {
	query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' RETURN n._hash ORDER BY n._hash ASC" // TODO: I'll worry about strings later.
	rs, _ := graph.Query(query)

	allHashes := rs.Results[1:] // Start at index 1 because index 0 has the header.

	h := sha1.New()
	_, err := h.Write([]byte(fmt.Sprintf("%x", allHashes))) // TODO: I'll worry about strings later.
	if err != nil {
		glog.Info("Error generating hash.")
	}
	bs := h.Sum(nil)

	totalResources = len(allHashes)
	hash = fmt.Sprintf("%x", bs)
	glog.Info("Total resources:", totalResources)
	glog.Info("All hashes:", fmt.Sprintf("%s", allHashes)) // TODO: I'll worry about strings later.
	glog.Info("Current hash:", hash)
	return totalResources, hash
}
