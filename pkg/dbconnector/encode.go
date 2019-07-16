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
	"strings"
)

// Tells whether the given clusterName is valid, i.e. has no illegal characters and isn't empty
func ValidateClusterName(clusterName string) error {
	if len(clusterName) == 0 {
		return errors.New("Order contains blank ClusterName")
	}
	if strings.Contains(clusterName, "/") || strings.Contains(clusterName, ".") || strings.Contains(clusterName, "=") || strings.Contains(clusterName, "'") {
		return errors.New("Order contains ClusterName with illegal characters: /, ., =, or '")
	}
	return nil
}

// Given a resource, output all the redisgraph properties for it in map[string]interface{} (always string or int64, that's what redisgraph supports) pairs.
func (r Resource) encodeProperties() (map[string]interface{}, error) {
	res := make(map[string]interface{}, len(r.Properties))
	for k, v := range r.Properties {
		// Get all the rg props for this property.
		if k == "label" { // Labels will be supported in a future release.
			continue
		}
		partial, err := encodeProperty(k, v)
		if err != nil { // if anything went wrong just log a warning and skip it
			// glog.Warning("Skipping property ", k, " on resource ", r.UID, ": ", err)
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

// Outputs all the redisgraph properties that come out of a given property on a resource.
// Outputs exclusively in our supported types: string, []string, map[string]string, and int64.
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

	// Switch over all the default json.Unmarshal types. These are the only possible types that could be in the map. For each, we go through and convert to what we want them to be.
	// Useful doc regarding default types: https://golang.org/pkg/encoding/json/#Unmarshal
	switch typedVal := value.(type) {
	case string:
		if !strings.Contains(typedVal, "'") {
			if key == "kind" { // we lowercase the kind.
				res[key] = strings.ToLower(typedVal)
			} else {
				res[key] = typedVal
			}
		}
	case []interface{}:
		elementStrings := make([]string, 0, len(typedVal))
		for _, e := range typedVal {
			elementString := fmt.Sprintf("%v", e)
			if strings.Contains(elementString, "'") { // skip any with bad chars
				continue
			}
			elementStrings = append(elementStrings, elementString)
		}

		res[key] = strings.Join(elementStrings, ", ") // e.g. val1, val2, val3

	/*
		// TODO maps will be supported in a later release - they require some special logic around updating, since the property keys aren't predefined.
		case map[string]interface{}:
			for k, v := range typedVal {
				switch typedElement := v.(type) {
				case string:
					if strings.Contains(typedElement, "'") { // skip any with bad chars
						continue
					}
					res[fmt.Sprintf("__%s-%s", key, k)] = v // e.g. __map-key1=val1, __map-key2=val2, __map-key3=val3
				default: // Skip anything not a string.
					continue
				}
			}
	*/
	case int64:
		res[key] = typedVal
	case float64: // As of 4/15/2019 we don't have any numerical properties that aren't ints.
		res[key] = int64(typedVal)
	case bool: // as of 4/2/2019 redisgraph does not support bools so we convert to string.
		if typedVal {
			res[key] = "true"
		} else {
			res[key] = "false"
		}
	default:
		return nil, fmt.Errorf("Property type unsupported: %s %v", reflect.TypeOf(typedVal), typedVal)
	}

	if len(res) == 0 {
		return nil, errors.New("No valid redisgraph properties found")
	}

	return res, nil
}
