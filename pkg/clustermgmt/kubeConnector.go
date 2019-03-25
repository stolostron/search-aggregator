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
