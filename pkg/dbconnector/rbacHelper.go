/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"strings"

	"github.com/golang/glog"
)

// The rbac string is defined as "namespace_apigroup_kind".  For non-namespaced resources
// or resources without an apigroup we'll use the null string, for example: "null_null_kind1"
func (r *Resource) addRbacProperty() {
	rbac := []string{"null", "null", "null"}

	if r.Properties == nil { // init props if it was nil
		r.Properties = make(map[string]interface{})
	}

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
			if t != nil {
				glog.Warning("Property 'namespace' must be a string or nil.  Got invalid value from resource: ", r)
			}
		}
	} else {
		switch t := r.Properties["_clusterNamespace"].(type) {
		case string:
			if t != "" {
				rbac[0] = t
			}
		default:
			// rbac[0] is already initialized to the string "null".
			if t != nil {
				glog.Warning("Property '_clusterNamespace' must be a string or nil.  Got invalid value from resource: ", r)
			}
		}
	}

	// Get the apigroup.
	switch t := r.Properties["apigroup"].(type) {
	case string:
		if t != "" {
			rbac[1] = t
		}
	default:
		// rbac[1] is already initialized to the string "null".
		if t != nil {
			glog.Warning("Property 'apigroup' must be a string or nil. Got invalid value from resource: ", r)
		}
	}

	// Get the kind.
	if r.ResourceString == "" {
		glog.Warning("Received a resource with an empty ResourceString.  Resource: ", r)
	}
	rbac[2] = r.ResourceString

	r.Properties["_rbac"] = strings.Join(rbac, "_")
}
