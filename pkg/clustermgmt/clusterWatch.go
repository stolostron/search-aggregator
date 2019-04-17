/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package clustermgmt

import (
	"time"

	"github.com/golang/glog"
	mcm "github.ibm.com/IBMPrivateCloud/hcm-api/pkg/apis/mcm/v1alpha1"
	mcmClientset "github.ibm.com/IBMPrivateCloud/hcm-api/pkg/client/clientset_generated/clientset"
	informers "github.ibm.com/IBMPrivateCloud/hcm-api/pkg/client/informers_generated/externalversions"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/config"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	clientset "k8s.io/cluster-registry/pkg/client/clientset/versioned"
)

// WatchClusters watches k8s cluster and clusterstatus objects and updates the search cache.
func WatchClusters() {
	var clientConfig *rest.Config
	var err error

	if config.Cfg.KubeConfig != "" {
		glog.Infof("Creating k8s client using path: %s", config.Cfg.KubeConfig)
		clientConfig, err = clientcmd.BuildConfigFromFlags("", config.Cfg.KubeConfig)
	} else {
		glog.Info("Creating k8s client using InClusterConfig()")
		clientConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		glog.Fatal("Error Constructing Client From Config: ", err)
	}

	stopper := make(chan struct{})
	defer close(stopper)

	// Initialize the cluster client, used for Cluster resource
	clusterClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		glog.Fatal("Cannot Construct Cluster Client From Config: ", err)
	}

	// Initialize the mcm client, used for ClusterStatus resource
	mcmClient, err := mcmClientset.NewForConfig(clientConfig)
	if err != nil {
		glog.Fatal("Cannot Construct MCM Client From Config: ", err)
	}
	clusterStatusFactory := informers.NewSharedInformerFactory(mcmClient, 0)
	clusterStatusInformer := clusterStatusFactory.Mcm().V1alpha1().ClusterStatuses().Informer()

	clusterStatusInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			clusterStatus, ok := obj.(*mcm.ClusterStatus)
			if !ok {
				glog.Error("Failed to assert ClusterStatus informer obj to mcm.ClusterStatus")
				return
			}

			cluster, err := clusterClient.ClusterregistryV1alpha1().
				Clusters(clusterStatus.GetNamespace()).
				Get(clusterStatus.GetName(), metav1.GetOptions{})
			if err != nil {
				glog.Error("Failed to fetch cluster resource: ", err)
			}

			resource := transformCluster(clusterStatus, cluster)

			glog.Info("Inserting Cluster resource in RedisGraph. ", resource)

			_, _, err = db.Insert([]*db.Resource{&resource})
			if err != nil {
				glog.Error("Error adding Cluster kind with error: ", err)
			}
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			clusterStatus, ok := next.(*mcm.ClusterStatus)
			if !ok {
				glog.Error("Failed to assert ClusterStatus informer obj to mcm.ClusterStatus")
				return
			}

			cluster, err := clusterClient.ClusterregistryV1alpha1().
				Clusters(clusterStatus.GetNamespace()).
				Get(clusterStatus.GetName(), metav1.GetOptions{})
			if err != nil {
				glog.Error("Failed to fetch cluster resource: ", err)
			}

			resource := transformCluster(clusterStatus, cluster)

			glog.Info("Updating Cluster resource in RedisGraph. ", resource)
			_, _, err = db.Update([]*db.Resource{&resource})
			if err != nil {
				glog.Error("Error updating Cluster kind with errors: ", err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			clusterStatus, ok := obj.(*mcm.ClusterStatus)
			if !ok {
				glog.Error("Failed to assert ClusterStatus informer obj to mcm.ClusterStatus")
				return
			}

			glog.Info("Deleting Cluster resource in RedisGraph.")
			uid := string(clusterStatus.GetUID())
			_, err = db.Delete([]string{uid})
			if err != nil {
				glog.Error("Error deleting Cluster kind with error: ", err)
			}
		},
	})

	clusterStatusInformer.Run(stopper)
}

func transformCluster(clusterStatus *mcm.ClusterStatus, cluster *clusterregistry.Cluster) db.Resource {
	props := make(map[string]interface{})

	props["name"] = clusterStatus.GetName()
	props["kind"] = "Cluster"
	props["cluster"] = "local-cluster" // Needed for rbac
	props["selfLink"] = clusterStatus.GetSelfLink()
	props["created"] = clusterStatus.GetCreationTimestamp().UTC().Format(time.RFC3339)

	if clusterStatus.GetLabels() != nil {
		props["label"] = clusterStatus.GetLabels()
	}
	if clusterStatus.GetNamespace() != "" {
		props["namespace"] = clusterStatus.GetNamespace()
	}

	// we are pulling the status from the cluster object and cluster info from the clusterStatus object :(
	if cluster != nil && len(cluster.Status.Conditions) > 0 && cluster.Status.Conditions[0].Type != "" {
		props["status"] = string(cluster.Status.Conditions[0].Type)
	} else {
		props["status"] = "offline"
	}

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

	return db.Resource{
		Kind:       "Cluster",
		UID:        string(clusterStatus.GetUID()),
		Properties: props,
	}
}
