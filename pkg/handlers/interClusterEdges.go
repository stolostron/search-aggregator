/*
 * (C) Copyright IBM Corporation 2019 All Rights Reserved
 * Copyright (c) 2020 Red Hat, Inc.
 * Copyright Contributors to the Open Cluster Management project
 */
package handlers

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
	rg2 "github.com/redislabs/redisgraph-go"
)

var ApplicationLastUpdated time.Time
var previousAppInstance int

func getApplicationUpdateTime() time.Time {
	return ApplicationLastUpdated
}

func currAppInstance() int {
	n, err := rand.Int(rand.Reader, big.NewInt(99999))
	if err != nil {
		glog.Error("Not able to generate random number for appInstance")
		if previousAppInstance == 99999 {
			return previousAppInstance - 1
		}
		return previousAppInstance + 1
	}
	return int(n.Int64())
}

// runs all the specific inter-cluster relationships we want to connect
func BuildInterClusterEdges() {
	var tranforms = []struct {
		transfrom   func() (rg2.QueryResult, error)
		description string
		getUpdate   func() time.Time
	}{
		{
			buildSubscriptions,
			"connecting subscription edges",
			getApplicationUpdateTime,
		},
	}

	for {
		interval := time.Duration(config.Cfg.EdgeBuildRateMS) * time.Millisecond
		time.Sleep(interval)

		glog.V(3).Info("Building intercluster edges")

		for _, edgeFunc := range tranforms {
			// if no updates have been made during the sleep skip edge building
			// logic here is if timestamp < current time - interval
			updateTime := edgeFunc.getUpdate()
			// Comment out the Next 3 lines if you want to test intercluster locally
			if updateTime.Before(time.Now().Add(-interval)) {
				glog.V(3).Infof("Skipping %s because nothing has changed", edgeFunc.description)
				continue
			}
			glog.V(3).Infof("Running  %s because change observed", edgeFunc.description)
			_, err := edgeFunc.transfrom()
			if err != nil {
				glog.Errorf("Error %s : %s", edgeFunc.description, err)
			}
		}
	}
}

func getUIDsForSubscriptions() (*rg2.QueryResult, error) {
	query := "MATCH (n {kind: 'subscription'}) RETURN n._uid"
	uidResults, err := db.Store.Query(query)
	return uidResults, err
}

func buildSubscriptions() (rg2.QueryResult, error) {
	// Record start time
	start := time.Now()
	currentAppInstance := currAppInstance()
	// Making sure that this instanceID is different from the previous
	for currentAppInstance == previousAppInstance {
		currentAppInstance = currAppInstance()
	}

	// list of remote subscriptions
	query := "MATCH (n:Subscription) WHERE n.cluster <> 'local-cluster' RETURN n._uid, n._hostingSubscription"
	remoteSubscriptions, err := db.Store.Query(query)
	if err != nil {
		return rg2.QueryResult{}, err
	}
	if !remoteSubscriptions.Empty() { //Check if any results are returned
		// list of hub subscriptions
		query = "MATCH (n:Subscription) WHERE  n.cluster='local-cluster' RETURN n._uid, n.namespace+'/'+n.name"
		hubSubscriptons, err := db.Store.Query(query)
		if err != nil {
			return rg2.QueryResult{}, err
		}

		//Adding all hubsubscriptions to a map: key is subscription's "namespace+'/'+name", value is UID
		hubSubMap := make(map[string]string)
		for hubSubscriptons.Next() {
			hubRecord := hubSubscriptons.Record()
			hubSubMap[hubRecord.GetByIndex(1).(string)] = hubRecord.GetByIndex(0).(string)
		}

		for remoteSubscriptions.Next() {
			remoteRecord := remoteSubscriptions.Record()
			var remoteSub [2]string

			if _, ok := remoteRecord.GetByIndex(0).(string); ok {
				remoteSub[0] = remoteRecord.GetByIndex(0).(string)
			}
			if _, ok := remoteRecord.GetByIndex(1).(string); ok {
				remoteSub[1] = remoteRecord.GetByIndex(1).(string)
			}
			var hubSubUID string
			var ok bool
			if remoteSub[1] != "" {
				hubSubUID, ok = hubSubMap[remoteSub[1]]
			}
			if ok {
				// Add an edge between remoteSub and hubSub.
				query0 := db.SanitizeQuery("MATCH (hubSub:Subscription {_uid: '%s'}), (remoteSub:Subscription {_uid: '%s'}) CREATE (remoteSub)-[:hostedSub {_interCluster: true,app_instance: %d}]->(hubSub)",
					hubSubUID, remoteSub[0], currentAppInstance)
				resp, err := db.Store.Query(query0)
				if err != nil {
					glog.Errorf("Error %s : %s", query, err) //Logging error so that loop will continue
				} else {
					glog.V(4).Info("Number of edges created by query: ", query, " is : ", resp.RelationshipsCreated())
				}
			}
		}
		//Delete interclusters with other instance ids after all hub subscriptions are processed
		deleteOldInstance := db.SanitizeQuery("MATCH ()-[e {_interCluster:true}]->() WHERE (type(e)='hostedSub' OR type(e)='usedBy' OR type(e)='deployedBy') AND e.app_instance<>%d DELETE e",
			currentAppInstance)
		_, err = db.Store.Query(deleteOldInstance)
		if err != nil {
			return rg2.QueryResult{}, err
		}
	} else {
		//Delete interclusters because there is no remote subscriptions
		deleteOldInstance := db.SanitizeQuery("MATCH ()-[e {_interCluster:true}]->() WHERE (type(e)='hostedSub' OR type(e)='usedBy' OR type(e)='deployedBy') AND e.app_instance<>%d DELETE e",
			currentAppInstance)
		_, err = db.Store.Query(deleteOldInstance)
		if err != nil {
			return rg2.QueryResult{}, err
		}

	}
	previousAppInstance = currentAppInstance // Next iteration we dont want to use this ID
	// Record elapsed time
	elapsed := time.Since(start)
	// Log a warning if it takes more than 100ms.
	if elapsed.Nanoseconds() > 100*1000*1000 {
		glog.Warningf("Intercluster edge deletion and re-creation took %s", elapsed)
	} else {
		glog.V(4).Infof("Intercluster edge deletion and re-creation took %s", elapsed)
	}
	return rg2.QueryResult{}, nil
}
