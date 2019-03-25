package dbconnector

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"
	rg "github.com/redislabs/redisgraph-go"
)

// Resource - Describes a resource
type Resource struct {
	Kind       string `json:"kind,omitempty"`
	UID        string `json:"uid,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Cluster    string `json:"cluster,omitempty"`
	Properties map[string]interface{}
}

// Flush writes changes to db and clears local client.
func Flush() error {
	_, err := graph.Flush()
	if err != nil {
		glog.Error("Error commiting new resources to RedisGraph.", err)
	}
	return err
}

// Insert resource in RedisGraph
func Insert(resource *Resource) error {
	// Check required fields.
	if resource.UID == "" || resource.Properties == nil || resource.Properties["kind"] == nil {
		return fmt.Errorf("resource is missing 1 or more required field. Required fields are UID, Properties, and Properties[kind]")
	}

	// Sanitize and re-format inputs.
	for key, value := range resource.Properties {

		// Sanitize keys.
		if strings.Contains(key, ".") || strings.Contains(key, "=") || strings.Contains(key, "/") {
			glog.Warning("Resource: [", resource.Properties["name"], "]. Removing property [", key, "] because it contains unsafe characters..")
			delete(resource.Properties, key)
		}

		// Sanitize values.
		if value == nil || value == "" {
			glog.Warning("Resource: [", resource.Properties["name"], "]. Removing property [", key, "] because it's value is nil or empty string.")
			delete(resource.Properties, key)
		}
		if reflect.TypeOf(value).Kind().String() == "string" {
			if strings.Contains(value.(string), "'") { // TODO: Add other unsafe characters.
				glog.Warning("Resource: [", resource.Properties["name"], "]. Removing property[", key, "] because it's value[", value, "] contains unsafe characters.")
				delete(resource.Properties, key)
			}
		} else if key == "label" && reflect.TypeOf(value).Kind().String() == "map" {
			// Re-fromat labels because RedisGraph doesn't support lists.
			delete(resource.Properties, key)

			// glog.Info("Resource: ", resource.Properties["name"], " (", resource.Properties["kind"], ")")
			for labelKey, labelValue := range value.(map[string]interface{}) {
				// glog.Info("  > Label: ", labelKey, "=", labelValue)
				labelKey = strings.Replace(labelKey, "/", "_slash_", -1) // FIXME: Need a better way to do this.
				labelKey = strings.Replace(labelKey, ".", "_dot_", -1)   // FIXME: Need a better way to do this.
				labelKey = strings.Replace(labelKey, "=", "_eq_", -1)    // FIXME: Need a better way to do this.
				resource.Properties["_label__"+labelKey] = labelValue
			}

			// } else if reflect.TypeOf(value).Kind().String() == "slice" {
			// 	glog.Warning("Removing property [" + key + "] because it's value is a slice and it's not supported.")
			// 	delete(resource.Properties, key)

		} else if reflect.TypeOf(value).Kind().String() != "string" &&
			reflect.TypeOf(value).Kind().String() != "float64" {

			glog.Warning("Removing property [", key, "] because it's value kind[", reflect.TypeOf(value).Kind(), "] is not a string and it's not supported.")
			delete(resource.Properties, key)
		}
	}

	// Format the resource as a RedisGraph node.
	resource.Properties["cluster"] = resource.Cluster
	resource.Properties["_uid"] = resource.UID

	// FIXME: Need to revisit RBAC.
	//  -> 1. Need to use namespace of cluster.
	//  -> 2. Resources from hub cluster that doesn't have a namespace.
	if resource.Properties["namespace"] != "" && resource.Properties["namespace"] != nil {
		resource.Properties["_rbac"] = resource.Properties["namespace"]
	}

	err := graph.AddNode(&rg.Node{
		ID:         resource.UID, // NOTE: This is supported by RedisGraph but doesn't work in the redisgraph-go client.
		Label:      resource.Properties["kind"].(string),
		Properties: resource.Properties,
	})

	if err != nil {
		glog.Error("Error adding resource node:", err, resource)
		return fmt.Errorf("error addding resource in RedisGraph")
	}
	return nil
}

// DeleteCluster deletes all resources for the cluster.
func DeleteCluster(clusterName string) error {
	query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' DELETE n"
	_, err := graph.Query(query)
	if err != nil {
		glog.Error("Error running RedisGraph delete query:", err, query)
		return fmt.Errorf("error deleting all resources for cluster " + clusterName)
	}
	glog.Info("Deleted all previous resources for cluster:", clusterName)
	return nil
}

// Delete will delete the resource from RedisGraph
func Delete(uid string) error {
	query := "MATCH (n) WHERE n._uid = '" + uid + "' DELETE n"
	_, deleteErr := graph.Query(query)
	if deleteErr != nil {
		glog.Error("Error deleting resource "+uid+" in RedisGraph.", deleteErr)
		return fmt.Errorf("error deleting resource")
	}
	return nil
}

// Update resources in RedisGraph
func Update(resource *Resource) error {

	// FIXME: Properly update resource. Deleting and recreating is inefficient and will cause problems with edges.

	delError := Delete(resource.UID)
	if delError != nil {
		glog.Error("Error deleting resource (from update)", delError)
		return delError
	}

	insertError := Insert(resource)
	if insertError != nil {
		glog.Error("Error inserting resource(from update)", insertError)
		return insertError
	}
	return nil
}
