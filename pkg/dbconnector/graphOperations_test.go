/*
 * (C) Copyright IBM Corporation 2019 All Rights Reserved
 * Copyright Contributors to the Open Cluster Management project
*/
package dbconnector

import (
	"errors"
	"testing"

	rg2 "github.com/redislabs/redisgraph-go"
	"github.com/stretchr/testify/assert"
)

type MockCache struct {
}

func init() {
	Store = MockCache{}
}
func (mc MockCache) Query(q string) (*rg2.QueryResult, error) {
	if q == "MATCH (n {cluster:'good-cluster-name'}) DELETE n" || insertQueryCheck(q) || deleteQueryCheck(q) {
		return &rg2.QueryResult{}, nil
	}
	return &rg2.QueryResult{}, errors.New("Incorrect Query formed")
}

func TestDeleteCluster(t *testing.T) {
	_, err := DeleteCluster("good-cluster-name")
	assert.NoError(t, err)

}
func TestBadDeleteCluster(t *testing.T) {
	_, err := DeleteCluster("bad-cluster=name")
	assert.Error(t, err)
}

func TestMergeDummyCluster(t *testing.T) {
	_, err := MergeDummyCluster("fake-cluster")
	assert.Error(t, err)
}
