/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.

Copyright (c) 2020 Red Hat, Inc.
*/
// Copyright Contributors to the Open Cluster Management project

package dbconnector

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Tells whether the given clusterName is valid, i.e. has no illegal characters and isn't empty
func ValidateClusterName(clusterName string) error {
	if len(clusterName) == 0 {
		return errors.New("clusterName must not be empty.")
	}
	if strings.Contains(clusterName, "/") || strings.Contains(clusterName, ".") ||
		strings.Contains(clusterName, "=") || strings.Contains(clusterName, "'") {
		return errors.New("clusterName contains illegal characters: /, ., =, or '")
	}
	return nil
}

// Given a resource, output all the properties for it in map[string]interface{}
// (always string or int64, that's what Redisgraph supports) pairs.
func (r Resource) EncodeProperties() (map[string]interface{}, error) {
	res := make(map[string]interface{}, len(r.Properties))
	for k, v := range r.Properties {
		// Get all the rg props for this property.
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
// Outputs exclusively in our supported types: string, []string, map[string]string, and int64 and []interface.
func encodeProperty(key string, value interface{}) (map[string]interface{}, error) {

	// Sanitize value
	if value == nil || value == "" { // value == "" is false for anything not a string
		return nil, errors.New("Empty Value")
	}

	res := make(map[string]interface{})

	// Switch over all the default json.Unmarshal types. These are the only possible types that could be in the map.
	// For each, we go through and convert to what we want them to be.
	// Useful doc regarding default types: https://golang.org/pkg/encoding/json/#Unmarshal
	switch typedVal := value.(type) {
	case string:
		if key == "kind" { // we lowercase the kind.
			res[key] = strings.ToLower(sanitizeValue(typedVal))
		} else {
			res[key] = sanitizeValue(typedVal)
		}

	case []interface{}:
		// RedisGraph 2.2 supports a list of properties.
		// we are encoding as a list of values with individually quoted strings
		elementStrings := make([]string, 0, len(typedVal))
		for _, e := range typedVal {
			elementString := fmt.Sprintf("'%v'", sanitizeValue(fmt.Sprintf("%v", e)))
			elementStrings = append(elementStrings, elementString)
		}
		sort.Strings(elementStrings)                       // Sorting to make comparisons more predictable
		sanitizedStr := strings.Join(elementStrings, ", ") // e.g. 'val1', 'val2', 'val3'
		tmpInterface := make([]interface{}, 1)             //store the value as list to allow partial matching
		tmpInterface[0] = sanitizedStr
		res[key] = tmpInterface

	case map[string]interface{}:
		if key == "label" || key == "addonStatus" {
			labelStrings := make([]string, 0, len(typedVal))
			for key, value := range typedVal {
				labelString := fmt.Sprintf("'%v=%v'", sanitizeValue(key), sanitizeValue(fmt.Sprintf("%v", value)))
				labelStrings = append(labelStrings, labelString)
			}
			sort.Strings(labelStrings)                       // Sorting to make comparisons more predictable
			sanitizedStr := strings.Join(labelStrings, ", ") // e.g. 'key1=val1', 'key2=val2', 'key3=val3'
			tmpInterface := make([]interface{}, 1)           //store the value as list to allow partial matching
			tmpInterface[0] = sanitizedStr
			res[key] = tmpInterface
		}

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
