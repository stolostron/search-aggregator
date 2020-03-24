//# Copyright (c) 2020 Red Hat, Inc

package dbconnector

import (
	"fmt"

	"github.com/golang/glog"
)

// ExistingIndexMap - map to hold all resource kinds that have index built in redisgraph
var ExistingIndexMap = make(map[string]bool)

// GetIndexes() - returns map to hold all resource kinds that have index built in redisgraph
func GetIndexes() {
	resp, err := Store.Query("Match (n) return distinct labels(n)")
	if err == nil {
		for _, kind := range resp.Results[1:] {
			//if the label is not present add to map and set to true
			if _, indexPresent := ExistingIndexMap[kind[0]]; !indexPresent {
				ExistingIndexMap[kind[0]] = true
			}
		}
	} else {
		glog.Error("Error retrieving node labels from redisgraph while creating indices.")
	}

}

// Given a resource, inserts index on resource uid into redisgraph.
func insertIndex(kind, property string) error {
	query := fmt.Sprintf("CREATE INDEX ON :%s(%s)", kind, property) //CREATE INDEX ON :Pod(_uid)"
	_, err := Store.Query(query)
	return err
}
