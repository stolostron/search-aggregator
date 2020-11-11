/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.

Copyright (c) 2020 Red Hat, Inc.
*/

package clustermgmt

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/cluster/v1beta1"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// Watches ManagedCluster and ManagedClusterInfo objects and updates
// the search graph with a Cluster pseudo node.
func WatchClusters() {
	glog.Info("Begin ClusterWatch routine")

	dynamicClient := config.GetDynamicClient()
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 60*time.Second)

	// Create GVR for ManagedCluster and ManagedClusterInfo
	managedClusterGvr, _ := schema.ParseResourceArg("managedclusters.v1.cluster.open-cluster-management.io")
	managedClusterInfoGvr, _ := schema.ParseResourceArg("managedclusterinfos.v1beta1.internal.open-cluster-management.io")

	//Create Informers for ManagedCluster and ManagedClusterInfo
	managedClusterInformer := dynamicFactory.ForResource(*managedClusterGvr).Informer()
	managedClusterInfoInformer := dynamicFactory.ForResource(*managedClusterInfoGvr).Informer()

	// Create handlers for events
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processClusterUpsert(obj)
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			processClusterUpsert(next)
		},
		DeleteFunc: func(obj interface{}) {
			processClusterDelete(obj)
		},
	}

	// Add Handlers to both Informers
	managedClusterInformer.AddEventHandler(handlers)
	managedClusterInfoInformer.AddEventHandler(handlers)

	// Periodically check if the ManagedCluster/ManagedClusterInfo resource exists
	go stopAndStartInformer("cluster.open-cluster-management.io/v1", managedClusterInformer)
	go stopAndStartInformer("internal.open-cluster-management.io/v1beta1", managedClusterInfoInformer)
}

// Stop and Start informer according to Rediscover Rate
func stopAndStartInformer(groupVersion string, informer cache.SharedIndexInformer) {
	var stopper chan struct{}
	informerRunning := false

	for {
		_, err := config.GetKubeClient().ServerResourcesForGroupVersion(groupVersion)
		// we fail to fetch for some reason other than not found
		if err != nil && !isClusterMissing(err) {
			glog.Errorf("Cannot fetch resource list for %s, error message: %s ", groupVersion, err)
		} else {
			if informerRunning && isClusterMissing(err) {
				glog.Infof("Stopping cluster informer routine because %s resource not found.", groupVersion)
				stopper <- struct{}{}
				informerRunning = false
			} else if !informerRunning && !isClusterMissing(err) {
				glog.Infof("Starting cluster informer routine for cluster watch for %s resource", groupVersion)
				stopper = make(chan struct{})
				informerRunning = true
				go informer.Run(stopper)
			}
		}
		time.Sleep(time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond)
	}
}

var mux sync.Mutex

func processClusterUpsert(obj interface{}) {
	// Lock so only one goroutine at a time can access add a cluster.
	// Helps to eliminate duplicate entries.
	mux.Lock()
	defer mux.Unlock()
	j, err := json.Marshal(obj.(*unstructured.Unstructured))
	if err != nil {
		glog.Warning("Error unmarshalling object from Informer in processClusterUpsert.")
	}

	// We update by name, and the name *should be* the same for a given cluster in either object
	// Objects from a given cluster collide and update rather than duplicate insert
	// Unmarshall either ManagedCluster or ManagedClusterInfo
	// check which object we are using

	var resource db.Resource
	switch obj.(*unstructured.Unstructured).GetKind() {
	case "ManagedCluster":
		managedCluster := clusterv1.ManagedCluster{}
		err = json.Unmarshal(j, &managedCluster)
		if err != nil {
			glog.Warning("Failed to Unmarshal MangedCluster", err)
		}
		resource = transformManagedCluster(&managedCluster)
	case "ManagedClusterInfo":
		managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
		err = json.Unmarshal(j, &managedClusterInfo)
		if err != nil {
			glog.Warning("Failed to Unmarshal ManagedclusterInfo", err)
		}
		resource = transformManagedClusterInfo(&managedClusterInfo)
	default:
		glog.Warning("ClusterWatch received unknown kind.", obj.(*unstructured.Unstructured).GetKind())
	}

	// Upsert (attempt update, attempt insert on failure)
	glog.V(2).Info("Updating Cluster resource by name in RedisGraph. ", resource)
	res, err, alreadySET := db.UpdateByName(resource)
	if err != nil {
		glog.Warning("Error on UpdateByName() ", err)
	}

	if alreadySET {
		glog.V(4).Infof("Node for cluster %s already exist on DB.", resource.Properties["name"])
		return
	}

	if db.IsGraphMissing(err) || (err == nil && !db.IsPropertySet(res)) {
		glog.Infof("Node for cluster %s does not exist, inserting it.", resource.Properties["name"])
		_, _, err = db.Insert([]*db.Resource{&resource}, "")
		if err != nil {
			glog.Error("Error adding Cluster node with error: ", err)
			return
		}
	}

	// If a cluster is offline we remove the resources from that cluster, but leave the cluster resource object.
	/*if resource.Properties["status"] == "offline" {
		glog.Infof("Cluster %s is offline, removing cluster resources from datastore.", cluster.GetName())
		delClusterResources(cluster)
	}*/

}

func isClusterMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "could not find the requested resource")
}

// Transform ManagedCluster object into db.Resource suitable for insert into redis
func transformManagedCluster(managedCluster *clusterv1.ManagedCluster) db.Resource {
	// https://github.com/open-cluster-management/api/blob/master/cluster/v1/types.go#L78
	// We use ManagedCluster as the primary source of information
	// Properties duplicated between this and ManagedClusterInfo are taken from ManagedCluster
	props := make(map[string]interface{})

	if managedCluster.GetLabels() != nil {
		var labelMap map[string]interface{}
		clusterLabels, _ := json.Marshal(managedCluster.GetLabels())
		err := json.Unmarshal(clusterLabels, &labelMap)
		// Unmarshaling labels to map[string]interface{}, so that it will be accounted for while encoding properties
		// This was getting skipped before as map[string]string was not handled in switch case encode#77
		if err == nil {
			props["label"] = labelMap
		}
	}

	props["apigroup"] = "internal.open-cluster-management.io" // maps rbac to ManagedClusterInfo
	props["kind"] = "Cluster"
	props["name"] = managedCluster.GetName()              // must match ManagedClusterInfo
	props["_clusterNamespace"] = managedCluster.GetName() // maps to the namespace of ManagedClusterInfo
	props["created"] = managedCluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	cpuCapacity := managedCluster.Status.Capacity["cpu"]
	props["cpu"], _ = cpuCapacity.AsInt64()
	memCapacity := managedCluster.Status.Capacity["memory"]
	props["memory"] = memCapacity.String()
	props["kubernetesVersion"] = managedCluster.Status.Version.Kubernetes

	for _, condition := range managedCluster.Status.Conditions {
		props[condition.Type] = string(condition.Status)
	}

	resource := db.Resource{
		Kind:           "Cluster",
		UID:            string("cluster__" + managedCluster.GetName()),
		Properties:     props,
		ResourceString: "managedclusterinfos", // Maps rbac to ManagedClusterInfo
	}

	return resource
}

// Transform ManagedClusterInfo object into db.Resource suitable for insert into redis
func transformManagedClusterInfo(managedClusterInfo *clusterv1beta1.ManagedClusterInfo) db.Resource {
	// https://github.com/open-cluster-management/multicloud-operators-foundation/
	//    blob/master/pkg/apis/internal.open-cluster-management.io/v1beta1/clusterinfo_types.go
	props := make(map[string]interface{})

	props["kind"] = "Cluster"
	props["name"] = managedClusterInfo.GetName()
	props["_clusterNamespace"] = managedClusterInfo.GetNamespace() // Needed for rbac mapping.
	props["apigroup"] = "internal.open-cluster-management.io"      // Maps rbac to ManagedClusterInfo

	props["consoleURL"] = managedClusterInfo.Status.ConsoleURL
	props["nodes"] = int64(len(managedClusterInfo.Status.NodeList))

	resource := db.Resource{
		Kind:           "Cluster",
		UID:            string("cluster__" + managedClusterInfo.GetName()),
		Properties:     props,
		ResourceString: "managedclusterinfos", // Maps rbac to ManagedClusterInfo.
	}

	return resource
}

// Deletes a cluster resource and all resources from the cluster.
func processClusterDelete(obj interface{}) {
	glog.Info("Processing Cluster Delete.")

	clusterName := obj.(*unstructured.Unstructured).GetName()
	clusterUID := string("cluster__" + obj.(*unstructured.Unstructured).GetName())
	glog.Infof("Deleting Cluster resource %s and all resources from the cluster. UID %s", clusterName, clusterUID)

	_, err := db.Delete([]string{clusterUID})
	if err != nil {
		glog.Error("Error deleting Cluster node with error: ", err)
	}
	delClusterResources(clusterUID, clusterName)
}

// Removes all the resources for a cluster, but doesn't remove the Cluster resource object.
func delClusterResources(clusterUID string, clusterName string) {
	_, err := db.DeleteCluster(clusterName)
	if err != nil {
		glog.Error("Error deleting current resources for cluster: ", err)
	} else {
		db.DeleteClustersCache(clusterUID)
	}
}
