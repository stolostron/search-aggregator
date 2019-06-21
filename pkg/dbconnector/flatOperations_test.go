/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package dbconnector

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestGetClusterStatusArgs(t *testing.T) {
	actual, err := getClusterStatusArgs("test")
	expected := []interface{}{"cluster:test"}
	assert.Equal(t, actual, expected, "args test")
	assert.NoError(t, err)
}
