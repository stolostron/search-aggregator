// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
package dbconnector

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

var clusterName string = "testCluster"

func deleteQueryCheck(q string) bool {
	delMultipleQuery := "MATCH (s0 {_uid: 'srcUID1'})-[e0:edgeType1]->(d0 {_uid: 'destUID1'}), (s1:srcKind1 {_uid: 'srcUID1'})-[e1:edgeType1]->(d1:destKind2 {_uid: 'destUID2'}), (s2:srcKind1 {_uid: 'srcUID1'})-[e2:edgeType2]->(d2:destKind3 {_uid: 'destUID3'}) DELETE e0, e1, e2"
	delSingleQuery := "MATCH (s0:srcKind1 {_uid: 'srcUID1'})-[e0:edgeType1]->(d0:destKind1 {_uid: 'destUID1'}) DELETE e0"
	delChunkedInCaseOfErrorQuery := "MATCH (s0:srcKind1 {_uid: 'srcUID1'})-[e0:edgeType1]->(d0:destKind2 {_uid: 'destUID2'}) DELETE e0"

	if q == delMultipleQuery || q == delSingleQuery || q == delChunkedInCaseOfErrorQuery {
		return true
	}
	return false
}
func TestChunkedDeleteEdge(t *testing.T) {
	chunkedOpRes := ChunkedDeleteEdge(initTestEdges(), clusterName)
	t.Logf("%+v\n", chunkedOpRes)
	assert.Equal(t, 0, len(chunkedOpRes.ResourceErrors))
	assert.Equal(t, 3, chunkedOpRes.SuccessfulResources)
}

func TestChunkedDeleteSingleEdge(t *testing.T) {
	chunkedOpRes := ChunkedDeleteEdge(initTestSingleEdge(), clusterName)
	t.Logf("%+v\n", chunkedOpRes)
	assert.Equal(t, 0, len(chunkedOpRes.ResourceErrors))
	assert.Equal(t, 1, chunkedOpRes.SuccessfulResources)
}

func TestChunkedDeleteSingleErrorEdge(t *testing.T) {
	chunkedOpRes := ChunkedDeleteEdge(initTestSingleErrorEdge(), clusterName)
	t.Logf("%+v\n", chunkedOpRes)
	assert.Equal(t, 1, len(chunkedOpRes.ResourceErrors))
	assert.Equal(t, 0, chunkedOpRes.SuccessfulResources)
}

func TestChunkedDeleteErrorEdges(t *testing.T) {
	chunkedOpRes := ChunkedDeleteEdge(initTestErrorEdges(), clusterName)
	t.Logf("%+v\n", chunkedOpRes)
	assert.Equal(t, 1, len(chunkedOpRes.ResourceErrors))
	assert.Equal(t, 2, chunkedOpRes.SuccessfulResources)
}
