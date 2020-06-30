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


	// TODO: watch ManagedClusterInfo & ManagedCluster

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

	// TODO: abstract away duplicated for loop
	// Periodically check if the Managed Cluster Info resource exists
	go stopAndStartInformer("cluster.open-cluster-management.io/v1", managedClusterInformer)
	go stopAndStartInformer("internal.open-cluster-management.io/v1beta1", managedClusterInfoInformer)
}

func stopAndStartInformer(groupVersion string, informer cache.SharedIndexInformer){
    var stopper chan struct{}
    informerRunning := false

    for {
        _, err := config.KubeClient.ServerResourcesForGroupVersion(groupVersion)
        // we fail to fetch for some reason other than not found
        if err != nil && !isClusterMissing(err) {
            glog.Errorf("Cannot fetch resource list for %s, error message: %s ",groupVersion, err)
        } else {
            if informerRunning && isClusterMissing(err) {
                glog.Infof("Stopping cluster informer routine because %s resource not found.", groupVersion)
                stopper <- struct{}{}
                informerRunning = false
            } else if !informerRunning && !isClusterMissing(err) {
                glog.Infof("Starting cluster informer routine for cluster watch for % resource", groupVersion) // can I get resource string here?
                stopper = make(chan struct{})
                informerRunning = true
                go informer.Run(stopper)
            }
        }
        time.Sleep(time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond)
    }
}



