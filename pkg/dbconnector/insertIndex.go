//# Copyright (c) 2020 Red Hat, Inc

package dbconnector

import (
	"sync"

	"github.com/golang/glog"
)

// ExistingIndexMap - map to hold all resource kinds that have index built in redisgraph
var ExistingIndexMap = make(map[string]bool)

// GetIndexes - returns map to hold all resource kinds that have index built in redisgraph
func GetIndexes() {
	resp, err := Store.Query("MATCH (n) RETURN distinct labels(n)")
	if err == nil {
		var ExistingIndexMapMutex = sync.RWMutex{}
		if resp.Results != nil {
			for _, kind := range resp.Results[1:] {
				//if the label is not present add to map and set to true
				ExistingIndexMapMutex.RLock()
				exists := ExistingIndexMap[kind[0]]
				ExistingIndexMapMutex.RUnlock()

				if !exists {
					ExistingIndexMapMutex.Lock() // Lock map before writing
					ExistingIndexMap[kind[0]] = true
					ExistingIndexMapMutex.Unlock() // Unlock map after writing
				}
			}
		}
	} else {
		glog.Error("Error retrieving node labels from redisgraph while creating indices.")
	}

}

// Given a resource, inserts index on resource uid into redisgraph.
func insertIndex(kind, property string) error {
	query := SanitizeQuery("CREATE INDEX ON :%s(%s)", kind, property) //CREATE INDEX ON :Pod(_uid)"
	_, err := Store.Query(query)
	return err
}
