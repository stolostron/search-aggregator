/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
// Copyright (c) 2020 Red Hat, Inc.
// begin transition
package clustermgmt

import (
	"strings"
	"sync"
	"time"

	//v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"encoding/json"
    "github.com/open-cluster-management/search-aggregator/pkg/config"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"github.com/golang/glog"
	"k8s.io/client-go/tools/cache"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	//clusterv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/cluster/v1beta1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

var statClusterMap map[string]bool // Install/uninstall jobs might take some time to start - if cluster is in unknown status, we use this map to restart the clusterInformer in order to update cluster status
var statClusterMapMutex = sync.RWMutex{}

//ClusterStat struct stores all resources needed to calculate status of the cluster
type ClusterStat struct {
	cluster       *clusterregistry.Cluster
	clusterStatus *clusterv1.ManagedClusterStatus
}

// WatchClusters watches k8s cluster and clusterstatus objects and updates the search cache.
func WatchClusters() {
	glog.Info("Begin Watch Clusters Routine")
	// first time we call this // kubeclient now in var config.KubeClient *kubeClientset.Clientset 
	hubClientConfig, err := config.InitClient()
	if err != nil {
		glog.Info("Unable to create clientset ", err)
	}


	statClusterMap = map[string]bool{}
	var stopper chan struct{}
	informerRunning := false


    // Initialize the dynamic client, used for CRUD operations on arbitrary k8s resources
	dynamicClientset, err := dynamic.NewForConfig(hubClientConfig)
    if err != nil {
        // not fatal glog.fatal("cannot construct dynamic client from config: ", err)
    }

	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientset, 0)
	// gvr, _ := schema.ParseResourceArg("managedclusterinfos.v1beta1.internal.open-cluster-management.io")	
	gvr, _ := schema.ParseResourceArg("managedclusters.v1.cluster.open-cluster-management.io")
	clusterInformer := dynamicFactory.ForResource(*gvr).Informer() // for ManagedCluster

	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			glog.Info("received add event ")
			processClusterUpsert(obj, config.KubeClient)
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			glog.Info("received update event", err)
			processClusterUpsert(next, config.KubeClient)
		},
		DeleteFunc: func(obj interface{}) {
			glog.Info("received delete event")
			/*cluster, ok := obj.(*clusterregistry.Cluster)
			if !ok {
				glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
				return
			}
			delCluster(cluster)*/
		},
	})




	/* mcmClient, err := config.InitClient()
	if err != nil {
		glog.Info("Unable to create clientset ", err)
	}
	statClusterMap = map[string]bool{}
	var stopper chan struct{}
	informerRunning := false

	clusterFactory := informers.NewSharedInformerFactory(clusterClient, 0)
	clusterInformer := clusterFactory.Clusterregistry().V1alpha1().Clusters().Informer()
	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processClusterUpsert(obj, mcmClient)
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			processClusterUpsert(next, mcmClient)
		},
		DeleteFunc: func(obj interface{}) {
			cluster, ok := obj.(*clusterregistry.Cluster)
			if !ok {
				glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
				return
			}
			delCluster(cluster)
		},
	})
	*/
	// periodically check if the cluster resource exists and start/stop the informer accordingly
	for {
		_, err := config.KubeClient.ServerResourcesForGroupVersion("clusterregistry.k8s.io/v1alpha1")
		// we fail to fetch for some reason other than not found
		if err != nil && !isClusterMissing(err) {
			glog.Error("Cannot fetch resource list for clusterregistry.k8s.io/v1alpha1: ", err)
		} else {
			if isClusterMissing(err) && informerRunning {
				glog.Info("Stopping cluster informer routine because clusterregistry resource not found")
				stopper <- struct{}{}
				informerRunning = false
			} else if !isClusterMissing(err) && !informerRunning {
				glog.Info("Starting cluster informer routine for cluster watch")
				stopper = make(chan struct{})
				informerRunning = true
				go clusterInformer.Run(stopper)
			} else {
				//If any clusters are in `unknown` status, restart the informers - this is a workaround instead of watching the install, uninstall jobs for a cluster
				//TODO: Remove this and get cluster status from cluster object using issue (open-cluster-management/backlog#1518)
				if len(statClusterMap) > 0 {
					glog.V(2).Info("Restarting cluster informer routine for cluster watch")
					stopper <- struct{}{}
					stopper = make(chan struct{})
					informerRunning = true
					go clusterInformer.Run(stopper)
				}
			}
		}

		time.Sleep(time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond)
	}
	/**/
}

