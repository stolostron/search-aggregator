// Copyright (c) 2020 Red Hat, Inc.

package ccx

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// Insight Spec
type Insight struct {
	Report InsightReport `json:"report"`
}

// InsightReport report
type InsightReport struct {
	Meta struct {
		LastChecked string `json:"last_checked_at"`
	} `json:"meta"`
	Data []InsightData `json:"data"`
}

// InsightData Insight rule violation data
type InsightData struct {
	Created      string `json:"created_at"`
	Description  string `json:"description"`
	Details      string `json:"details"`
	Reason       string `json:"reason"`
	Resolution   string `json:"resolution"`
	RiskOfChange int64  `json:"risk_of_change"`
	TotalRisk    int64  `json:"total_risk"`
	ID           string `json:"rule_id"`
}

// Sync Pull Insights from CCX and merge with our search data.
func Sync() {
	glog.Info("Starting Sync()")

	for {
		glog.Info("Fetching data from CCX")

		// Here we need to make the HTTP call.
		// You can look at the collector pkg/send/httpsClient.go for an example to initialize and use the client.

		// We need to get the clusters under management || that have CCX enabled.
		var clusters [4]string
		clusters[0] = "34c3ecc5-624a-49a5-bab8-4fdc5e51a266"
		clusters[1] = "74ae54aa-6577-4e80-85e7-697cb646ff37"
		clusters[2] = "a7467445-8d6a-43cc-b82c-7007664bdf69"
		clusters[3] = "ee7d2bf4-8933-4a3a-8634-3328fe806e08"

		for c := range clusters {
			// GET url for each cluster (11789772 is the org.. need to look into this)
			clusterReportURL := config.Cfg.CCXServer + "/v1/report/11789772/" + clusters[c]
			resp, err := http.Get(clusterReportURL)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()

			var insight Insight
			data, _ := ioutil.ReadAll(resp.Body)
			err = json.Unmarshal(data, &insight)
			if err != nil {
				panic(err)
			}

			processData(&insight, clusters[c])
		}
		time.Sleep(time.Duration(30) * time.Second)
	}
}

func processData(data *Insight, cluster string) {
	glog.Info("Process Data")

	//Here we loop the results from the API and extract the data we want to index.
	for rule := range data.Report.Data {
		// for each CCX Insight rule violation create a node
		ruleViolation := data.Report.Data[rule]
		props := make(map[string]interface{})
		props["name"] = string(ruleViolation.ID + "_" + cluster) // TODO better naming?
		props["namespace"] = cluster
		props["kind"] = "Insight"
		props["created"] = ruleViolation.Created
		props["description"] = ruleViolation.Description
		props["details"] = ruleViolation.Details
		props["reason"] = ruleViolation.Reason
		props["resolution"] = ruleViolation.Resolution
		props["totalRisk"] = ruleViolation.TotalRisk
		props["riskOfChange"] = ruleViolation.RiskOfChange
		props["lastChanged"] = data.Report.Meta.LastChecked

		resource := db.Resource{
			Kind:           "Insight",                                // Insight? CCXInsight?
			UID:            string(ruleViolation.ID + "_" + cluster), // TODO find a better unique identifier?
			Properties:     props,
			ResourceString: "insights", // Needed for rbac, map to real cluster resource.
		}

		upsertNode(resource)
	}
}

func upsertNode(resource db.Resource) {
	glog.Info("Upserting node.")

	// Here we insert/update the node into the search data.
	// See clusterWatch.go  processClusterUpsert()

	res, err, alreadySET := db.UpdateByName(resource) // <- TODO UpdateByName() is optimized fot the Cluster nodes, we may need to make some changes there.
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
		return
	}
	glog.Warning("Unknown error upserting CCX node.") // This should not happen.
}
