// Copyright (c) 2020 Red Hat, Inc.

package ccx

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// PostBody Request body listing all clusters to get a report for
type PostBody struct {
	Clusters []string `json:"clusters"`
}

// ResponseBody Reports array
type ResponseBody struct {
	Reports map[string]interface{} `json:"reports"`
}

// Policy Spec
type Policy struct {
	Report PolicyReport `json:"report"`
}

// PolicyReport report
type PolicyReport struct {
	Meta struct {
		LastChecked string `json:"last_checked_at"`
	} `json:"meta"`
	Data []ReportData `json:"data"`
}

// ReportData rule violation data
type ReportData struct {
	Created      string `json:"created_at"`
	Description  string `json:"description"`
	Details      string `json:"details"`
	Reason       string `json:"reason"`
	Resolution   string `json:"resolution"`
	RiskOfChange int64  `json:"risk_of_change"`
	TotalRisk    int64  `json:"total_risk"`
	ID           string `json:"rule_id"`
}

// Sync - Query violations from CCX and merge with our search data.
func Sync() {
	glog.Info("Starting CCX Sync()")

	for {
		// We need to get the clusters under management || that have CCX enabled.
		mockClusters := PostBody{
			Clusters: []string{
				"34c3ecc5-624a-49a5-bab8-4fdc5e51a266",
				"74ae54aa-6577-4e80-85e7-697cb646ff37",
				"a7467445-8d6a-43cc-b82c-7007664bdf69",
				"ee7d2bf4-8933-4a3a-8634-3328fe806e08",
			},
		}
		reqBody, _ := json.Marshal(mockClusters)
		clusterReportURL := config.Cfg.CCXServer + "/clusters"
		postResp, postErr := http.Post(clusterReportURL, "application/json", bytes.NewBuffer(reqBody))
		if postErr != nil {
			glog.Error(postErr)
		}
		defer postResp.Body.Close()

		var responseBody ResponseBody
		var policy Policy
		// return []byte of response body
		data, _ := ioutil.ReadAll(postResp.Body)
		// unmarshal response data into the ResponseBody struct
		unmarshalError := json.Unmarshal(data, &responseBody)
		if unmarshalError != nil {
			glog.Error(unmarshalError)
		}
		// loop through the clusters in the response and create the PolicyReport node for each violation
		for cluster := range responseBody.Reports {
			// convert report data into []byte
			reportBytes, _ := json.Marshal(responseBody.Reports[cluster])
			// unmarshal response data into the Policy struct
			unmarshalReportError := json.Unmarshal(reportBytes, &policy)
			if unmarshalReportError != nil {
				glog.Error(unmarshalReportError)
			}
			processData(&policy, cluster)
		}

		time.Sleep(time.Duration(30) * time.Second)
	}
}

func processData(data *Policy, cluster string) {
	glog.Info("Process Data")

	//Here we loop the results from the API and extract the data we want to index.
	for rule := range data.Report.Data {
		// for each CCX rule violation create a node
		ruleViolation := data.Report.Data[rule]
		props := make(map[string]interface{})
		props["name"] = string(ruleViolation.ID + "_" + cluster) // TODO better naming?
		props["namespace"] = cluster                             // is this redundant with _hubNamespace?
		props["_clusterNamespace"] = cluster
		props["cluster"] = cluster
		props["kind"] = "PolicyReport"
		props["apigroup"] = "open-cluster-management.io"
		props["ruleID"] = ruleViolation.ID
		props["created"] = ruleViolation.Created
		props["message"] = ruleViolation.Description
		props["details"] = ruleViolation.Details
		props["reason"] = ruleViolation.Reason
		props["resolution"] = ruleViolation.Resolution
		props["totalRisk"] = ruleViolation.TotalRisk
		props["riskOfChange"] = ruleViolation.RiskOfChange
		props["lastChanged"] = data.Report.Meta.LastChecked

		resource := db.Resource{
			Kind:           "PolicyReport",
			UID:            string(ruleViolation.ID + "_" + cluster), // TODO find a better unique identifier?
			Properties:     props,
			ResourceString: "policyreports", // Needed for rbac, map to real cluster resource.
		}

		upsertNode(resource)
	}
}

func upsertNode(resource db.Resource) {
	glog.V(4).Info("Upserting node.")
	res, err, alreadySET := db.UpdateByName(resource) // <- TODO UpdateByName() is optimized fot the Cluster nodes, we may need to make some changes there.
	if err != nil {
		glog.Warning("Error on UpdateByName() ", err)
	}
	if alreadySET {
		glog.V(4).Infof("Node for rule violation %s already exist on DB.", resource.Properties["name"])
		return
	}

	if db.IsGraphMissing(err) || !db.IsPropertySet(res) {
		glog.Infof("Node for CCX rule violation %s does not exist, inserting it.", resource.Properties["name"])
		_, _, err = db.Insert([]*db.Resource{&resource}, "")
		if err != nil {
			glog.Error("Error adding rule violation node with error: ", err)
			return
		}
		return
	}
	glog.Warning("Unknown error upserting CCX node.") // This should not happen.
}
