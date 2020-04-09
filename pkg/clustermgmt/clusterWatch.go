/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package clustermgmt

import (
	"encoding/json"
	"strings"
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

var CurrClusterName string

const HIVE_DOMAIN = "hive.openshift.io"
const UNINSTALL_LABEL = HIVE_DOMAIN + "/uninstall"
const INSTALL_LABEL = HIVE_DOMAIN + "/install"
const CLUSTER_LABEL = HIVE_DOMAIN + "/cluster-deployment-name"

var anyClusterPending bool // Install/uninstall jobs might take some time to start - if cluster is pending, we use anyClusterPending bool to restart the clusterInformer in order to update cluster status

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
	var stopper chan struct{}
	informerRunning := false

	clusterFactory := informers.NewSharedInformerFactory(clusterClient, 0)
	clusterInformer := clusterFactory.Clusterregistry().V1alpha1().Clusters().Informer()
	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			glog.Info("In Add")
			processClusterUpsert(obj, mcmClient)

		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			glog.Info("In Update")
			processClusterUpsert(next, mcmClient)
		},
		DeleteFunc: func(obj interface{}) {
			cluster, ok := obj.(*clusterregistry.Cluster)
			glog.Info("In Delete for cluster, ", cluster.Name)
			if !ok {
				glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
				return
			}
			delCluster(cluster)
		},
	})

	// periodically check if the cluster resource exists and start/stop the informer accordingly
	for {
		glog.Info("periodically check if the cluster resource exists and start/stop the informer accordingly")
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
				if anyClusterPending {
					glog.Info("Warning: Will restart cluster informer routine for cluster watch")
					// stopper <- struct{}{}
					// stopper = make(chan struct{})
					// informerRunning = true
					// anyClusterPending = false
					// go clusterInformer.Run(stopper)
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
	glog.Info("In processClusterUpsert, cluster: ", cluster.Name)

	CurrClusterName = cluster.Name
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}
	clusterStatus, err = mcmClient.McmV1alpha1().ClusterStatuses(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Error("Failed to fetch cluster status: ", err)
		clusterStatus = nil //&mcm.ClusterStatus{}
	}

	//get the clusterDeployment if it exists
	clusterDeployment, err = config.HiveClient.HiveV1().ClusterDeployments(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Error("Failed to fetch cluster deployment: ", err)
		clusterDeployment = nil
	}
	//get install/uninstall jobs for cluster if they exist
	jobs := config.KubeClient.BatchV1().Jobs(cluster.GetNamespace())
	uninstallLabel := CLUSTER_LABEL + "=" + cluster.Name + "," + UNINSTALL_LABEL + "=true"
	installLabel := CLUSTER_LABEL + "=" + cluster.Name + "," + INSTALL_LABEL + "=true"
	listOptions := v1.ListOptions{}
	listOptions.LabelSelector = uninstallLabel //"hive.openshift.io/cluster-deployment-name=test-cluster2, hive.openshift.io/install: "true""
	uninstallJobs, err = jobs.List(listOptions)
	if err != nil {
		glog.Error("Failed to fetch uninstall jobs: ", err)
		uninstallJobs = nil
	}
	listOptions.LabelSelector = installLabel //"hive.openshift.io/cluster-deployment-name=test-cluster2, hive.openshift.io/install: "true""
	installJobs, err = jobs.List(listOptions)
	if err != nil {
		glog.Error("Failed to fetch install jobs: ", err)
		installJobs = nil
	}
	glog.Info("Calling transformcluster for cluster: ", cluster.Name, " currCluster: ", CurrClusterName)
	resource := transformCluster(cluster, clusterStatus)
	glog.Info("Calling getStatus for cluster: ", cluster.Name, " currCluster: ", CurrClusterName)
	clusterstat := ClusterStat{cluster: cluster, clusterStatus: clusterStatus, uninstallJobs: uninstallJobs, installJobs: installJobs, clusterdeployment: clusterDeployment}
	resource.Properties["status"] = getStatus(clusterstat)
	//resource.Properties["status"] = getStatus(cluster, clusterStatus, uninstallJobs, installJobs, clusterDeployment)

	glog.Info("status set for cluster: ", resource.Properties["status"])
	if resource.Properties["status"] != "ok" && resource.Properties["status"] != "offline" && resource.Properties["status"] != "detached" {
		// Install/uninstall jobs might take some time to start - if cluster is pending, we use anyClusterPending bool to restart the clusterInformer in order to update cluster status -
		//TODO: Remove this workaround and get a cluster status variable from mcm with each cluster resource
		anyClusterPending = true
	}

	//Ensure that the cluster resource is still present before inserting into Redisgraph. Race conditions are seen between add and delete resources
	c, err := config.ClusterClient.ClusterregistryV1alpha1().Clusters(cluster.Namespace).Get(cluster.Name, v1.GetOptions{})
	if err != nil {
		glog.Warningf("The cluster %s to add/update is not present anymore", cluster.Name)
	}

	glog.Info("Updating Cluster resource by name in RedisGraph. ", resource)
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
	} else {
		if c.Name != cluster.Name {
			glog.Warningf("Warning: Will have to delete %s from Redisgraph", cluster.Name)
			//delCluster(cluster)
		}
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
	glog.Info("Leaving processClusterUpsert")
	glog.Info("\n")

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
	glog.Info("Checking for ", action, " jobs")
	if jobs != nil && len(jobs.Items) > 0 {
		glog.Info("Jobs != nil")
		for _, job := range jobs.Items {
			glog.Info("Current job: ", job)
			if job.Status.Active == 1 {
				glog.Info("Job active. Returning ", action)
				if action == "uninstall" {
					glog.Info(action, " Job active. Returning destroying")
					return "destroying"
				} else {
					glog.Info(action, " Job active. Returning creating")
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

//Similar to console-ui's cluster status - https://github.com/open-cluster-management/console-api/blob/98a3a58bed402930c557c0e9c854deab8f84cf38/src/v2/models/cluster.js#L30
func getStatus(cs ClusterStat) string {
	// func getStatus(cluster *clusterregistry.Cluster, clusterStatus *mcm.ClusterStatus, uninstallJobs *batch.JobList, installJobs *batch.JobList, cd *hive.ClusterDeployment) string {
	cluster := cs.cluster
	cd := cs.clusterdeployment
	glog.Info("In getstatus for cluster: ", cluster.Name)
	glog.Info("In getstatus, cluster.Name: ", cluster.Name)
	glog.Info("In getstatus, CurrClusterName: ", CurrClusterName)

	if cluster.Name != CurrClusterName {
		glog.Warning("Warning: Cluster names do not match. Another cluster has started through the update process.")
	}

	// we are using a combination of conditions to determine cluster status
	var clusterdeploymentStatus = ""
	var status = ""

	if cd != nil {
		if cs.uninstallJobs != nil && len(cs.uninstallJobs.Items) > 0 && chkJobActive(cs.uninstallJobs, "uninstall") != "" {
			clusterdeploymentStatus = chkJobActive(cs.uninstallJobs, "uninstall")
		} else if cs.installJobs != nil && len(cs.installJobs.Items) > 0 && chkJobActive(cs.installJobs, "install") != "" {
			clusterdeploymentStatus = chkJobActive(cs.installJobs, "install")
		} else {
			glog.Warning("Warning: uninstallJobs and installJobs nil or len(items) = 0")
			glog.Info("len(cs.installJobs.Items): ", len(cs.installJobs.Items))
			glog.Info("len(cs.uninstallJobs.Items): ", len(cs.uninstallJobs.Items))

			clusterdeploymentStatus = "unknown"
			glog.Info("cd.Status.ClusterVersionStatus.Conditions: ", cd.Status.ClusterVersionStatus.Conditions)
			if len(cd.Status.ClusterVersionStatus.Conditions) > 0 {
				for _, condition := range cd.Status.ClusterVersionStatus.Conditions {
					if condition.Type == "Available" {
						if condition.Status == "True" {
							clusterdeploymentStatus = "detached"
						}
						break
					}
				}
			} else {
				glog.Info("len(cd.Status.ClusterVersionStatus.Conditions) is zero ")
			}
		}
		glog.Info("clusterdeploymentStatus0: ", clusterdeploymentStatus)
	}
	if cluster != nil {
		//if clusterStatus != nil && clusterStatus.Name != "" && cluster.DeletionTimestamp != nil {
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
			glog.Info("clusterdeploymentStatus1: ", clusterdeploymentStatus, " is returned. So status1 is pending ")

			return clusterdeploymentStatus
		}
		glog.Info("clusterdeploymentStatus2: ", clusterdeploymentStatus)
		glog.Info("status2: ", status, " is returned")

		return status
	}
	glog.Info("clusterdeploymentStatus3: ", clusterdeploymentStatus, " is returned")
	glog.Info("status3: ", status)

	return clusterdeploymentStatus
}
