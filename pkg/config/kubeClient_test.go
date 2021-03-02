// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package config

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// This prevents using the default /.kube/config when running tests locally
	Cfg.KubeConfig = "some-path"
	os.Exit(m.Run())
}

func TestGetKubeClient(t *testing.T) {
	kubeClient := GetKubeClient()
	if kubeClient == nil {
		t.Error("Failed testing Kube Client.")
	}
}

func TestGetDynamicClient(t *testing.T) {
	dynamicClient := GetDynamicClient()
	if dynamicClient == nil {
		t.Error("Failed testing  Dynamic Client.")
	}
}

func TestGetDiscoveryClient(t *testing.T) {
	dynamicClient := GetDiscoveryClient()
	if dynamicClient == nil {
		t.Error("Failed testing  Discovery Client.")
	}
}
