package dbconnector

import (
	"strings"

	"github.com/golang/glog"
)

// The rbac string is defined as "namespace_apigroup_kind".  For non-namespaced resources
// or resources without an apigroup we'll use the null string, for example: "null_null_kind1"
func (r *Resource) addRbacProperty() {
	rbac := []string{"null", "null", "null"}

	// Get the namespace.
	// For resources in the hub cluster we use their namespace.
	// When a resource is on a remote cluster we will use the namespace mapped to that cluster.
	if r.Properties["cluster"] == "local-cluster" {
		switch t := r.Properties["namespace"].(type) {
		case string:
			if t != "" {
				rbac[0] = t
			}
		default:
			// rbac[0] is already initialized to the string "null".
			glog.Warning("Property 'namespace' must be a string.  Got invalid value from resource: ", r)
		}
	} else {
		glog.Error("TODO: Need to use the namespace mapped to the remote cluster.")
	}

	// Get the apigroup.
	switch t := r.Properties["apigroup"].(type) {
	case string:
		if t != "" {
			rbac[1] = t
		}
	default:
		// rbac[1] is already initialized to the string "null".
		glog.Warning("Property 'apigroup' must be a string. Got invalid value from resource: ", r)
	}

	// Get the kind.
	// TODO: Use r.Kind  (Currently this is empty, must initialize before I can use it here.)
	switch t := r.Properties["kind"].(type) {
	case string:
		if t != "" {
			rbac[2] = t
		}
	default:
		// rbac[2] is already initialized to the string "null".
		glog.Warning("Property 'kind' must be a string. Got invalid value from resource: ", r)
	}

	r.Properties["_rbac"] = strings.Join(rbac, "_")
}
