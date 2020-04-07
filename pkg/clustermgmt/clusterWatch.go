/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package clustermgmt

import (
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	mcm "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/mcm/v1alpha1"
	mcmClientset "github.com/open-cluster-management/multicloud-operators-foundation/pkg/client/clientset_generated/clientset"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
	hive "github.com/openshift/hive/pkg/apis/hive/v1"
	hiveClientset "github.com/openshift/hive/pkg/client/clientset-generated/clientset"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	informers "k8s.io/cluster-registry/pkg/client/informers/externalversions"
)

const HIVE_DOMAIN = "hive.openshift.io"
const UNINSTALL_LABEL = HIVE_DOMAIN + "/uninstall"
const INSTALL_LABEL = HIVE_DOMAIN + "/install"
const CLUSTER_LABEL = HIVE_DOMAIN + "/cluster-deployment-name"

var anyClusterPending bool // Install/uninstall jobs might take some time to start - if cluster is pending, we use anyClusterPending bool to restart the clusterInformer in order to update cluster status

// WatchClusters watches k8s cluster and clusterstatus objects and updates the search cache.
func WatchClusters() {
	var err error
	clusterClient, mcmClient, hiveClient, kubeClient, err := config.InitClient()
	var stopper chan struct{}
	informerRunning := false

	clusterFactory := informers.NewSharedInformerFactory(clusterClient, 0)
	clusterInformer := clusterFactory.Clusterregistry().V1alpha1().Clusters().Informer()
	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processClusterUpsert(obj, mcmClient, hiveClient, kubeClient)

		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			processClusterUpsert(next, mcmClient, hiveClient, kubeClient)
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
					glog.Info("Restarting cluster informer routine for cluster watch")
					stopper <- struct{}{}
					stopper = make(chan struct{})
					informerRunning = true
					anyClusterPending = false
					go clusterInformer.Run(stopper)
				}
			}
		}

		time.Sleep(time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond)
	}
}

func processClusterUpsert(obj interface{}, mcmClient *mcmClientset.Clientset, hiveClient *hiveClientset.Clientset, kubeClient *kubeClientset.Clientset) {
	cluster, ok := obj.(*clusterregistry.Cluster)
	if !ok {
		glog.Error("Failed to assert Cluster informer obj to clusterregistry.Cluster")
		return
	}
	glog.Info("********* cluster: ", cluster.Name)
	clusterStatus, err := mcmClient.McmV1alpha1().
		ClusterStatuses(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Error("Failed to fetch cluster status: ", err)
	}

	//get the clusterDeployment if it exists
	clusterDeployment, err := hiveClient.HiveV1().ClusterDeployments(cluster.GetNamespace()).Get(cluster.GetName(), v1.GetOptions{})
	if err != nil {
		glog.Error("Failed to fetch cluster deployment: ", err)
	}
	//get install/uninstall jobs for cluster if they exist
	jobs := kubeClient.BatchV1().Jobs(cluster.GetNamespace())
	uninstallLabel := CLUSTER_LABEL + "=" + cluster.Name + "," + UNINSTALL_LABEL + "=true"
	installLabel := CLUSTER_LABEL + "=" + cluster.Name + "," + INSTALL_LABEL + "=true"
	listOptions := v1.ListOptions{}
	listOptions.LabelSelector = uninstallLabel //"hive.openshift.io/cluster-deployment-name=test-cluster2, hive.openshift.io/install: "true""
	uninstallJobs, err := jobs.List(listOptions)
	listOptions.LabelSelector = installLabel //"hive.openshift.io/cluster-deployment-name=test-cluster2, hive.openshift.io/install: "true""
	installJobs, err := jobs.List(listOptions)
	resource := transformCluster(cluster, clusterStatus)

	resource.Properties["status"] = getStatus(cluster, clusterStatus, uninstallJobs, installJobs, clusterDeployment)
	if resource.Properties["status"] == "pending" {
		// Install/uninstall jobs might take some time to start - if cluster is pending, we use anyClusterPending bool to restart the clusterInformer in order to update cluster status -
		//TODO: Remove this workaround and get a cluster status variable from mcm with each cluster resource
		anyClusterPending = true
	}

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
	glog.Info(action, " Jobs nil")
	return ""
}

//Similar to console-ui's cluster status - https://github.com/open-cluster-management/console-api/blob/98a3a58bed402930c557c0e9c854deab8f84cf38/src/v2/models/cluster.js#L30
func getStatus(cluster *clusterregistry.Cluster, clusterStatus *mcm.ClusterStatus, uninstallJobs *batch.JobList, installJobs *batch.JobList, cd *hive.ClusterDeployment) string {
	glog.Info("Inside getstatus fn for cluster: ", cluster.Name)
	glog.Info("len install jobs: ", len(installJobs.Items))
	glog.Info("len uninstall jobs: ", len(uninstallJobs.Items))

	// we are using a combination of conditions to determine cluster status
	var clusterdeploymentStatus = ""
	var status = ""

	if cd != nil {
		glog.Info("cd: ", cd)

		if uninstallJobs != nil && len(uninstallJobs.Items) > 0 {
			clusterdeploymentStatus = chkJobActive(uninstallJobs, "uninstall")
		} else if installJobs != nil && len(installJobs.Items) > 0 {
			clusterdeploymentStatus = chkJobActive(installJobs, "install")
		} else {
			if len(cd.Status.Conditions) > 0 {
				for _, condition := range cd.Status.Conditions {
					if condition.Type == "Available" {
						if condition.Status == "True" {
							clusterdeploymentStatus = "detached"
						} else {
							clusterdeploymentStatus = "unknown"
						}
						break
					}
				}
			} else {
				glog.Info("len(cd.Status.Conditions) is zero ")
			}
		}
		glog.Info("status in cd != nil ", status)
		glog.Info("clusterdeploymentStatus in cd != nil ", clusterdeploymentStatus)

	}
	if cluster != nil {
		if clusterStatus != nil && cluster.DeletionTimestamp != nil {
			glog.Info("Returning detaching")

			return "detaching"
		}

		// we are pulling the status from the cluster object and cluster info from the clusterStatus object :(
		if len(cluster.Status.Conditions) > 0 {
			if cluster.Status.Conditions[0].Type != "" {
				status = string(cluster.Status.Conditions[0].Type)
			} else {
				// status with conditions[0].type === '' indicates cluster is offline
				status = "offline"
			}
		} else {
			// Empty status indicates cluster has not been imported
			if reflect.DeepEqual(clusterregistry.Cluster{}.Status, cluster.Status) {
				status = "pending"
			}
		}

		// If cluster is pending import because Hive is installing or uninstalling,
		// show that status instead
		if status == "pending" && clusterdeploymentStatus != "" && clusterdeploymentStatus != "detached" {
			glog.Info("status: ", status)
			glog.Info("clusterdeploymentStatus: ", clusterdeploymentStatus)
			glog.Info("Returning clusterdeploymentStatus: ", clusterdeploymentStatus)

			return clusterdeploymentStatus
		}
		glog.Info("status: ", status)
		glog.Info("clusterdeploymentStatus: ", clusterdeploymentStatus)
		glog.Info("Returning status1: ", status)

		return status
	}
	glog.Info("status: ", status)
	glog.Info("clusterdeploymentStatus: ", clusterdeploymentStatus)
	glog.Info("Returning status2: ", status)

	glog.Info("overall status is: ", status)
	return clusterdeploymentStatus
}
