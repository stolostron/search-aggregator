/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package handlers

import (
	"testing"

	rg "github.com/redislabs/redisgraph-go"
	"github.com/stretchr/testify/assert"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

type MockCache struct {
}

func (mc MockCache) Query(input string) (rg.QueryResult, error) {
	dbhash := [][]string{{"test1"}, {"test2"}, {"test3"}}
	return rg.QueryResult{Results: dbhash}, nil
}

func TestComputeHash(t *testing.T) {
	fakeCache := MockCache{}
	db.Store = fakeCache
	total, hash := computeHash("anyinput")
	assert.Equal(t, 2, total)
	assert.Equal(t, "04bd4dbfb6ccfaa3f5853777ef04acc873bcb877", hash)
}
