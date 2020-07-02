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
	"time"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	clusterv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/cluster/v1beta1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// Watches ManagedCluster and ManagedClusterInfo objects and updates the search cache.
func WatchClusters() {
	glog.Info("Begin ClusterWatch routine")

	// Initialize the dynamic client, used for CRUD operations on arbitrary k8s resources.
	hubClientConfig, err := config.InitClient()
	if err != nil {
		glog.Info("Unable to create ClusterWatch clientset ", err)
	}
	dynamicClientset, err := dynamic.NewForConfig(hubClientConfig)
	if err != nil {
		glog.Warning("cannot construct dynamic client for cluster watch from config: ", err)
	}
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientset, 60*time.Second)

	// Create GVR for ManagedCluster and ManagedClusterInfo 
	managedClusterGvr, _ := schema.ParseResourceArg("managedclusters.v1.cluster.open-cluster-management.io")
	managedClusterInfoGvr, _ := schema.ParseResourceArg("managedclusterinfos.v1beta1.internal.open-cluster-management.io")

	//Create Informers for ManagedCluster and ManagedClusterInfo
	managedClusterInformer := dynamicFactory.ForResource(*managedClusterGvr).Informer()
	managedClusterInfoInformer := dynamicFactory.ForResource(*managedClusterInfoGvr).Informer()

	// Create handlers for events
	handlers :=	 cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processClusterUpsert(obj, config.KubeClient)
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			processClusterUpsert(next, config.KubeClient)
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
func stopAndStartInformer(groupVersion string, informer cache.SharedIndexInformer){
    var stopper chan struct{}
    informerRunning := false

    for {
        _, err := config.KubeClient.ServerResourcesForGroupVersion(groupVersion)
        // we fail to fetch for some reason other than not found
        if err != nil && !isClusterMissing(err) {
            glog.Errorf("Cannot fetch resource list for %s, error message: %s ", groupVersion, err)
        } else {
            if informerRunning && isClusterMissing(err) {
                glog.Infof("Stopping cluster informer routine because %s resource not found.", groupVersion)
                stopper <- struct{}{}
                informerRunning = false
            } else if !informerRunning && !isClusterMissing(err) {
				// can I get resource string here?
                glog.Infof("Starting cluster informer routine for cluster watch for %s resource", groupVersion)
                stopper = make(chan struct{})
                informerRunning = true
                go informer.Run(stopper)
            }
        }
        time.Sleep(time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond)
    }
}

