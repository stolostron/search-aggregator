/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
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
	if len(graph.Nodes) == 0 && len(graph.Edges) == 0 {
		glog.Info("Flush() was called, but there's no new resources to commit.")
		return nil
	}
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
		return fmt.Errorf("resource is missing 1 or more required field. Required fields are UID, Properties, and Properties[kind].  Resource: %s", resource)
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
				resource.Properties["__label-"+labelKey] = labelValue
			}

		} else if reflect.TypeOf(value).Kind().String() == "slice" {
			// Re-format because RedisGraph doesn't support slice/lists.
			delete(resource.Properties, key)

			for index, itemValue := range value.([]interface{}) {
				resource.Properties[fmt.Sprintf("__%s-%x", key, index)] = itemValue.(string)
			}

		} else if reflect.TypeOf(value).Kind().String() != "string" &&
			reflect.TypeOf(value).Kind().String() != "float64" {

			glog.Warning("Removing property [", key, "] because it's value kind[", reflect.TypeOf(value).Kind(), "] is not a string and it's not supported.")
			delete(resource.Properties, key)
		}
	}

	// Format the resource as a RedisGraph node.
	resource.Properties["cluster"] = resource.Cluster
	resource.Properties["_uid"] = resource.UID

	// Add _rbac property. This will have the format "namespace_apigroup_kind"
	resource.addRbacProperty()

	// Using lowercase to stay compatible with previous release.
	resource.Properties["kind"] = strings.ToLower(resource.Properties["kind"].(string))

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
	// glog.Warning("Delete Results ", res) //RM
	if deleteErr != nil {
		glog.Error("Error deleting resource "+uid+" in RedisGraph.", deleteErr)
		return fmt.Errorf("error deleting resource")
	}

	return nil
}

// Outputs all the redisgraph properties that come out of a given property on a resource.
func encodeProperty(key string, value interface{}) (map[string]interface{}, error) {

	// Sanitize key
	key = strings.Replace(key, ".", "_dot_", -1)
	key = strings.Replace(key, "/", "_slash_", -1)
	key = strings.Replace(key, "=", "_eq_", -1)

	// Sanitize value
	if value == nil || value == "" { // value == "" is false for anything not a string
		return nil, errors.New("Empty Value")
	}

	res := make(map[string]interface{})

	// Switch over all the supported property types.
	switch typedVal := value.(type) {
	case string:
		if !strings.Contains(typedVal, "'") {
			res[key] = typedVal
		}
	case []string:
		for i, str := range typedVal {
			if strings.Contains(str, "'") { // skip any with bad chars
				continue
			}
			res[strings.Join([]string{"__", key, "-", strconv.Itoa(i)}, "")] = str // e.g. __array-0=foo, __array-1=bar, __array-2=baz
		}
	case map[string]string:
		for k, v := range typedVal {
			res[strings.Join([]string{"__", key, "-", k}, "")] = v // e.g. __map-key1=val1, __map-key2=val2, __map-key3=val3
		}
	case int64:
		res[key] = typedVal
	case bool: // as of 4/2/2019 redisgraph does not support bools so we convert to string.
		if typedVal {
			res[key] = "true"
		} else {
			res[key] = "false"
		}
	default:
		return nil, errors.New("Property type unsupported")
	}

	if len(res) == 0 {
		return nil, errors.New("No valid redisgraph properties found")
	}

	return res, nil
}

// Given a resource, output all the redisgraph properties for it in string/interface{} (always string or int64, that's what redisgraph supports) pairs.
func (r Resource) encodeProperties() (map[string]interface{}, error) {
	res := make(map[string]interface{}, len(r.Properties))
	for k, v := range r.Properties {
		// Get all the rg props for this property.
		partial, err := encodeProperty(k, v)
		if err != nil { // if anything went wrong just log a warning and skip it
			glog.Error("Skipping property ", k, " on resource ", r.UID, ": ", err)
			continue
		}

		// Merge all the props that came out into the larger map.
		for pk, pv := range partial {
			res[pk] = pv
		}
	}

	if len(res) == 0 {
		return nil, errors.New("No valid redisgraph properties found")
	}
	return res, nil
}

// Update resources in RedisGraph
// Returns a map from UID to error associated with that UID, for errors on individual resources. Second err return is for the query.
func UpdateResources(resources []Resource) (map[string]error, error) {

	if len(resources) == 0 {
		return nil, nil
	}
	errors := make(map[string]error)

	// Form query string with MATCH and SET to update all the resources at once.
	// Useful doc: https://oss.redislabs.com/redisgraph/commands/#set
	matchStrings := []string{} // Build the MATCH portion
	setStrings := []string{}   // Build the SET portion. Declare this at the same time so that we can do this in one pass.
	for i, resource := range resources {
		matchStrings = append(matchStrings, fmt.Sprintf("(n%d:%s {_uid: '%s'})", i, resource.Properties["kind"], resource.UID)) // e.g. (n0:Pod {_uid: 'abc123'})
		encodedProps, err := resource.encodeProperties()
		if err != nil {
			glog.Error("Cannot update resource ", resource.UID, ": ", err)
			errors[resource.UID] = err
			continue
		}
		for k, v := range encodedProps {
			switch typed := v.(type) { // At this point it's either string or int64. Need to wrap in quotes if it's string
			case int64:
				setStrings = append(setStrings, fmt.Sprintf("n%d.%s=%d", i, k, typed)) // e.g. n0.<key>=<value>
			default:
				setStrings = append(setStrings, fmt.Sprintf("n%d.%s='%s'", i, k, typed)) // e.g. n0.<key>=<value>
			}
		}
	}

	queryString := fmt.Sprintf("%s%s", "MATCH "+strings.Join(matchStrings, ", "), "SET "+strings.Join(setStrings, ", "))

	// glog.Warning(queryString) //RM
	_, err := graph.Query(queryString)
	if err != nil {
		glog.Error("Error when updating resources: ", err)
	}

	if len(errors) != 0 {
		return errors, err
	}
	return nil, err
}
