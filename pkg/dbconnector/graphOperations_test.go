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

	"github.com/stretchr/testify/assert"
)

func TestHashesQuery(t *testing.T) {
	actual, _ := hashesQuery("test")
	expected := "MATCH (n) WHERE n.cluster = 'test' RETURN n._hash ORDER BY n._hash ASC"
	assert.Equal(t, actual, expected, "query test")
}

func TestDeleteQuery(t *testing.T) {
	actual, error := deleteClusterQuery("test1")
	expected := "MATCH (n) WHERE n.cluster = 'test1' DELETE n"
	assert.False(t, error != nil)
	assert.Equal(t, actual, expected, "query test")
}
