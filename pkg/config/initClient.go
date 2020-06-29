/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
// Copyright (c) 2020 Red Hat, Inc.

package config

import (
	"github.com/golang/glog"

	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//KubeClient - Client to get jobs resource
var KubeClient *kubeClientset.Clientset

//InitClient - Initialize all clientsets
func InitClient() (*rest.Config, error) {
	var clientConfig *rest.Config
	var err error
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

	// Initialize the kube client.
	kubeClient, err := kubeClientset.NewForConfig(clientConfig)
	if err != nil {
		glog.Error("Cannot Construct kube Client From Config: ", err)
	}
	KubeClient = kubeClient

	return clientConfig, err
}
