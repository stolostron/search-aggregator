module github.com/open-cluster-management/search-aggregator

go 1.13

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/gorilla/mux v1.7.4
	github.com/kennygrant/sanitize v1.2.4
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/open-cluster-management/api v0.0.0-20200623215229-19a96fed707a
	github.com/open-cluster-management/multicloud-operators-foundation v0.0.0-20200629084830-3965fdd47134
	github.com/redislabs/redisgraph-go v2.0.2+incompatible
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.0.0-20200302210943-78000ba7a073 // indirect
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v13.0.0+incompatible
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	github.com/coreos/etcd => github.com/coreos/etcd v3.3.24+incompatible
	github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.1.0
	github.com/docker/docker => github.com/docker/docker v1.13.1
	github.com/hashicorp/consul => github.com/hashicorp/consul v1.7.4
	github.com/openshift/origin => github.com/openshift/origin v1.2.0
	github.com/prometheus/prometheus => github.com/prometheus/prometheus v2.7.1+incompatible
	github.com/terraform-providers/terraform-provider-azurerm => github.com/terraform-providers/terraform-provider-azurerm v0.0.0-20200604143437-d38893bc4f78
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.7
	k8s.io/apiserver => k8s.io/apiserver v0.17.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4
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
	k8s.io/kubernetes => k8s.io/kubernetes v1.17.11
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.4
	k8s.io/metrics => k8s.io/metrics v0.17.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.1
	sigs.k8s.io/cluster-api-provider-aws => sigs.k8s.io/cluster-api-provider-aws v0.4.0
	sigs.k8s.io/cluster-api-provider-azure => sigs.k8s.io/cluster-api-provider-azure v0.4.0
	sigs.k8s.io/cluster-api-provider-openstack => sigs.k8s.io/cluster-api-provider-openstack v0.3.0
)