func processClusterUpsert(obj interface{}, mcmClient *kubeClientset.Clientset) {
	glog.Infof("Processing Cluster Upsert")

	var err error

	j, err := json.Marshal(obj.(*unstructured.Unstructured))
	if err != nil {
		panic(err) // Will be caught by handleRoutineExit
	}

	managedCluster :=  clusterv1.ManagedCluster{}
	err = json.Unmarshal(j, &managedCluster)
	if err != nil {
		panic(err) // Will be caught by handleRoutineExit
	}
	glog.Infof("Managed Cluster Info as string: %s \n", managedCluster) 

	/*cluster, ok = obj.(*clusterregistry.Cluster) // ManagedClusterInfo will not assert as cluster ..
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}*/
	// clusterStatus from ManagedCluster or ManagedClusterInfo?
	// https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/cluster/v1beta1/clusterinfo_types.go
	// https://github.com/open-cluster-management/api/blob/master/cluster/v1/types.go#L78
    // Old Definition https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/mcm/v1alpha1/clusterstatus_types.go	

	resource := transformCluster(&managedCluster, &managedCluster.Status)
	//clusterstat := ClusterStat{clusterStatus: clusterStatus}
	resource.Properties["status"] = "TODO" // TODO: Get the status.
	clustName, ok := resource.Properties["name"].(string)
	// Install/uninstall jobs might take some time to start - if cluster is in unknown status, we use statClusterMap to restart the clusterInformer in order to update cluster status -
	// TODO: Remove this workaround and get a cluster status variable from mcm with each cluster resource
	// Use cluster, clusterstatus
	if ok {
		statClusterMapMutex.RLock()
		present := statClusterMap[clustName]
		statClusterMapMutex.RUnlock()

		if present {
			//delete the cluster from map if status is not unknown anymore
			if resource.Properties["status"] != "unknown" {
				statClusterMapMutex.Lock()
				delete(statClusterMap, clustName)
				statClusterMapMutex.Unlock()
			}
		} else {
			//add the cluster to map if status is unknown
			if resource.Properties["status"] == "unknown" {
				statClusterMapMutex.Lock()
				statClusterMap[clustName] = true
				statClusterMapMutex.Unlock()
			}
		}
	}

	// Ensure that the cluster resource is still present before inserting into data store.
	/* assuming it's still there 
	c, err := cluster.ClusterregistryV1alpha1().Clusters(cluster.Namespace).Get(cluster.Name, v1.GetOptions{})
	if err != nil {
		glog.Warningf("The cluster %s to add/update is not present anymore.", cluster.Name)
		delCluster(cluster)
		return
	}
	*/

	_, _, err = db.Insert([]*db.Resource{&resource}, "")
/*
	glog.V(2).Info("Updating Cluster resource by name in RedisGraph. ", resource)
	_, err = db.UpdateByName(resource)
	if (db.IsGraphMissing(err) || !db.IsPropertySet(res)) && (c.Name == cluster.Name) {
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
*/
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



func transformCluster(cluster *clusterv1.ManagedCluster, clusterStatus *clusterv1.ManagedClusterStatus) db.Resource {

	props := make(map[string]interface{})


	// get these fields from object
	props["name"] = cluster.GetName()

	props["kind"] = "Cluster"
	/*props["apigroup"] = "clusterregistry.k8s.io"
	props["selfLink"] = cluster.GetSelfLink()
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
	if cluster.GetNamespace() != "" {
		props["namespace"] = cluster.GetNamespace()
	}

	if clusterStatus != nil {
		props["consoleURL"] = clusterStatus.Spec.ConsoleURL
		props["cpu"], _ = clusterStatus.Spec.Capacity.Cpu().AsInt64()
		props["memory"] = clusterStatus.Spec.Capacity.Memory().String()
		props["klusterletVersion"] = clusterStatus.Spec.KlusterletVersion
		props["kubernetesVersion"] = clusterStatus.Spec.Version

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
	}
*/

	return db.Resource{
		Kind:           "ManagedCluster",
		UID:            string(cluster.GetUID()),
		Properties:     props,
		ResourceString: "managedclusters",
	}

}


// Deletes a cluster resource and all resourcces from the cluster.
func delCluster(cluster *clusterregistry.Cluster) {
	glog.Infof("Deleting Cluster resource %s and all resources from the cluster.", cluster.Name)
	uid := string(cluster.GetUID())
	_, err := db.Delete([]string{uid})
	if err != nil {
		glog.Error("Error deleting Cluster kind with error: ", err)
	}
	delClusterResources(cluster)
}

// Removes all the resources for a cluster, but doesn't remove the Cluster resource object.
func delClusterResources(cluster *clusterregistry.Cluster) {
	_, err := db.DeleteCluster(cluster.GetName())
	if err != nil {
		glog.Error("Error deleting current resources for cluster: ", err)
	} else {
		db.DeleteClustersCache(string(cluster.GetUID()))
	}
}

