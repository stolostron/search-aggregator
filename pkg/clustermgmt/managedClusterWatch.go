/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
// Copyright (c) 2020 Red Hat, Inc.

package clustermgmt

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	//mcm "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/mcm/v1alpha1" // deprecated
	//mcmClientset "github.com/open-cluster-management/multicloud-operators-foundation/pkg/client/clientset_generated/clientset" // deprecated
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
	hive "github.com/openshift/hive/pkg/apis/hive/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	informers "k8s.io/cluster-registry/pkg/client/informers/externalversions"
	"k8s.io/client-go/dynamic"
	ManagedClusterInfo "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/cluster/v1beta1"
	// clusterv1 "github.com/open-cluster-management/api/cluster/v1" // ManagedCluster 
	// &clusterv1.ManagedCluster{} 
)

var statClusterMap map[string]bool // Install/uninstall jobs might take some time to start - if cluster is in unknown status, we use this map to restart the clusterInformer in order to update cluster status
var statClusterMapMutex = sync.RWMutex{}

const HIVE_DOMAIN = "hive.openshift.io"
const UNINSTALL_LABEL = HIVE_DOMAIN + "/uninstall"
const INSTALL_LABEL = HIVE_DOMAIN + "/install"
const CLUSTER_LABEL = HIVE_DOMAIN + "/cluster-deployment-name"

//ClusterStat struct stores all resources needed to calculate status of the cluster
type ClusterStat struct {
	cluster           *clusterregistry.Cluster // deprecated
	// clusterStatus     *mcm.ClusterStatus // deprecated
	uninstallJobs     *batch.JobList
	installJobs       *batch.JobList
	clusterdeployment *hive.ClusterDeployment
}

