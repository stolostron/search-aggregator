// Copyright (c) 2020 Red Hat, Inc.

package ccx

import (
	"time"

	"github.com/golang/glog"
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

	upsertNode()
}

func upsertNode() {
	glog.Info("Upserting node.")

	// Here we insert/update the node into the search data.
	// See clusterWatch.go  processClusterUpsert()
}
