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
	mcm "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/mcm/v1alpha1"
	mcmClientset "github.com/open-cluster-management/multicloud-operators-foundation/pkg/client/clientset_generated/clientset"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
	hive "github.com/openshift/hive/pkg/apis/hive/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	informers "k8s.io/cluster-registry/pkg/client/informers/externalversions"
)

var statClusterMap map[string]bool // Install/uninstall jobs might take some time to start - if cluster is in unknown status, we use this map to restart the clusterInformer in order to update cluster status
var statClusterMapMutex = sync.RWMutex{}

const HIVE_DOMAIN = "hive.openshift.io"
const UNINSTALL_LABEL = HIVE_DOMAIN + "/uninstall"
const INSTALL_LABEL = HIVE_DOMAIN + "/install"
const CLUSTER_LABEL = HIVE_DOMAIN + "/cluster-deployment-name"

//ClusterStat struct stores all resources needed to calculate status of the cluster
type ClusterStat struct {
	cluster           *clusterregistry.Cluster
	clusterStatus     *mcm.ClusterStatus
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

func processClusterUpsert(obj interface{}, mcmClient *mcmClientset.Clientset) {
	var err error
	var cluster *clusterregistry.Cluster
	var clusterStatus *mcm.ClusterStatus
	var clusterDeployment *hive.ClusterDeployment
	var ok bool
	var installJobs, uninstallJobs *batch.JobList

	cluster, ok = obj.(*clusterregistry.Cluster)
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}
	clusterStatus, err = mcmClient.McmV1alpha1().ClusterStatuses(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Error("Failed to fetch cluster status: ", err)
		clusterStatus = nil //If there is an error fetching clusterStatus, reset it to nil
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
	resource.Properties["status"] = getStatus(clusterstat)
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

	//Ensure that the cluster resource is still present before inserting into Redisgraph.
	c, err := config.ClusterClient.ClusterregistryV1alpha1().Clusters(cluster.Namespace).Get(cluster.Name, v1.GetOptions{})
	if err != nil {
		glog.Warningf("The cluster %s to add/update is not present anymore", cluster.Name)
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

	/// If a cluster is offline we should remove the cluster objects
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

func delCluster(cluster *clusterregistry.Cluster) {
	glog.Info("Deleting Cluster resource ", cluster.Name, " from RedisGraph.")
	uid := string(cluster.GetUID())
	_, err := db.Delete([]string{uid})
	if err != nil {
		glog.Error("Error deleting Cluster kind with error: ", err)
	}

	// When a cluster (ClusterStatus) gets deleted, we must remove all resources for that cluster from RedisGraph.
	_, err = db.DeleteCluster(cluster.GetName())
	if err != nil {
		glog.Error("Error deleting current resources for cluster: ", err)
	}
}

//TODO: Remove this and get cluster status from cluster object using issue (https://github.com/open-cluster-management/backlog/issues/1518)
//Similar to console-ui's cluster status - https://github.com/open-cluster-management/console-api/blob/98a3a58bed402930c557c0e9c854deab8f84cf38/src/v2/models/cluster.js#L30
// If clusterdeployment resource is present and install/uninstall jobs are active - cluster is in creating/destroying status
//If jobs are not active, status is based on clusterdeployment's status(cd.Status.ClusterVersionStatus)
// if status.conditions.type is `Available` and status.conditions.status is `True`, then the cluster is detached

// If cluster resource is present with a deletionTimestamp, cluster is in detaching mode
// cluster with status with conditions[0].type === '' indicates cluster is offline
// Empty status indicates cluster has not been imported - is in pending mode
// If cluster is pending import because Hive is installing or uninstalling, cluster status based on jobs will be creating/destroying
func getStatus(cs ClusterStat) string {
	cluster := cs.cluster
	cd := cs.clusterdeployment

	// we are using a combination of conditions to determine cluster status
	var clusterdeploymentStatus, status string

	if cd != nil {
		if cs.uninstallJobs != nil && len(cs.uninstallJobs.Items) > 0 && chkJobActive(cs.uninstallJobs, "uninstall") != "" {
			clusterdeploymentStatus = chkJobActive(cs.uninstallJobs, "uninstall")
		} else if cs.installJobs != nil && len(cs.installJobs.Items) > 0 && chkJobActive(cs.installJobs, "install") != "" {
			clusterdeploymentStatus = chkJobActive(cs.installJobs, "install")
		} else {
			clusterdeploymentStatus = "unknown"
			if len(cd.Status.ClusterVersionStatus.Conditions) > 0 {
				for _, condition := range cd.Status.ClusterVersionStatus.Conditions {
					if condition.Type == "Available" {
						if condition.Status == "True" {
							clusterdeploymentStatus = "detached"
						}
						break
					}
				}
			}
		}
	}
	if cluster != nil {
		if cluster.DeletionTimestamp != nil {
			return "detaching"
		}
		// Empty status indicates cluster has not been imported
		status = "pending"
		// we are pulling the status from the cluster object and cluster info from the clusterStatus object :(
		if len(cluster.Status.Conditions) > 0 {
			status = string(cluster.Status.Conditions[0].Type)
		}
		if status == "" { // status with conditions[0].type === '' indicates cluster is offline
			status = "offline"
		}
		// If cluster is pending import because Hive is installing or uninstalling,
		// show that status instead
		if status == "pending" && clusterdeploymentStatus != "" && clusterdeploymentStatus != "detached" {
			return clusterdeploymentStatus
		}
		return status
	}
	return clusterdeploymentStatus
}
