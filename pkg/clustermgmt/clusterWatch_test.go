/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.
*/
// Copyright (c) 2020 Red Hat, Inc.

package clustermgmt

import (
	"testing"

	"github.com/stretchr/testify/assert"

	mcmapi "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/mcm/v1alpha1"
	utils "github.com/open-cluster-management/search-aggregator/pkg/utils"
	// hive "github.com/openshift/hive/pkg/apis/hive/v1"
	batch "k8s.io/api/batch/v1"
	clusterregistry "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
)

func TestTransformCluster(t *testing.T) {
	testcluster := clusterregistry.Cluster{}
	testclusterstatus := mcmapi.ClusterStatus{}
	utils.UnmarshalFile("cluster.json", &testcluster, t)
	utils.UnmarshalFile("clusterstatus.json", &testclusterstatus, t)
	result := transformCluster(&testcluster, &testclusterstatus)
	assert.Equal(t, result.Kind, "Cluster", "Test Kind")
	assert.Equal(t, result.ResourceString, "clusters", "Test ResourceString")
	assert.Equal(t, result.UID, "1baa5f8a-758f-11e9-9527-667a72062d69", "Test UID")
	assert.Equal(t, result.Properties["name"], "xav-cluster", "Test Name")
	assert.Equal(t, result.Properties["namespace"], "xav-cluster-ns", "Test namespace")
	assert.Equal(t, (result.Properties["label"]).(map[string]interface{})["cloud"], "IBM", "Test label")
	assert.Equal(t, result.Properties["created"], "2019-05-13T14:55:11Z", "Test created")
	assert.Equal(t, result.Properties["consoleURL"], "https://222.222.222.222:8443", "Test consoleURL")
	assert.Equal(t, result.Properties["cpu"], int64(24), "Test cpu")
	assert.Equal(t, result.Properties["memory"], "98143Mi", "Test memory")
	assert.Equal(t, result.Properties["nodes"], int64(3), "Test nodes")
	assert.Equal(t, result.Properties["storage"], "60Gi", "Test storage")

}

// func TestGetStatusCreating(t *testing.T) {
// 	testcluster := &clusterregistry.Cluster{}
// 	testclusterstatus := &mcmapi.ClusterStatus{}
// 	testinstalljob := &batch.JobList{}
// 	testuninstalljob := &batch.JobList{}

// 	testcd := hive.ClusterDeployment{}
// 	clustStat := ClusterStat{cluster: testcluster, clusterStatus: testclusterstatus, uninstallJobs: testuninstalljob, installJobs: testinstalljob, clusterdeployment: &testcd}
// 	utils.UnmarshalFile("cluster2.json", &testcluster, t)
// 	utils.UnmarshalFile("clusterstatus.json", &testclusterstatus, t)
// 	utils.UnmarshalFile("clusterinstalljob.json", &testinstalljob, t)
// 	utils.UnmarshalFile("clustercd.json", &testcd, t)

// 	result := getStatus(clustStat)
// 	assert.Equal(t, result, "creating", "Test Status")
// }

// func TestGetStatusPending(t *testing.T) {
// 	testcluster := &clusterregistry.Cluster{}
// 	testclusterstatus := &mcmapi.ClusterStatus{}
// 	testinstalljob := &batch.JobList{}
// 	testuninstalljob := &batch.JobList{}

// 	testcd := hive.ClusterDeployment{}
// 	clustStat := ClusterStat{cluster: testcluster, clusterStatus: testclusterstatus, uninstallJobs: testuninstalljob, installJobs: testinstalljob, clusterdeployment: &testcd}
// 	utils.UnmarshalFile("cluster2.json", &testcluster, t)
// 	utils.UnmarshalFile("cluster-cd-detached.json", &testcd, t)

// 	result := getStatus(clustStat)
// 	assert.Equal(t, result, "pending", "Test Status")
// }

// func TestGetStatusDetaching(t *testing.T) {
// 	testcluster := &clusterregistry.Cluster{}
// 	testclusterstatus := &mcmapi.ClusterStatus{}
// 	testinstalljob := &batch.JobList{}
// 	testuninstalljob := &batch.JobList{}

// 	testcd := hive.ClusterDeployment{}
// 	clustStat := ClusterStat{cluster: testcluster, clusterStatus: testclusterstatus, uninstallJobs: testuninstalljob, installJobs: testinstalljob, clusterdeployment: &testcd}
// 	utils.UnmarshalFile("cluster-detaching.json", &testcluster, t)
// 	utils.UnmarshalFile("clustercd.json", &testcd, t)

// 	result := getStatus(clustStat)
// 	assert.Equal(t, result, "detaching", "Test Status")
// }

// func TestGetStatusUnknown(t *testing.T) {
// 	testcluster := &clusterregistry.Cluster{}
// 	testclusterstatus := &mcmapi.ClusterStatus{}
// 	testinstalljob := &batch.JobList{}
// 	testuninstalljob := &batch.JobList{}

// 	testcd := hive.ClusterDeployment{}
// 	clustStat := ClusterStat{cluster: testcluster, clusterStatus: testclusterstatus, uninstallJobs: testuninstalljob, installJobs: testinstalljob, clusterdeployment: &testcd}
// 	utils.UnmarshalFile("cluster2.json", &testcluster, t)
// 	utils.UnmarshalFile("clustercd.json", &testcd, t)

// 	result := getStatus(clustStat)
// 	assert.Equal(t, result, "unknown", "Test Status")
// }

// func TestGetStatusOffline(t *testing.T) {
// 	testcluster := &clusterregistry.Cluster{}
// 	testclusterstatus := &mcmapi.ClusterStatus{}
// 	testinstalljob := &batch.JobList{}
// 	testuninstalljob := &batch.JobList{}

// 	testcd := hive.ClusterDeployment{}
// 	clustStat := ClusterStat{cluster: testcluster, clusterStatus: testclusterstatus, uninstallJobs: testuninstalljob, installJobs: testinstalljob, clusterdeployment: &testcd}
// 	utils.UnmarshalFile("cluster-offline.json", &testcluster, t)
// 	utils.UnmarshalFile("cluster-cd-detached.json", &testcd, t)

// 	result := getStatus(clustStat)
// 	assert.Equal(t, result, "offline", "Test Status")
// }

func TestJobActive(t *testing.T) {

	testjob := &batch.JobList{}

	utils.UnmarshalFile("clusterinstalljob.json", &testjob, t)
	assert.Equal(t, chkJobActive(testjob, "install"), "creating", "Test Status")
	assert.Equal(t, chkJobActive(testjob, "uninstall"), "destroying", "Test Status")
}
