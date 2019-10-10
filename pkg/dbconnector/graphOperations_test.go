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
)

type MockCache struct {
	goodQuery string
	ret       QueryResult
}

func (mc MockCache) Query(input string) (QueryResult, error) {
	if input == mc.goodQuery {
		return mc.ret, nil
	}
	return QueryResult{}, errors.New("Incorrect Query formed")
}

func TestDeleteCluster(t *testing.T) {

	mc := MockCache{}                                                         // Construct mock cache using type defined above
	mc.goodQuery = "MATCH (n) WHERE n.cluster = 'good-cluster-name' DELETE n" // Dictate what the next input to mocked Query
	Store = mc
	resp, err := DeleteCluster("good-cluster-name")
	assert.NoError(t, err)
	assert.NotNil(t, resp)

}
func TestBadDeleteCluster(t *testing.T) {

	mc := MockCache{}                                                         // Construct mock cache using type defined above
	mc.goodQuery = "MATCH (n) WHERE n.cluster = 'good-cluster-name' DELETE n" // Dictate what the next input to mocked Query
	Store = mc
	_, err := DeleteCluster("bad-cluster=name")
	assert.Error(t, err)
}
