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


	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	//clusterv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/cluster/v1beta1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// Watches ManagedCluster and ManagedClusterInfo objects and updates the search cache.
func WatchClusters() {
	glog.Info("Begin ClusterWatch routine")

	var stopper chan struct{}
	informerRunning := false

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

	// Watch ManagedCluster resource
	gvr, _ := schema.ParseResourceArg("managedclusters.v1.cluster.open-cluster-management.io")
	clusterInformer := dynamicFactory.ForResource(*gvr).Informer()
	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processClusterUpsert(obj, config.KubeClient)
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			processClusterUpsert(next, config.KubeClient)
		},
		DeleteFunc: func(obj interface{}) {
			processClusterDelete(obj)
		},
	})

	// TODO: watch ManagedClusterInfo

	// gvr, _ := schema.ParseResourceArg("managedclusterinfos.v1beta1.internal.open-cluster-management.io")

	/*
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
		_, err := config.KubeClient.ServerResourcesForGroupVersion("cluster.open-cluster-management.io/v1")
		// we fail to fetch for some reason other than not found
		if err != nil && !isClusterMissing(err) {
			glog.Error("Cannot fetch resource list for cluster.open-cluster-management.io/v1: ", err)
		} else {
			if informerRunning && isClusterMissing(err) {
				glog.Info("Stopping cluster informer routine because ManagedCluster resource not found.")
				stopper <- struct{}{}
				informerRunning = false
			} else if !informerRunning && !isClusterMissing(err) {
				glog.Info("Starting cluster informer routine for cluster watch")
				stopper = make(chan struct{})
				informerRunning = true
				go clusterInformer.Run(stopper)
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

	managedCluster := clusterv1.ManagedCluster{}
	err = json.Unmarshal(j, &managedCluster)
	if err != nil {
		glog.Warning("Error on ManagedCluster unmarshal.")
	}

	/*cluster, ok = obj.(*clusterregistry.Cluster) // ManagedClusterInfo will not assert as cluster ..
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}*/

	// https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/cluster/v1beta1/clusterinfo_types.go
	// https://github.com/open-cluster-management/api/blob/master/cluster/v1/types.go#L78
	// Old Definition https://github.com/open-cluster-management/multicloud-operators-foundation/blob/master/pkg/apis/mcm/v1alpha1/clusterstatus_types.go

	resource := transformCluster(&managedCluster, &managedCluster.Status)
	resource.Properties["status"] = "" // TODO: Get the status.

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

func transformCluster(cluster *clusterv1.ManagedCluster, clusterStatus *clusterv1.ManagedClusterStatus) db.Resource {

	props := make(map[string]interface{})


	// get these fields from ManagedCluster object
	props["name"] = cluster.GetName()
	props["kind"] = "Cluster"
	props["apigroup"] = "cluster.open-cluster-management.io"
	props["created"] = cluster.GetCreationTimestamp().UTC().Format(time.RFC3339)
	props["selfLink"] = cluster.GetSelfLink()





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


		glog.Infof("memory:\t %s", props["memory"])
		fmt.Printf("ManagedCluster:\n%+v", cluster)
		capacity := cluster.Status.Capacity["memory"] // pointer reference required .. "
		props["memory"] = capacity.String()
/*
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

	managedCluster := clusterv1.ManagedCluster{}
	err = json.Unmarshal(j, &managedCluster)
	if err != nil {
		glog.Error("Failed to unmarshall ManagedCluster on processDeleteCluster")
	}
	clusterName := managedCluster.GetName()
	clusterUID := string("cluster_" + managedCluster.GetUID())
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
