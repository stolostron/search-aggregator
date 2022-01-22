module github.com/stolostron/search-aggregator

go 1.15

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/gorilla/mux v1.7.4
	github.com/kennygrant/sanitize v1.2.4
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/redislabs/redisgraph-go v2.0.2+incompatible
	github.com/stolostron/multicloud-operators-foundation v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2 // indirect
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v13.0.0+incompatible
	open-cluster-management.io/api v0.6.0
)

replace (
	github.com/buger/jsonparser => github.com/buger/jsonparser v1.1.1
	github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.1.0
	github.com/docker/docker => github.com/docker/docker v1.13.1
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	github.com/hashicorp/terraform => github.com/openshift/terraform v0.12.20-openshift-4
	github.com/hashicorp/terraform-plugin-sdk => github.com/openshift/hashicorp-terraform-plugin-sdk v1.14.0-openshift
	github.com/hashicorp/terraform@v0.13.4 => github.com/openshift/terraform v0.12.20-openshift-4
	github.com/kubevirt/terraform-provider-kubevirt => github.com/kubevirt/terraform-provider-kubevirt v0.0.0-20210628085519-5c4934a8bda8
	github.com/metal3-io/baremetal-operator => github.com/metal3-io/baremetal-operator v0.0.0-20211220105604-05d12b6768a9
	github.com/metal3-io/baremetal-operator/apis => github.com/metal3-io/baremetal-operator/apis v0.0.0-20220119160837-9b26aa7816ca
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils => github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.0.0-20220119160837-9b26aa7816ca //indirect
	github.com/metal3-io/cluster-api-provider-baremetal => github.com/openshift/cluster-api-provider-baremetal v0.0.0-20220104154407-0716ee4cb33b
	github.com/openshift/client-go => github.com/openshift/client-go v3.9.0+incompatible
	github.com/openshift/hive/apis => github.com/openshift/hive/apis v0.0.0-20220121012553-a0671aa97ef3
	github.com/stolostron/multicloud-operators-foundation => github.com/stolostron/multicloud-operators-foundation v1.0.0-2021-10-26-20-16-14.0.20220110023249-172fb944faa9
	github.com/terraform-providers/terraform-provider-azurerm => github.com/terraform-providers/terraform-provider-azurerm v0.0.0-20200604143437-d38893bc4f78
	github.com/terraform-providers/terraform-provider-ignition/v2 => github.com/community-terraform-providers/terraform-provider-ignition/v2 v2.1.0
	golang.org/x/text => golang.org/x/text v0.3.5
	k8s.io/api => k8s.io/api v0.21.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.1
	k8s.io/apiserver => k8s.io/apiserver v0.17.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.21.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.4
	k8s.io/cloud-provides => k8s.io/cloud-provides v0.17.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.4
	k8s.io/code-generator => k8s.io/code-generator v0.17.4
	k8s.io/component-base => k8s.io/component-base v0.17.4
	k8s.io/cri-api => k8s.io/cri-api v0.17.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.4
	k8s.io/kubectl => k8s.io/kubectl v0.17.4
	k8s.io/kubelet => k8s.io/kubelet v0.17.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.4
	k8s.io/metrics => k8s.io/metrics v0.17.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.1
	kubevirt.io/client-go => kubevirt.io/client-go v0.49.0
	sigs.k8s.io/cluster-api-provider-aws => sigs.k8s.io/cluster-api-provider-aws v0.4.0
	sigs.k8s.io/cluster-api-provider-azure => sigs.k8s.io/cluster-api-provider-azure v0.4.0
	sigs.k8s.io/cluster-api-provider-openstack => sigs.k8s.io/cluster-api-provider-openstack v0.3.0
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.10.0
)