// WatchClusters watches k8s cluster and clusterstatus objects and updates the search cache.
func WatchClusters() {

	clusterClient, mcmClient, err := config.InitClient()
	if err != nil {
		glog.Info("Unable to create clientset ", err)
	}
	statClusterMap = map[string]bool{}
	var stopper chan struct{}
	informerRunning := false
    /*
	clusterFactory := informers.NewSharedInformerFactory(clusterClient, 0) // create an informer to watch changes to the clusterClient
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

	// create informer to watch changes to the ManagedClusterInfo custom resource 
	clusterInformer := initializeDynamicInformer(clusterClient)

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

	if clusterStatus != nil { // TODO: confirm managed cluster has all of these properties
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

func chkJobActive(jobs *batch.JobList, action string) string {
	if jobs != nil && len(jobs.Items) > 0 {
		for _, job := range jobs.Items {
			if job.Status.Active == 1 {
				if action == "uninstall" {
					return "destroying"
				} else {
					return "creating"
				}
			}
		}
	}
	return ""
}

// Deletes a cluster resource and all resources from the cluster.
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

//TODO: Remove this and get cluster status from cluster object using issue (https://github.com/open-cluster-management/backlog/issues/1518)
// Similar to console-ui's cluster status - https://github.com/open-cluster-management/console-api/blob/98a3a58bed402930c557c0e9c854deab8f84cf38/src/v2/models/cluster.js#L30
// If clusterdeployment resource is present and install/uninstall jobs are active - cluster is in creating/destroying status
// If jobs are not active, status is based on clusterdeployment's status(cd.Status.ClusterVersionStatus)
// if status.conditions.type is `Available` and status.conditions.status is `True`, then the cluster is detached

// If cluster resource is present with a deletionTimestamp, cluster is in detaching mode
// cluster with status with conditions[0].type === '' indicates cluster is offline
// Empty status indicates cluster has not been imported - is in pending mode
// If cluster is pending import because Hive is installing or uninstalling, cluster status based on jobs will be creating/destroying

// initializes a dynamic informer specifically to watch ManagedClusterInfo type
func initializeDynamicInformerForManagedClusterInfo(mcmClient *mcmClientset.Clientset) dynamic.dynamicinformer {

    // Initialize the dynamic client, used for CRUD operations on arbitrary k8s resources
    dynamicclientset, err := dynamic.newforconfig()
    if err != nil {
        // not fatal glog.fatal("cannot construct dynamic client from config: ", err)
    }

    // create dynamic factories
    // factory for building dynamic informer objects used with crds and arbitrary k8s objects
    dynamicfactory := dynamic.dynamicinformer.newdynamicsharedinformerfactory(dynamicclientset, 0)

	managedclusterinfo := &ManagedClusterInfo{}

	// create dynamic informer
	informer := dynamicfactory.forresource(managedclusterinfo)
	glog.infof("found new resource %s, creating informer\n", managedclusterinfo.string())

	// set up handler to pass this informer's resources into transformer informer.addeventhandler(cache.resourceeventhandlerfuncs{
		addfunc: func(obj interface{}) *unstructured.unstructured {
			managedClusterProcessUpsert(obj)
		},
		updatefunc: func(obj interface{}) *unstructured.unstructured {
			managedClusterprocessClusterUpsert(next, mcmClient)
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

	return informer
}

// we will use managed cluster just like processClsuterUpsert 
func managedClusterProcessClusterUpsert(obj interface{}, mcmClient *mcmClientset.Clientset) {

	// read get managedClusterInfo Object
	typedResource := &ManagedClusterInfo{}
	err = json.Unmarshal(obj.(*unstructured.Unstructured), &typedResource)
	if err != nil {
		panic(err) // Will be caught by handleRoutineExit
		// don't panic... maybe panic?	
	}
	managedClusterInfo = ManagedClusterInfo{&typedResource} //managedClusterInfo.Status

	var err error
	var cluster *clusterregistry.Cluster
	var clusterStatus *mcm.ClusterStatus // mci.status
	var clusterDeployment *hive.ClusterDeployment
	var ok bool
	var installJobs, uninstallJobs *batch.JobList

	cluster, ok = obj.(*clusterregistry.Cluster)
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}
	//clusterStatus, err = mcmClient.McmV1alpha1().ClusterStatuses(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Error("Failed to fetch cluster status: ", err)
		//clusterStatus = nil //If there is an error fetching clusterStatus, reset it to nil
	}

	//get the clusterDeployment if it exists
	clusterDeployment, err = config.HiveClient.HiveV1().ClusterDeployments(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Info("Failed to fetch cluster deployment: ", err)
		clusterDeployment = nil //If there is an error fetching clusterDeployment, reset it to nil
	}
	//get install/uninstall jobs for cluster if they exist
	jobs := config.KubeClient.BatchV1().Jobs(cluster.GetNamespace())
	uninstallLabel := CLUSTER_LABEL + "=" + cluster.Name + "," + UNINSTALL_LABEL + "=true"
	installLabel := CLUSTER_LABEL + "=" + cluster.Name + "," + INSTALL_LABEL + "=true"
	listOptions := v1.ListOptions{}
	listOptions.LabelSelector = uninstallLabel //"hive.openshift.io/cluster-deployment-name=test-cluster,hive.openshift.io/uninstall="true""
	uninstallJobs, err = jobs.List(listOptions)
	if err != nil {
		glog.Error("Failed to fetch uninstall jobs: ", err)
		uninstallJobs = nil //If there is an error fetching uninstallJobs, reset it to nil
	}
	listOptions.LabelSelector = installLabel //"hive.openshift.io/cluster-deployment-name=test-cluster,hive.openshift.io/install="true""
	installJobs, err = jobs.List(listOptions)
	if err != nil {
		glog.Error("Failed to fetch install jobs: ", err)
		installJobs = nil //If there is an error fetching installJobs, reset it to nil
	}

	resource := transformCluster(cluster, clusterStatus)
	clusterstat := ClusterStat{cluster: cluster, clusterStatus: clusterStatus, uninstallJobs: uninstallJobs, installJobs: installJobs, clusterdeployment: clusterDeployment}
	resource.Properties["status"] = getStatus(clusterstat) // new method 
	clustName, ok := resource.Properties["name"].(string)
	// Install/uninstall jobs might take some time to start - if cluster is in unknown status, we use statClusterMap to restart the clusterInformer in order to update cluster status -
	//TODO: Remove this workaround and get a cluster status variable from mcm with each cluster resource
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
	c, err := config.ClusterClient.ClusterregistryV1alpha1().Clusters(cluster.Namespace).Get(cluster.Name, v1.GetOptions{})
	if err != nil {
		glog.Warningf("The cluster %s to add/update is not present anymore.", cluster.Name)
		delCluster(cluster)
		return
	}

	glog.V(2).Info("Updating Cluster resource by name in RedisGraph. ", resource)
	res, err := db.UpdateByName(resource)
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

	// If a cluster is offline we remove the resources from that cluster, but leave the cluster resource object.
	if resource.Properties["status"] == "offline" {
		glog.Infof("Cluster %s is offline, removing cluster resources from datastore.", cluster.GetName())
		delClusterResources(cluster)
	}
}



