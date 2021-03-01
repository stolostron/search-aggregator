/*
 * (C) Copyright IBM Corporation 2019 All Rights Reserved
 * Copyright (c) 2020 Red Hat, Inc.
 * Copyright Contributors to the Open Cluster Management project
*/

package config

import (
	"github.com/golang/glog"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	kubeClientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeClient - Client to get jobs resource
func GetKubeClient() *kubeClientset.Clientset {
	kubeClient, err := kubeClientset.NewForConfig(getClientConfig())
	if kubeClient == nil || err != nil {
		glog.Error("Error getting the kube clientset. ", err)
	}
	return kubeClient
}

// Discovery Client
func GetDiscoveryClient() *discovery.DiscoveryClient {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(getClientConfig())
	if err != nil {
		glog.Warning("Error getting the discovery client. ", err)
	}
	return discoveryClient
}

// Dynamic Client
func GetDynamicClient() dynamic.Interface {
	dynamicClientset, err := dynamic.NewForConfig(getClientConfig())
	if err != nil {
		glog.Warning("Error getting the dynamic client. ", err)
	}
	return dynamicClientset
}

func getClientConfig() *rest.Config {
	var clientConfig *rest.Config
	var err error
	if Cfg.KubeConfig != "" {
		glog.V(1).Infof("Creating k8s client using path: %s", Cfg.KubeConfig)
		clientConfig, err = clientcmd.BuildConfigFromFlags("", Cfg.KubeConfig)
	} else {
		clientConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Warning("Error getting the kube client config. ", err)
		return &rest.Config{}
	}
	return clientConfig
}
