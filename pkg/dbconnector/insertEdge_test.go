// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
package dbconnector

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func initTestEdges() []Edge {
	edge1 := Edge{SourceUID: "srcUID1", DestUID: "destUID1",
		EdgeType:   "edgeType1",
		SourceKind: "", DestKind: "destKind1"}
	edge2 := Edge{SourceUID: "srcUID1", DestUID: "destUID2",
		EdgeType:   "edgeType1",
		SourceKind: "srcKind1", DestKind: "destKind2"}
	edge3 := Edge{SourceUID: "srcUID1", DestUID: "destUID3",
		EdgeType:   "edgeType2",
		SourceKind: "srcKind1", DestKind: "destKind3"}
	testEdges := []Edge{edge1, edge2, edge3}
	return testEdges
}

func initTestSingleEdge() []Edge {
	edge1 := Edge{SourceUID: "srcUID1", DestUID: "destUID1",
		EdgeType:   "edgeType1",
		SourceKind: "srcKind1", DestKind: "destKind1"}
	testEdge := []Edge{edge1}
	return testEdge
}

func initTestSingleErrorEdge() []Edge {
	edge1 := Edge{SourceUID: "", DestUID: "destUID1",
		EdgeType:   "edgeType1",
		SourceKind: "srcKind1", DestKind: "destKind1"}
	testEdge := []Edge{edge1}
	return testEdge
}

func initTestErrorEdges() []Edge {
	edge1 := Edge{SourceUID: "srcUID1", DestUID: "destUID1",
		EdgeType:   "edgeType1",
		SourceKind: "srcKind1", DestKind: "destKind1"}
	edge2 := Edge{SourceUID: "srcUID1", DestUID: "destUID2",
		EdgeType:   "edgeType1",
		SourceKind: "srcKind1", DestKind: "destKind2"}
	edge3 := Edge{SourceUID: "", DestUID: "destUID3",
		EdgeType:   "edgeType2",
		SourceKind: "srcKind1", DestKind: "destKind3"}
	testEdges := []Edge{edge1, edge2, edge3}
	return testEdges
}

func insertQueryCheck(q string) bool {
	queries := []string{"MATCH (s:srcKind1 {_uid: 'srcUID1'}), (d) WHERE d._uid='destUID1' OR d._uid='destUID2' CREATE (s)-[:edgeType1]->(d)",
		"MATCH (s:srcKind1 {_uid: 'srcUID1'}), (d:destKind3) WHERE d._uid='destUID3' CREATE (s)-[:edgeType2]->(d)"}
	for _, query := range queries {
		if query == q {
			return true
		}
	}
	return false
}
func TestChunkedInsertEdge(t *testing.T) {
	chunkedOpRes := ChunkedInsertEdge(initTestEdges(), clusterName)
	t.Logf("%+v\n", chunkedOpRes)
	assert.Equal(t, 0, len(chunkedOpRes.ResourceErrors))
	assert.Equal(t, 3, chunkedOpRes.SuccessfulResources)
}

func TestChunkedInsertErrorEdges(t *testing.T) {
	chunkedOpRes := ChunkedInsertEdge(initTestErrorEdges(), clusterName)
	t.Logf("%+v\n", chunkedOpRes)
	assert.Equal(t, 1, len(chunkedOpRes.ResourceErrors))
	assert.Equal(t, 2, chunkedOpRes.SuccessfulResources)
}
