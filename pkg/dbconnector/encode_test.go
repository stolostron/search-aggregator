/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.

Copyright (c) 2020 Red Hat, Inc.
*/
package dbconnector

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func Test_ValidateClusterName(t *testing.T) {

	error1 := ValidateClusterName("test")
	assert.Equal(t, error1, nil, "test")

	error2 := ValidateClusterName("te'st")
	assert.Equal(t, "clusterName contains illegal characters: /, ., =, or '", error2.Error(), "te'st")

	error3 := ValidateClusterName("te/st")
	assert.Equal(t, "clusterName contains illegal characters: /, ., =, or '", error3.Error(), "te/st")

	error4 := ValidateClusterName("te.st")
	assert.Equal(t, "clusterName contains illegal characters: /, ., =, or '", error4.Error(), "te.st")

	error5 := ValidateClusterName("=test")
	assert.Equal(t, "clusterName contains illegal characters: /, ., =, or '", error5.Error(), "=test")

	error6 := ValidateClusterName("")
	assert.Equal(t, "clusterName must not be empty.", error6.Error(), "empty string")
}

func Test_EncodeProperties(t *testing.T) {
	props := make(map[string]interface{})

	// Test empty properties
	resource := Resource{Kind: "test", UID: "test1", Properties: props}
	result1, error1 := resource.EncodeProperties()
	assert.Equal(t, map[string]interface{}(nil), result1)
	assert.Equal(t, "No valid redisgraph properties found", error1.Error())

	// Test with properties to encode.
	props["abc"] = "xyz"
	resource2 := Resource{Kind: "test", UID: "test2", Properties: props}
	result2, error2 := resource2.EncodeProperties()
	assert.Equal(t, props, result2)
	assert.Equal(t, nil, error2, "Must not have errors.")
}

func Test_encodeProperty(t *testing.T) {

	result1, error1 := encodeProperty("emptyValue", "")
	assert.Equal(t, "Empty Value", error1.Error(), "Test empty value.")
	assert.Equal(t, nil, result1["emptyValue"])

	// case string
	result2, error2 := encodeProperty("kind", "SomeKindValue")
	assert.Equal(t, "somekindvalue", result2["kind"], "Should converted kind value to lowercase")
	assert.Equal(t, nil, error2)

	result3, error3 := encodeProperty("someKey", "some'Value")
	assert.Equal(t, "some\\'Value", result3["someKey"], "Should sanitize string values with single quotes")
	assert.Equal(t, nil, error3)

	result4, error4 := encodeProperty("someKey", "some\"Value")
	assert.Equal(t, "some\\\"Value", result4["someKey"], "Should sanitize string values with double quotes")
	assert.Equal(t, nil, error4)

	// case []interface{}
	list := make([]interface{}, 2)
	list[0] = "value1"
	list[1] = "value2"
	result5, error5 := encodeProperty("list", list)
	assert.Equal(t, "value1, value2", result5["list"], "Should encode array into a single string.")
	assert.Equal(t, nil, error5)

	// case map[string]interfce{}
	mapValue := make(map[string]interface{})
	mapValue["key1"] = "value1"
	mapValue["key2"] = "value2"
	result6, error6 := encodeProperty("label", mapValue)
	assert.Equal(t, "key1=value1; key2=value2", result6["label"], "Should encode labels map into a single string.")
	assert.Equal(t, nil, error6)

	// case int64
	result7, error7 := encodeProperty("int64", int64(123))
	assert.Equal(t, int64(123), result7["int64"], "Should encode int64 into int64.")
	assert.Equal(t, nil, error7)

	// case float64
	result8, error8 := encodeProperty("float64", float64(12.3))
	assert.Equal(t, int64(12), result8["float64"], "Should encode float64 into int64.")
	assert.Equal(t, nil, error8)

	// case bool
	result9, error9 := encodeProperty("boolean_true", true)
	assert.Equal(t, "true", result9["boolean_true"], "Should encode boolean into string (true).")
	assert.Equal(t, nil, error9)

	result10, error10 := encodeProperty("boolean_false", false)
	assert.Equal(t, "false", result10["boolean_false"], "Should encode boolean into string (false).")
	assert.Equal(t, nil, error10)

	// case default

	result11, error11 := encodeProperty("default", []string{})
	assert.Equal(t, nil, result11["default"], "Should print error if received property is unsupported.")
	assert.Equal(t, "Property type unsupported: []string []", error11.Error())
}
