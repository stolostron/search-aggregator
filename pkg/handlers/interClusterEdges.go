/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.
*/
// Copyright (c) 2020 Red Hat, Inc.

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
			glog.V(3).Infof("Running  %s because changed observed", edgeFunc.description)
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

/*func buildPolicyEdges() (rg.QueryResult, error) {
	// Record start time
	start := time.Now()
	policiesWithParent := make(map[string]bool)
	//Get all the Nodes connected to Kind :policy with interCluter true - store the UIDs
	// These Policies may be Parent Policies  which We would like Policy and VA/MA from remotes to connected to
	hub_query := "MATCH (n)-[:ownedBy {_interCluster: true}]->(k:Policy) RETURN n._uid"
	connectedPolicies, herr := db.Store.Query(hub_query)
	if herr != nil {
		return rg.QueryResult{}, herr
	}
	if len(connectedPolicies.Results) > 1 { //  If there is any result
		for _, hubPolicy := range connectedPolicies.Results[1:] {
			policiesWithParent[hubPolicy[0]] = true
		}

	}
	//Get all the VA/MA policies using (VAMA)-[:ownedBy]->polices from Remote clusters , will get vama policy uids , their parents uid and namespace/name of the Parent policy in HUB -- refer comment before hub_query
	mc_query := "MATCH (child)-[:ownedBy]->(policy:Policy) where exists(policy._parentPolicy)=true AND policy.cluster!='local-cluster' AND policy.apigroup='policy.mcm.ibm.com' AND (child.kind='mutationpolicy' OR child.kind='vulnerabilitypolicy') return  policy._parentPolicy,policy._uid,child._uid"
	mcPolicies, merr := db.Store.Query(mc_query)
	if merr != nil {
		return rg.QueryResult{}, merr
	}
	if len(mcPolicies.Results) > 1 { //  If there is any result
		//create a map for having unique policyIds , VA/MA Uids
		resultUids := make(map[string]string)
		for _, mcPolicy := range mcPolicies.Results[1:] {
			if mcPolicy[0] != "" && mcPolicy[1] != "" && mcPolicy[2] != "" {
				resultUids[mcPolicy[1]] = mcPolicy[0] // Set the parent -> _parentPolicy
				resultUids[mcPolicy[2]] = mcPolicy[0] // Set the VA/MA (child) -> _parentPolicy
			}
		}

		//Now iterate the result and create the needed edges
		for key, val := range resultUids {
			_, ok := policiesWithParent[key]
			if ok {
				// If we have the Source UID - we expect that no one can manually edit the Parent and point to a different parent
				// If they delete the edge to Policy will automatically gets deleted
				continue //we already have the uid connected to a parent policy
			} else { // We need to create an edge between the Key and Val
				remoteUID := key
				hubParentPolicy := strings.Split(val, "/") // index 0 namespace , index 1 policy name
				//create Edge
				createQuery := fmt.Sprintf("MATCH (parent {kind: 'policy', cluster: 'local-cluster', namespace: '%s', name: '%s'}), (policy {_uid: '%s'}) CREATE (policy)-[:ownedBy {_interCluster: true}]->(parent)", hubParentPolicy[0], hubParentPolicy[1], remoteUID)
				glog.V(4).Infof("Policy InterCluster edge to be created %s : ", createQuery)
				_, crerr := db.Store.Query(createQuery)
				if crerr != nil {
					glog.V(4).Infof("Policy InterCluster  edge creation failed  %s : ", createQuery)
					continue // if there is an error continue with next Policy
				}
			}
		}
	}
	// Typical Setup of Policies (1) A Parent Policy in HUB (2) A policy in HUB which is exaclty copied to REMOTE (3) The Replica of Policy in item -2- In REMOTE (4) A VA or MA policy connected to -Policy in 3-
	// 2 hop query , get all subscriptions that violates either VA/MA policy -> connected to its remote  parent
	// make a direct connection from subscription to parent policy - Also We are NOT making Node match like (s:Subscription) , as we noticed that (s: {kind:'subscription'}) was better
	randomTrackId := rand.Intn(99999) // A track Id which we will use in Redisgraph to find out what edges were created in this call , rest all can be deleted . This is to avoid creating deplicate edges by redisgraph
	querySubToPolicy := fmt.Sprintf("MATCH (subscription {kind:'subscription'})-[:violates]->(vama)-[:ownedBy {_interCluster:true}]->(parent{kind:'policy'}) where vama.kind='mutationpolicy' OR vama.kind='vulnerabilitypolicy' WITH subscription as SS , parent as PP MATCH (SSA {kind:'subscription'}),(PPA {kind:'policy'}) where SSA._uid=SS._uid AND PPA._uid=PP._uid  CREATE (SSA)-[t:violates {_interCluster: true,pol_instance: %d}]->(PPA)", randomTrackId)
	glog.V(4).Infof("Policy InterCluster 2 Hop Query for Subsciption -> Policy edge to be created %s  ", querySubToPolicy)
	_, subPolErr := db.Store.Query(querySubToPolicy)
	if subPolErr != nil {
		return rg.QueryResult{}, subPolErr
	}

	// The Above quey will add edges to Subscriptions to Policies - but it can also create duplicate edges, but we know what we created in this instance by using the randomTrackId
	//We will delete all the duplicate edges - they will have old different randomTrackID - If we are unlucky to get same ramdom ID 2 consecutive times , then we will have , duplicates for some time , though the data is valid
	deleteSubToPolicy := fmt.Sprintf("MATCH (subscription:Subscription)-[e:violates{_interCluster:true}]->(parent:Policy) where e.pol_instance!=%d delete e", randomTrackId)
	glog.V(4).Infof("Policy InterCluster Delete Query for Subscription -> Policy edge to be deleted %s  ", deleteSubToPolicy)
	_, delSubPolErr := db.Store.Query(deleteSubToPolicy)
	if delSubPolErr != nil {
		return rg.QueryResult{}, delSubPolErr
	}
	//We need to call build Subscriptions as we need to pull the edges to Application
	_, errSubs := buildSubscriptions()
	if delSubPolErr != nil {
		return rg.QueryResult{}, errSubs
	}
	// Record elapsed time
	elapsed := time.Since(start)
	// Log a warning if it takes more than 100ms.
	if elapsed.Nanoseconds() > 100*1000*1000 {
		glog.Warningf("Intercluster policy edge creation took %s", elapsed)
	} else {
		glog.V(4).Infof("Intercluster policy edge creation took %s", elapsed)
	}
	return rg.QueryResult{}, nil

}*/

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
			// remoteSub[1] has the hosting subscription information which is in the format hosting subscription's "namespace+'/'+name"
			if remoteSub[1] != "" {
				//So, we look up if the hostingSubscription is in the hubSubMap. If it is there, get the UID
				if hubSubUID, ok := hubSubMap[remoteSub[1]]; ok {

					//TODO: For the subscription model, all intercluster edges are named as 'hostedSub {_interCluster: true}'. Change this to relevant names in future
					//To add edges from hubSub to all resources connected to the remoteSub (bidirectional) - incoming edges and outgoing edges
					// Add an edge between remoteSub and hubSub. Add edges from hubSub to all resources the remoteSub connects to
					query1 := db.SanitizeQuery("MATCH (hubSub {_uid: '%s'}) MATCH (remoteSub {_uid: '%s'})-[]->(n) WHERE n.kind <> 'application' AND n.kind <> 'subscription' MERGE (remoteSub)-[:hostedSub {_interCluster: true,app_instance: %d}]->(hubSub) MERGE (n)-[:hostedSub {_interCluster: true,app_instance: %d}]->(hubSub)", hubSubUID, remoteSub[0], currentAppInstance, currentAppInstance)
					// Add edges from hubSub to all resources that flow into remoteSub eg: pods, deployments, services, replicasets etc.
					query2 := db.SanitizeQuery("MATCH (hubSub {_uid: '%s'}) MATCH (remoteSub {_uid: '%s'})<-[]-(n) MERGE (n)-[r:hostedSub {_interCluster: true,app_instance: %d}]->(hubSub)", hubSubUID, remoteSub[0], currentAppInstance)
					// Connect all resources that flow into remoteSub with the hubsub's channel
					query3 := db.SanitizeQuery("MATCH (hubSub {_uid: '%s'})-[]->(chan) MATCH  (remoteSub {_uid: '%s'})<-[]-(n)  WHERE chan.kind = 'channel' MERGE (n)-[r:hostedSub {_interCluster: true,app_instance: %d}]->(chan)", hubSubUID, remoteSub[0], currentAppInstance)
					// Connect the remoteSub with the hubsub's application
					query4 := db.SanitizeQuery("MATCH (hubSub {_uid: '%s'})<-[]-(app) MATCH  (remoteSub {_uid: '%s'})  WHERE app.kind = 'application' MERGE (remoteSub)-[:deployedBy {_interCluster: true,app_instance: %d}]->(app)", hubSubUID, remoteSub[0], currentAppInstance)
					// Connect all resources that flow into remoteSub with the hubsub's application
					query5 := db.SanitizeQuery("MATCH (hubSub {_uid: '%s'})<-[]-(app) MATCH  (remoteSub {_uid: '%s'})<-[]-(n)  WHERE app.kind = 'application' MERGE (n)-[r:deployedBy {_interCluster: true,app_instance: %d}]->(app)", hubSubUID, remoteSub[0], currentAppInstance)
					// Connect resources that are connected to remoteSub with the hubsub's application - add check to avoid connecting application to itself
					query6 := db.SanitizeQuery("MATCH (hubSub {_uid: '%s'})<-[]-(app) MATCH  (remoteSub {_uid: '%s'})-[]->(n)  WHERE app.kind = 'application' AND n.kind <> 'application' AND n.kind <> 'subscription' MERGE (n)-[r:usedBy {_interCluster: true,app_instance: %d}]->(app)", hubSubUID, remoteSub[0], currentAppInstance)

					queries := [...]string{query1, query2, query3, query4, query5, query6}
					for _, query := range queries {
						_, err = db.Store.Query(query)
						if err != nil {
							glog.Errorf("Error %s : %s", query, err) //Logging error so that loop will continue
						}
					}
				}
			}
		}
		//Delete interclusters with other instance ids after all hub subscriptions are processed
		deleteOldInstance := db.SanitizeQuery("MATCH ()-[e {_interCluster:true}]->() WHERE (type(e)='hostedSub' OR type(e)='usedBy' OR type(e)='deployedBy') AND e.app_instance<>%d DELETE e", currentAppInstance)
		_, err = db.Store.Query(deleteOldInstance)
		if err != nil {
			return rg2.QueryResult{}, err
		}
	} else {
		//Delete interclusters because there is no remote subscriptions
		deleteOldInstance := db.SanitizeQuery("MATCH ()-[e {_interCluster:true}]->() WHERE (type(e)='hostedSub' OR type(e)='usedBy' OR type(e)='deployedBy') AND e.app_instance<>%d DELETE e", currentAppInstance)
		_, err = db.Store.Query(deleteOldInstance)
		if err != nil {
			return rg2.QueryResult{}, err
		}

	}
	previousAppInstance = currentAppInstance // Next iteration we dont want to use this ID
	// Record elapsed time
	elapsed := time.Since(start)
	glog.Info("Time taken to insert intercluster edges: ", elapsed)

	// Log a warning if it takes more than 100ms.
	if elapsed.Nanoseconds() > 100*1000*1000 {
		glog.Warningf("Intercluster edge deletion and re-creation took %s", elapsed)
	} else {
		glog.V(4).Infof("Intercluster edge deletion and re-creation took %s", elapsed)
	}
	return rg2.QueryResult{}, nil
}
