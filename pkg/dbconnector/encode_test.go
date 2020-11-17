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

func TestValidateClusterName(t *testing.T) {
	error1 := ValidateClusterName("test")
	assert.Equal(t, error1, nil, "test")
	error2 := ValidateClusterName("te'st")
	assert.Error(t, error2)
	error3 := ValidateClusterName("te/st")
	assert.Error(t, error3)
	error4 := ValidateClusterName("te.st")
	assert.Error(t, error4)
	error5 := ValidateClusterName("=test")
	assert.Error(t, error5)

}

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
	props["abc"] = "xyz"

	resource := Resource{Kind: "test", UID: "test123", Properties: props}

	result, err := resource.EncodeProperties()

	assert.Equal(t, props, result)
	assert.Equal(t, nil, err, "Must not have errors.")
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

	// case map[string]interfce{}

}
