// Copyright (c) 2020 Red Hat, Inc.

package config

import (
	"testing"
)

func TestGetKubeClient(t *testing.T) {
	dynamicClient := GetKubeClient()

	if dynamicClient == nil {
		t.Error("Error getting Kube Client")
	}
}

func TestGetDynamicClient(t *testing.T) {
	dynamicClient := GetDynamicClient()

	if dynamicClient == nil {
		t.Error("Error getting Dynamic Client")
	}
}

func TestGetDiscoveryClient(t *testing.T) {
	dynamicClient := GetDiscoveryClient()

	if dynamicClient == nil {
		t.Error("Error getting Discovery Client")
	}
}
