// Copyright (c) 2020 Red Hat, Inc.

package clustermgmt

import (
	"testing"

	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/cluster/v1beta1"
	utils "github.com/open-cluster-management/search-aggregator/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func Test_transformManagedCluster(t *testing.T) {
	managedCluster := clusterv1.ManagedCluster{}
	utils.UnmarshalFile("managedCluster.json", &managedCluster, t)

	result := transformManagedCluster(&managedCluster)

	assert.Equal(t, "Amazon", (result.Properties["label"]).(map[string]interface{})["cloud"], "Test property: label")
	assert.Equal(t, "internal.open-cluster-management.io", result.Properties["apigroup"], "Test property: apigroup")
	assert.Equal(t, "Cluster", result.Kind, "Test property: Kind")
	assert.Equal(t, "managed-cluster-01", result.Properties["name"], "Test property: name")
	assert.Equal(t, "2020-11-10T22:46:08Z", result.Properties["created"], "Test property: created")
	assert.Equal(t, int64(36), result.Properties["cpu"], "Test property: cpu")
	assert.Equal(t, "144576Mi", result.Properties["memory"], "Test property: memory")
	assert.Equal(t, "v1.18.3+6c42de8", result.Properties["kubernetesVersion"], "Test property: kubernetesVersion")

	assert.Equal(t, "True", result.Properties["HubAcceptedManagedCluster"], "Test property: HubAcceptedManagedCluster")
	assert.Equal(t, "True", result.Properties["ManagedClusterJoined"], "Test property: ManagedClusterJoined")
	assert.Equal(t, "True", result.Properties["ManagedClusterConditionAvailable"], "Test property: ManagedClusterConditionAvailable")

	assert.Equal(t, "managedclusterinfos", result.ResourceString, "Test property: ResourceString")
	assert.Equal(t, "cluster__managed-cluster-01", result.UID, "Test property: UID")
}

func Test_transformManagedClusterInfo(t *testing.T) {
	managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
	utils.UnmarshalFile("managedClusterInfo.json", &managedClusterInfo, t)

	result := transformManagedClusterInfo(&managedClusterInfo)

	assert.Equal(t, "internal.open-cluster-management.io", result.Properties["apigroup"], "Test property: apigroup")
	assert.Equal(t, result.Kind, "Cluster", "Test Kind")
	assert.Equal(t, "managed-cluster-01", result.Properties["name"], "Test property: name")
	assert.Equal(t, "https://console-openshift-console.apps.base-host-name.com", result.Properties["consoleURL"], "Test property: consoleURL")
	assert.Equal(t, int64(6), result.Properties["nodes"], "Test property: nodes")

	assert.Equal(t, "managedclusterinfos", result.ResourceString, "Test property: ResourceString")
	assert.Equal(t, "cluster__managed-cluster-01", result.UID, "Test property: UID")
}
