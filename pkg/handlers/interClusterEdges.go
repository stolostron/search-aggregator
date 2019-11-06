/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package handlers

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"

	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/config"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
	rg "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

var ApplicationLastUpdated time.Time
var PolicyLastUpdated time.Time

func getApplicationUpdateTime() time.Time {
	return ApplicationLastUpdated
}

func getPolicyUpdateTime() time.Time {
	return PolicyLastUpdated
}

// runs all the specific inter-cluster relationships we want to connect
func BuildInterClusterEdges() {
	var tranforms = []struct {
		transfrom   func() (rg.QueryResult, error)
		description string
		getUpdate   func() time.Time
	}{
		{
			buildSubscriptions,
			"connecting subscription edges",
			getApplicationUpdateTime,
		},
		{
			buildPolicyEdges,
			"connecting policy edges",
			getPolicyUpdateTime,
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

func getUIDsForSubscriptions() (rg.QueryResult, error) {
	query := "MATCH (n {kind: 'subscription'})  RETURN n._uid"
	uidResults, err := db.Store.Query(query)
	return uidResults, err
}

func buildPolicyEdges() (rg.QueryResult, error) {

	// Record start time
	start := time.Now()
	policiesWithParent := make(map[string]bool)
	//Get all the Nodes connected to a policy with interCluter true - store the UIDs
	hub_query := "MATCH (n)-[:ownedBy {_interCluster: true}]->( {kind: 'policy'}) RETURN n._uid"
	connectedPolicies, herr := db.Store.Query(hub_query)
	if herr != nil {
		return rg.QueryResult{}, herr
	}
	if len(connectedPolicies.Results) > 1 { //  If there is any result
		for _, hubPolicy := range connectedPolicies.Results[1:] {
			policiesWithParent[hubPolicy[0]] = true
		}

	}
	//Get all the polices from Remote clusters , if the UID is in the map above - we alredy have an edge , skip it - else create an edge
	mc_query := "MATCH (n{kind:'policy'}) where n.cluster!='local-cluster' AND exists(n._parentPolicy)=true AND n.apigroup='policy.mcm.ibm.com' return  n._parentPolicy,n._uid"
	mcPolicies, merr := db.Store.Query(mc_query)
	if merr != nil {
		return rg.QueryResult{}, merr
	}
	if len(mcPolicies.Results) > 1 { //  If there is any result
		for _, mcPolicy := range mcPolicies.Results[1:] {
			if mcPolicy[0] != "" && mcPolicy[1] != "" { // check if its valid policy and uid
				_, ok := policiesWithParent[mcPolicy[1]]
				if ok {
					continue //we already have the uid connected to a parent policy
				} else { // this parent is present in our Hub
					remoteUID := mcPolicy[1]
					hubPolicyInfo := strings.Split(mcPolicy[0], "/") // index 0 namespace , index 1 policy name
					//create Edge
					createQuery := fmt.Sprintf("MATCH (parent: {kind:'policy' cluster: 'local-cluster' namespace: '%s' name: '%s'}), (policy: {_uid: '%s}) CREATE (policy)-[:ownedBy {_interCluster: true}]->(parent)", hubPolicyInfo[0], hubPolicyInfo[1], remoteUID)
					glog.V(4).Infof("Policy edge to be created %s : ", createQuery)
					_, crerr := db.Store.Query(createQuery)
					if crerr != nil {
						glog.V(4).Infof("Policy edge creation failed  %s : ", createQuery)
						continue // if there is an error continue with next Policy
					}
				}
			}

		}
	} else {
		//there is no policy in managed - return with out any new edges
		return rg.QueryResult{}, nil
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

}

func buildSubscriptions() (rg.QueryResult, error) {
	// Record start time
	start := time.Now()
	query := "MATCH	()-[e]->() WHERE e._interCluster=true AND (type(e)='hostedSub' OR type(e)='usedBy' OR type(e)='deployedBy') DELETE e"
	_, err := db.Store.Query(query)
	if err != nil {
		return rg.QueryResult{}, err
	}

	// list of remote subscriptions
	query = "MATCH (n {kind: 'subscription'}) WHERE n.cluster != 'local-cluster' RETURN n._uid, n._hostingSubscription"
	remoteSubscriptions, err := db.Store.Query(query)
	if err != nil {
		return rg.QueryResult{}, err
	}

	if len(remoteSubscriptions.Results) > 1 { //Check if any results are returned
		// list of hub subscriptions
		query = "MATCH (n {kind: 'subscription', cluster: 'local-cluster'}) RETURN n._uid, n.namespace, n.name"
		hubSubscriptons, err := db.Store.Query(query)
		if err != nil {
			return rg.QueryResult{}, err
		}

		for _, remoteSub := range remoteSubscriptions.Results[1:] {
			if remoteSub[1] != "" {
				// parse the hosting subscription into name and namespace
				hostingSub := strings.Split(remoteSub[1], "/")
				if len(hostingSub) != 2 {
					msg := fmt.Sprintf("found incorrect hostingSubscription format when parsing: %s", remoteSub[1])
					glog.Errorf("Error %s : %s", rg.QueryResult{}, errors.New(msg)) //Logging error so that loop won't exit because of formatting error
					continue                                                        //Continue with the next remote subscription
				}
				namespace := hostingSub[0]
				name := hostingSub[1]

				// check if it is in the hub subs
				for _, hubSub := range hubSubscriptons.Results[1:] {
					if hubSub[1] == namespace && hubSub[2] == name {
						//TODO: For the subscription model, all intercluster edges are named as 'hostedSub {_interCluster: true}'. Change this to relevant names in future
						//To add edges from hubSub to all resources connected to the remoteSub (bidirectional) - incoming edges and outgoing edges
						// Add an edge between remoteSub and hubSub. Add edges from hubSub to all resources the remoteSub connects to
						query1 := fmt.Sprintf("MATCH (hubSub {_uid: '%s'}), (remoteSub {_uid: '%s'})-[]->(n) CREATE (remoteSub)-[:hostedSub {_interCluster: true}]->(hubSub), (n)-[:hostedSub {_interCluster: true}]->(hubSub)", hubSub[0], remoteSub[0])
						// Add edges from hubSub to all resources that flow into remoteSub eg: pods, deployments, services, replicasets etc.
						query2 := fmt.Sprintf("MATCH (hubSub {_uid: '%s'}), (remoteSub {_uid: '%s'})<-[]-(n) CREATE (n)-[r:hostedSub {_interCluster: true}]->(hubSub)", hubSub[0], remoteSub[0])
						// Connect all resources that flow into remoteSub with the hubsub's channel
						query3 := fmt.Sprintf("MATCH (hubSub {_uid: '%s'})-[]->(chan) ,  (remoteSub {_uid: '%s'})<-[]-(n)  WHERE chan.kind = 'channel' CREATE (n)-[r:hostedSub {_interCluster: true}]->(chan)", hubSub[0], remoteSub[0])
						// Connect the remoteSub with the hubsub's application
						query4 := fmt.Sprintf("MATCH (hubSub {_uid: '%s'})<-[]-(app) ,  (remoteSub {_uid: '%s'})  WHERE app.kind = 'application' CREATE (remoteSub)-[:deployedBy {_interCluster: true}]->(app)", hubSub[0], remoteSub[0])
						// Connect all resources that flow into remoteSub with the hubsub's application
						query5 := fmt.Sprintf("MATCH (hubSub {_uid: '%s'})<-[]-(app) ,  (remoteSub {_uid: '%s'})<-[]-(n)  WHERE app.kind = 'application' CREATE (n)-[r:deployedBy {_interCluster: true}]->(app)", hubSub[0], remoteSub[0])
						// Connect resources that are connected to remoteSub with the hubsub's application - add check to avoid connecting application to itself
						query6 := fmt.Sprintf("MATCH (hubSub {_uid: '%s'})<-[]-(app) ,  (remoteSub {_uid: '%s'})-[]->(n)  WHERE app.kind = 'application' AND n.kind != 'application' CREATE (n)-[r:usedBy {_interCluster: true}]->(app)", hubSub[0], remoteSub[0])

						queries := [...]string{query1, query2, query3, query4, query5, query6}
						for _, query := range queries {
							_, err = db.Store.Query(query)
							if err != nil {
								glog.Errorf("Error %s : %s", rg.QueryResult{}, err) //Logging error so that loop will continue
							}
						}
					}
				}
			}
		}
	}
	// Record elapsed time
	elapsed := time.Since(start)
	// Log a warning if it takes more than 100ms.
	if elapsed.Nanoseconds() > 100*1000*1000 {
		glog.Warningf("Intercluster edge deletion and re-creation took %s", elapsed)
	} else {
		glog.V(4).Infof("Intercluster edge deletion and re-creation took %s", elapsed)
	}

	return rg.QueryResult{}, nil
}
