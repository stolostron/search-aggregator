/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package clustermgmt

import (
	"strings"
	"time"

	"github.com/golang/glog"
	mcm "github.ibm.com/IBMPrivateCloud/hcm-api/pkg/apis/mcm/v1alpha1"
	mcmClientset "github.ibm.com/IBMPrivateCloud/hcm-api/pkg/client/clientset_generated/clientset"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/config"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	informers "k8s.io/cluster-registry/pkg/client/informers/externalversions"
)

// WatchClusters watches k8s cluster and clusterstatus objects and updates the search cache.
func WatchClusters() {
	var err error
	clusterClient, mcmClient, err := config.InitClient()
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

			glog.Info("Deleting Cluster resource in RedisGraph.")
			uid := string(cluster.GetUID())
			_, err = db.Delete([]string{uid})
			if err != nil {
				glog.Error("Error deleting Cluster kind with error: ", err)
			}

			// When a cluster (ClusterStatus) gets deleted, we must remove all resources for that cluster from RedisGraph.
			_, err := db.DeleteCluster(cluster.GetName())
			if err != nil {
				glog.Error("Error deleting current resources for cluster: ", err)
			}
		},
	})

	// periodically check if the cluster resource exists and start/stop the informer accordingly
	for {
		_, err := clusterClient.ServerResourcesForGroupVersion("clusterregistry.k8s.io/v1alpha1")
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
			}
		}

		time.Sleep(time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond)
	}
}

func processClusterUpsert(obj interface{}, mcmClient *mcmClientset.Clientset) {
	cluster, ok := obj.(*clusterregistry.Cluster)
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}

	clusterStatus, err := mcmClient.McmV1alpha1().
		ClusterStatuses(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Error("Failed to fetch cluster status: ", err)
	}

	resource := transformCluster(cluster, clusterStatus)

	glog.V(2).Info("Updating Cluster resource by name in RedisGraph. ", resource)
	res, err := db.UpdateByName(resource)
	if db.IsGraphMissing(err) || !db.IsPropertySet(res) {
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

	// If a cluster is offline we should remove the cluster objects
	if resource.Properties["status"] == "offline" {
		glog.Infof("Cluster %s appears to be offline, removing cluster resources from redis", cluster.GetName())
		_, err := db.DeleteCluster(cluster.GetName())
		if err != nil {
			glog.Error("Error deleting current resources for cluster: ", err)
		} else {
			db.DeleteClustersCache(resource.UID)
		}
	}
}

func isClusterMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "could not find the requested resource")
}

func transformCluster(cluster *clusterregistry.Cluster, clusterStatus *mcm.ClusterStatus) db.Resource {
	props := make(map[string]interface{})

	props["name"] = cluster.GetName()
	props["kind"] = "Cluster"
	props["apigroup"] = "clusterregistry.k8s.io"
	props["selfLink"] = cluster.GetSelfLink()
	props["created"] = cluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	if cluster.GetLabels() != nil {
		props["label"] = cluster.GetLabels()
	}
	if cluster.GetNamespace() != "" {
		props["namespace"] = cluster.GetNamespace()
	}

	// we are pulling the status from the cluster object and cluster info from the clusterStatus object :(
	if len(cluster.Status.Conditions) > 0 && cluster.Status.Conditions[0].Type != "" {
		props["status"] = string(cluster.Status.Conditions[0].Type)
	} else {
		props["status"] = "offline"
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

	return db.Resource{
		Kind:           "Cluster",
		UID:            string(cluster.GetUID()),
		Properties:     props,
		ResourceString: "clusters",
	}
}
