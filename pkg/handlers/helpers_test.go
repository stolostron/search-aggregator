/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package handlers

import (
	"testing"

	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
	rg2 "github.com/redislabs/redisgraph-go"
	"github.com/stretchr/testify/assert"
)

type MockCache struct {
}

func (mc MockCache) Query(input string) (*rg2.QueryResult, error) {
	//res := [][]string{{"Header"}, {"100"}}

	return &rg2.QueryResult{}, nil
}

func TestNodeCount(t *testing.T) {
	fakeCache := MockCache{}
	db.Store = fakeCache

	count := computeNodeCount("anyinput")
	assert.Equal(t, 0, count)
}