func processClusterUpsert(obj interface{}, kubeClient *kubeClientset.Clientset) {
	glog.Info("Processing Cluster Upsert.")


	j, err := json.Marshal(obj.(*unstructured.Unstructured))
	if err != nil {
		glog.Warning("Error unmarshalling object from Informer in processClusterUpsert.")
	}

	// check which object we are using
	managedCluster := clusterv1.ManagedCluster{}
	managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
	var resource db.Resource

	// Objects should collide on UID and update rather than duplicate insert
	err = json.Unmarshal(j, &managedClusterInfo)
	if err != nil {
		glog.V(3).Info("Error on ManagedClusterInfo unmarshal, trying ManagedCluster unmarshal")
		err = json.Unmarshal(j, &managedCluster)
		if err != nil{
			glog.Warning("Failed to Unmarshal MangedCluster or ManagedclusterInfo")
		} else {
			glog.V(3).Info("Successful on ManagedClusterInfo unmarshal")
			resource = transformManagedCluster(&managedCluster)
		}
	} else {
		glog.V(3).Info("Successful on ManagedCluster unmarshal")
		resource = transformManagedClusterInfo(&managedClusterInfo)
	}

	// TODO: assert or unmarshal ?? ^^
	// and here or in transform cluster

	/*cluster, ok = obj.(*clusterregistry.Cluster) // ManagedClusterInfo will not assert as cluster ..
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}*/

	// https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/cluster/v1beta1/clusterinfo_types.go
	// https://github.com/open-cluster-management/api/blob/master/cluster/v1/types.go#L78
	// Old Definition https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/mcm/v1alpha1/clusterstatus_types.go



	resource.Properties["status"] = "" // TODO: Get the status.
	//glog.Info(resource)

	// Ensure that the cluster resource is still present before inserting into data store.
	/* assuming it's still there
	c, err := cluster.ClusterregistryV1alpha1().Clusters(cluster.Namespace).Get(cluster.Name, v1.GetOptions{})
	if err != nil {
		glog.Warningf("The cluster %s to add/update is not present anymore.", cluster.Name)
		delCluster(cluster)
		return
	}
	*/

	glog.V(2).Info("Updating Cluster resource by name in RedisGraph. ", resource)
	res, err := db.UpdateByName(resource)
	if err != nil {
		glog.Warning("Error on UpdateByName() ", err)
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

// Get most of our properties from Managed Cluster
// Use ManagedClusterInfo only for things that aren't available here 
// https://github.com/open-cluster-management/api/blob/master/cluster/v1/types.go#L78
func transformManagedCluster(managedCluster *clusterv1.ManagedCluster ) db.Resource {
	props := make(map[string]interface{})
	// TODO: confirm UIDs are the same and will actually collide
	// because we update by name, and the name "should be" the same, we should be good.

	props["name"] = managedCluster.GetName()  // must match managedClusterInfo
	props["kind"] = "Cluster"
	props["apigroup"] = "cluster.open-cluster-management.io" // use ManagedCluster apigroup as "main" apigroup
	props["created"] = managedCluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	// if cluster.Status is available
	capacity := managedCluster.Status.Capacity["cpu"] // pointer dereference required 
	props["cpu"], _ = capacity.AsInt64()
	capacity = managedCluster.Status.Capacity["memory"]
	props["memory"] = capacity.String()
	props["kubernetesVersion"] = managedCluster.Status.Version.Kubernetes
	// props["klusterletVersion"] = clusterStatus.Spec.KlusterletVersion // not in ManagedCluster object

	resource := db.Resource{
		Kind:           "Cluster",
		UID:            string("cluster_" + managedCluster.GetUID()),
		Properties:     props,
		ResourceString: "managedclusters", // Needed for rbac, map to real cluster resource.
	}
	// glog.Info(resource)
	return resource

}

// get other properties from managedClusterInfo
// https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/cluster/v1beta1/clusterinfo_types.go#L24
func transformManagedClusterInfo(managedClusterInfo *clusterv1beta1.ManagedClusterInfo ) db.Resource {
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

	//fmt.Printf("\n\ninfo:\n\n%+v\n\n", cluster)
	//glog.Infof("NodeList %s", cluster.Status.NodeList)

/*
	// get capacity from Managed Cluster instead
	// Sum Capacity of all nodes 
	var cpu_sum int64
	var memory_sum int64
	for _, node  := range cluster.Status.NodeList{
		cpu := node.Capacity["cpu"]
		tmp, _ := cpu.AsInt64()
		cpu_sum += tmp

		memory := node.Capacity["memory"]
		tmp, _ = memory.AsInt64()
		memory_sum += tmp
	}

	props["cpu"] = cpu_sum
	props["memory"] = strconv.FormatInt(memory_sum,10)
	//glog.Info("Conditions: %s", cluster.Status.Conditions)
*/
	var HubAcceptedManagedCluster, ManagedClusterConditionAvailable, ManagedClusterJoined string
	for _, condition := range managedClusterInfo.Status.Conditions {
		if condition.Type == "HubAcceptedManagedCluster" {
			HubAcceptedManagedCluster = string(condition.Status)
		}
		if condition.Type == "ManagedClusterConditionAvailable" {
			ManagedClusterConditionAvailable = string(condition.Status)
		}
		if condition.Type == "ManagedClusterJoined" {
			ManagedClusterJoined = string(condition.Status)
		}

	}

	props["HubAcceptedManagedCluster"] = HubAcceptedManagedCluster
	props["ManagedClusterConditionAvailable"] = ManagedClusterConditionAvailable
	props["ManagedClusterJoined"] = ManagedClusterJoined

	props["consoleURL"] = managedClusterInfo.Status.ConsoleURL // not being populated yet 
	props["loggingEndpoint"] = managedClusterInfo.Status.LoggingEndpoint.String()
	glog.Info("logging endpoint: ", props["loggingEndpoint"])

/* 
	props["nodes"] = int64(0)
	nodes, ok := clusterStatus.Spec.Capacity["nodes"]
	if ok {
		props["nodes"], _ = nodes.AsInt64()
	}

	props["storage"] = ""
	storage, ok := clusterStatus.Spec.Capacity["storage"]
	if ok {
		props["storage"] = storage.String()
	}
*/

	resource := db.Resource{
		Kind:           "Cluster",
		//UID:            string("cluster_" + managedClusterInfo.GetUID()),
		Properties:     props,
		ResourceString: "managedclusters", // Needed for rbac, map to real cluster resource.
	}
	// glog.Info(resource)
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


