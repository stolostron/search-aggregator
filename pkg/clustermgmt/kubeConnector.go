/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package clustermgmt

import (
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var config *rest.Config
var clientConfigError error

// Init initializes the kube client.
func InitKubeConnector() {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		glog.Info("Creating k8s client using config from KUBECONFIG")
		config, clientConfigError = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".kube", "config")); os.IsNotExist(err) {
		glog.Info("Creating k8s client using InClusterConfig()")
		config, clientConfigError = rest.InClusterConfig()
	} else {
		glog.Info("Creating k8s client using config from ~/.kube/config")
		config, clientConfigError = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	}

	if clientConfigError != nil {
		glog.Fatal("Error Constructing Client From Config: ", clientConfigError)
	}
}
