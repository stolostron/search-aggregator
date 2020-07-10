// Copyright (c) 2020 Red Hat, Inc.

package ccx

import (
	"time"

	"github.com/golang/glog"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// Pull Insights from CCX and merge with our search data.
func CCXSync() {
	glog.Info("Starting CCXSync()")

	for {
		glog.Info("Fetching data from CCX")

		// Here we need to make the HTTP call.
		// You can look at the collector pkg/send/httpsClient.go for an example to initialize and use the client.

		// Then process the result.
		processData()

		time.Sleep(time.Duration(30) * time.Second)
	}
}

func processData() {
	glog.Info("Process Data")

	// Here we loop the results from the API and extract the data we want to index.

	// for each CCX Insight
	props := make(map[string]interface{})

	props["name"] = "the name"
	props["kind"] = "Insight"
	props["apigroup"] = "insight.open-cluster-management.io"

	resource := db.Resource{
		Kind:           "Insight",
		UID:            string("insight__" + "something unique about the insight"),
		Properties:     props,
		ResourceString: "insights", // Needed for rbac, map to real cluster resource.
	}

	upsertNode(resource)
}

func upsertNode(resource db.Resource) {
	glog.Info("Upserting node.")

	// Here we insert/update the node into the search data.
	// See clusterWatch.go  processClusterUpsert()

	res, err, alreadySET := db.UpdateByName(resource) // <- UpdateByName() is optimized fot the Cluster nodes, we may need to make some changes there.
	if err != nil {
		glog.Warning("Error on UpdateByName() ", err)
	}
	if alreadySET {
		glog.V(4).Infof("Node for Insight %s already exist on DB.", resource.Properties["name"])
		return
	}

	if db.IsGraphMissing(err) || !db.IsPropertySet(res) {
		glog.Infof("Node for CCX insight %s does not exist, inserting it.", resource.Properties["name"])
		_, _, err = db.Insert([]*db.Resource{&resource}, "")
		if err != nil {
			glog.Error("Error adding Insight node with error: ", err)
			return
		}
	}
	glog.Warning("Unknown error upserting CCX node.") // This should not happen.
}
