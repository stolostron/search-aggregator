package dbconnector

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestInsertEdgeQuery(t *testing.T) {
	edge := Edge{
		SourceUID: "srcUIDABC", DestUID: "destUIDXYZ",
		EdgeType:   "testEdge",
		SourceKind: "srcKindABC", DestKind: "destKindXYZ"}

	actualQuery := insertEdgeQuery(edge, "")
	expectedQuery := "MATCH (s:srcKindABC {_uid: 'srcUIDABC'}), (d:destKindXYZ {_uid: 'destUIDXYZ'}) CREATE (s)-[:testEdge]->(d)"

	assert.Equal(t, actualQuery, expectedQuery)
}
