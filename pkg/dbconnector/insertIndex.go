//# Copyright (c) 2020 Red Hat, Inc

package dbconnector

import (
	"fmt"

	"github.com/golang/glog"
)

// ExistingIndexMap - map to hold all resource kinds that have index built in redisgraph
var ExistingIndexMap = make(map[string]struct{})

// InsertKindUIDIndexes fetches all existing labels in the graph, transparently builds query for you and returns the response and errors given by redisgraph.
// Returns the result, any errors when inserting, and any error from the query itself.
func InsertKindUIDIndexes() ChunkedOperationResult {
	resp, err := Store.Query("Match (n) return distinct labels(n)")
	totalSuccessful := 0
	insertErrors := make(map[string]error)
	if err == nil {
		for _, kind := range resp.Results[1:] {
			if _, indexPresent := ExistingIndexMap[kind[0]]; !indexPresent {
				_, err := insertIndex(kind[0], "_uid")
				if err != nil {
					glog.Warning("Cannot insert index for ", kind[0], ", excluding it from insertion: ", err)
					insertErrors[kind[0]] = err
					continue
				} else {
					totalSuccessful++
					ExistingIndexMap[kind[0]] = struct{}{}
				}
			}
		}
	} else {
		glog.Warning("Error retrieving node labels from redisgraph while creating indices.")
	}
	ret := ChunkedOperationResult{
		ResourceErrors:      insertErrors,
		SuccessfulResources: totalSuccessful,
	}
	return ret
}

// Given a resource, inserts index on resource uid into redisgraph.
func insertIndex(kind, property string) (QueryResult, error) {
	query := fmt.Sprintf("CREATE INDEX ON :%s(%s)", kind, property) //CREATE INDEX ON :Pod(_uid)"
	resp, err := Store.Query(query)
	return resp, err
}
