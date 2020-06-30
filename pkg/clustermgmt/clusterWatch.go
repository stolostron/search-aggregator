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
	"fmt"
	"strconv"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	clusterv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/cluster/v1beta1"
	//clusterv1 "github.com/open-cluster-management/api/cluster/v1"
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
	//go stopAndStartInformer("cluster.open-cluster-management.io/v1", managedClusterInformer)
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
		glog.Warning("Error on ManagedCluster marshal.")
	}

	managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
	err = json.Unmarshal(j, &managedClusterInfo)
	if err != nil {
		glog.Warning("Error on ManagedClusterInfo unmarshal.")
	}

	/*cluster, ok = obj.(*clusterregistry.Cluster) // ManagedClusterInfo will not assert as cluster ..
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}*/

	// https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/cluster/v1beta1/clusterinfo_types.go
	// https://github.com/open-cluster-management/api/blob/master/cluster/v1/types.go#L78
	// Old Definition https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/mcm/v1alpha1/clusterstatus_types.go

	resource := transformCluster(&managedClusterInfo)
	resource.Properties["status"] = "" // TODO: Get the status.
	glog.Info(resource)
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

func transformCluster(cluster *clusterv1beta1.ManagedClusterInfo ) db.Resource {
	props := make(map[string]interface{})

	// get these fields from ManagedCluster object
	props["name"] = cluster.GetName()
	props["kind"] = "Cluster"
	props["apigroup"] = "internal.open-cluster-management.io"
	props["created"] = cluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	if cluster.GetLabels() != nil {
		var labelMap map[string]interface{}
		clusterLabels, _ := json.Marshal(cluster.GetLabels())
		err := json.Unmarshal(clusterLabels, &labelMap)
		// Unmarshaling labels to map[string]interface{}, so that it will be accounted for while encoding properties
		// This was getting skipped before as map[string]string was not handled in switch case encode#77
		if err == nil {
			props["label"] = labelMap
		}
	}


	fmt.Printf("\n\ninfo:\n\n%+v\n\n", cluster)
	glog.Infof("NodeList %s", cluster.Status.NodeList)

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
	glog.Info("Conditions: %s", cluster.Status.Conditions)

	var stat1, stat2, stat3 string 
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == "HubAcceptedManagedCluster" {
			stat1 = string(condition.Status)
		}
		if condition.Type == "ManagedClusterConditionAvailable" {
			stat2 = string(condition.Status)
		}
		if condition.Type == "ManagedClusterJoined" {
			stat3 = string(condition.Status)
		}

	}

	props["HubAcceptedManagedCluster"] = stat1
	props["ManagedClusterConditionAvailable"] = stat2
	props["ManagedClusterJoined"] = stat3

/*
	props["HubAcceptedManagedCluster"], _ = strconv.ParseBool(strings.ToLower(stat1))
	props["ManagedClusterConditionAvailable"], _ = strconv.ParseBool(strings.ToLower(stat2))
	props["ManagedClusterJoined"], _ = strconv.ParseBool(strings.ToLower(stat3))
*/
	glog.Info("consuleurl:", cluster.Status.ConsoleURL)
	props["consoleURL"] = cluster.Status.ConsoleURL // not in ManagedClusterInfo
/*	capacity := clusterStatus.Capacity["cpu"] // pointer dereference required 
	props["cpu"], _ = capacity.asint64()
	capacity = clusterStatus.Capacity["memory"]
	props["memory"] = capacity.String()
	props["kubernetesVersion"] = cluster.Status.Version.Kubernetes
	props["klusterletVersion"] = clusterStatus.Spec.KlusterletVersion // not in ManagedCluster object
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
	glog.Info(props) 

	return db.Resource{
		Kind:           "Cluster",
		UID:            string("cluster_" + cluster.GetUID()),
		Properties:     props,
		ResourceString: "managedclusters", // Needed for rbac, map to real cluster resource.
	}

}

// Deletes a cluster resource and all resourcces from the cluster.
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


