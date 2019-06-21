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
	"testing"

	"github.com/stretchr/testify/assert"

	rg "github.com/redislabs/redisgraph-go"
)

type MockCache struct {
	goodQuery string
	ret       rg.QueryResult
}

func (mc MockCache) Query(input string) (rg.QueryResult, error) {
	if input == mc.goodQuery {
		return mc.ret, nil
	}
	return rg.QueryResult{}, errors.New("Incorrect Query formed")
}

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

func TestHashes(t *testing.T) {

	mc := MockCache{}                                                                                    // Construct mock cache using type defined above
	mc.goodQuery = "MATCH (n) WHERE n.cluster = 'good-cluster-name' RETURN n._hash ORDER BY n._hash ASC" // Dictate what the next input to mocked Query
	Store = mc
	resp, clusterNameErr, err := Hashes("good-cluster-name")
	assert.NoError(t, err)
	assert.NoError(t, clusterNameErr)
	assert.NotNil(t, resp)
}
func TestHashesBadClusterName(t *testing.T) {

	mc := MockCache{}                                                                                    // Construct mock cache using type defined above
	mc.goodQuery = "MATCH (n) WHERE n.cluster = 'good-cluster-name' RETURN n._hash ORDER BY n._hash ASC" // Dictate what the next input to mocked Query
	Store = mc
	_, clusterNameErr, _ := Hashes("bad-cluster=name")
	assert.Error(t, clusterNameErr)
}

func TestDeleteCluster(t *testing.T) {

	mc := MockCache{}                                                         // Construct mock cache using type defined above
	mc.goodQuery = "MATCH (n) WHERE n.cluster = 'good-cluster-name' DELETE n" // Dictate what the next input to mocked Query
	Store = mc
	resp, clusterNameErr, err := DeleteCluster("good-cluster-name")
	assert.NoError(t, err)
	assert.NoError(t, clusterNameErr)
	assert.NotNil(t, resp)

}
func TestBadDeleteCluster(t *testing.T) {

	mc := MockCache{}                                                                                    // Construct mock cache using type defined above
	mc.goodQuery = "MATCH (n) WHERE n.cluster = 'good-cluster-name' RETURN n._hash ORDER BY n._hash ASC" // Dictate what the next input to mocked Query
	Store = mc
	_, clusterNameErr, _ := DeleteCluster("bad-cluster=name")
	assert.Error(t, clusterNameErr)
}
