/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package config

import (
	"github.com/golang/glog"
	mcmClientset "github.com/open-cluster-management/multicloud-operators-foundation/pkg/client/clientset_generated/clientset"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientset "k8s.io/cluster-registry/pkg/client/clientset/versioned"
)

var ClusterClient *clientset.Clientset

func InitClient() (*clientset.Clientset, *mcmClientset.Clientset, error) {
	var clientConfig *rest.Config
	var err error
	//(*clientset.Clientset, *versioned.Clientset)
	if Cfg.KubeConfig != "" {
		glog.Infof("Creating k8s client using path: %s", Cfg.KubeConfig)
		clientConfig, err = clientcmd.BuildConfigFromFlags("", Cfg.KubeConfig)
	} else {
		glog.Info("Creating k8s client using InClusterConfig()")
		clientConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatal("Error Constructing Client From Config: ", err)
	}

	// Initialize the mcm client, used for ClusterStatus resource
	mcmClient, err := mcmClientset.NewForConfig(clientConfig)
	if err != nil {
		glog.Fatal("Cannot Construct MCM Client From Config: ", err)
	}

	// Initialize the cluster client, used for Cluster resource
	clusterClient, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		glog.Fatal("Cannot Construct Cluster Client From Config: ", err)
	}
	ClusterClient = clusterClient
	return clusterClient, mcmClient, err
}