func processClusterUpsert(obj interface{}, kubeClient *kubeClientset.Clientset) {
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
		glog.Info("ManagedCluster")
		managedCluster := clusterv1.ManagedCluster{}
		err = json.Unmarshal(j, &managedCluster)
		resource = transformManagedCluster(&managedCluster)
	case "ManagedClusterInfo":
		glog.Info("ManagedClusterInfo")
		managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
		err = json.Unmarshal(j, &managedClusterInfo)
		resource = transformManagedClusterInfo(&managedClusterInfo)
	default:
		glog.Info("Unknown kind.", obj.(*unstructured.Unstructured).GetKind())
	}

	resource.Properties["status"] = "" // TODO: Get the status.  

	// TODO: confirm that resource still exists before updating
	// Ensure that the cluster resource is still present before inserting into data store.
	/*
	c, err := cluster.ClusterregistryV1alpha1().Clusters(cluster.Namespace).Get(cluster.Name, v1.GetOptions{})
	if err != nil {
		glog.Warningf("The cluster %s to add/update is not present anymore.", cluster.Name)
		delCluster(cluster)
		return
	}
	*/

	// Upsert (attempt update, attempt insert on failure)
	glog.V(2).Info("Updating Cluster resource by name in RedisGraph. ", resource)
	res, err, alreadySET := db.UpdateByName(resource)
	if err != nil {
		glog.Warning("Error on UpdateByName() ", err)
	}

	if alreadySET {
		return
	}

	if db.IsGraphMissing(err) || !db.IsPropertySet(res) /*&& (c.Name == cluster.Name)*/ {
		glog.Info("Cluster graph/key object does not exist, inserting new object")
		_, _, err = db.Insert([]*db.Resource{&resource}, "")
		if err != nil {
			glog.Error("Error adding Cluster kind with error: ", err)
			return
		}
	} else if err != nil { // generic error not known above
		glog.Error("Error updating Cluster kind with errors: ", err)
		return
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
func transformManagedCluster(managedCluster *clusterv1.ManagedCluster ) db.Resource {
	// https://github.com/open-cluster-management/api/blob/master/cluster/v1/types.go#L78
	// We use ManagedCluster as the primary source of information
	// Properties duplicated between this and ManagedClusterInfo are taken from ManagedCluster
	props := make(map[string]interface{})

	props["name"] = managedCluster.GetName()  // must match managedClusterInfo
	props["kind"] = "Cluster"
	props["apigroup"] = "cluster.open-cluster-management.io" // use ManagedCluster apigroup as "main" apigroup
	props["created"] = managedCluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	if &managedCluster.Status != nil { // managedCluster.Status is optional 
		capacity := managedCluster.Status.Capacity["cpu"] // pointer dereference required 
		props["cpu"], _ = capacity.AsInt64()
		capacity = managedCluster.Status.Capacity["memory"]
		props["memory"] = capacity.String()
		props["kubernetesVersion"] = managedCluster.Status.Version.Kubernetes
	}

	resource := db.Resource{
		Kind:           "Cluster",
		UID:            string("cluster_" + managedCluster.GetUID()),
		Properties:     props,
		ResourceString: "managedclusters", // Needed for rbac, map to real cluster resource.
	}

	return resource
}

// Transform ManagedCluster object into db.Resource suitable for insert into redis 
func transformManagedClusterInfo(managedClusterInfo *clusterv1beta1.ManagedClusterInfo ) db.Resource {
	// https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/cluster/v1beta1/clusterinfo_types.go#L24
	props := make(map[string]interface{})

	// get these fields from ManagedClusterInfo object 
	props["name"] = managedClusterInfo.GetName()
	props["kind"] = "Cluster"
	// props["apigroup"] = "internal.open-cluster-management.io"
	// props["created"] = cluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	if managedClusterInfo.GetLabels() != nil {
		var labelMap map[string]interface{}
		clusterLabels, _ := json.Marshal(managedClusterInfo.GetLabels())
		err := json.Unmarshal(clusterLabels, &labelMap)
		// Unmarshaling labels to map[string]interface{}, so that it will be accounted for while encoding properties
		// This was getting skipped before as map[string]string was not handled in switch case encode#77
		if err == nil {
			props["label"] = labelMap
		}
	}

	if &managedClusterInfo.Status != nil { // managedCluster.Status is optional 
		props["nodes"] = len(managedClusterInfo.Status.NodeList)
		for _, condition := range managedClusterInfo.Status.Conditions {
			props[condition.Type] = string(condition.Status)
		}
		props["consoleURL"] = managedClusterInfo.Status.ConsoleURL // not being populated yet 
	}

	resource := db.Resource{
		Kind:           "Cluster",
		//UID:            string("cluster_" + managedClusterInfo.GetUID()),
		Properties:     props,
		ResourceString: "managedclusters", // Needed for rbac, map to real cluster resource.
	}

	return resource
}

// Deletes a cluster resource and all resources from the cluster.
func processClusterDelete(obj interface{}) {
	glog.Info("Processing Cluster Delete.")

	j, err := json.Marshal(obj.(*unstructured.Unstructured))
	if err != nil {
		glog.Error("Failed to marshall ManagedCluster on processDeleteCluster")
	}

	managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
	err = json.Unmarshal(j, &managedClusterInfo)
	if err != nil {
		glog.Error("Failed to unmarshall ManagedClusterInfo on processDeleteCluster")
	}
	clusterName := managedClusterInfo.GetName()
	clusterUID := string("cluster_" + managedClusterInfo.GetUID())
	glog.Infof("Deleting Cluster resource %s and all resources from the cluster. UID %s", clusterName, clusterUID)

	_, err = db.Delete([]string{clusterUID})
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
