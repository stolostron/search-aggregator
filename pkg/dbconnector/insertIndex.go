// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package dbconnector

import (
	"sync"

	"github.com/golang/glog"
)

// ExistingIndexMap - map to hold all resource kinds that have index built in redisgraph
var ExistingIndexMap = make(map[string]bool)

// GetIndexes - returns map to hold all resource kinds that have index built in redisgraph
func GetIndexes() {
	glog.V(4).Info("Fetching indexes")
	resp, err := Store.Query("MATCH (n) RETURN distinct labels(n)")
	if err == nil {
		var ExistingIndexMapMutex = sync.RWMutex{}
		if !resp.Empty() {
			for resp.Next() {
				record := resp.Record()
				for _, kindVal := range record.Values() {
					if kind, ok := kindVal.(string); ok {

						//if the label is not present add to map and set to true
						ExistingIndexMapMutex.RLock()
						exists := ExistingIndexMap[kind]
						ExistingIndexMapMutex.RUnlock()

						if !exists {
							ExistingIndexMapMutex.Lock() // Lock map before writing
							ExistingIndexMap[kind] = true
							ExistingIndexMapMutex.Unlock() // Unlock map after writing
						}
					}
				}
			}
		}
	} else {
		glog.Error("Error retrieving node labels from redisgraph while creating indices.")
	}

}

// Given a resource, inserts index on resource uid into redisgraph.
func insertIndex(kind, property string) error {
	glog.V(4).Info("Inserting index")
	query := SanitizeQuery("CREATE INDEX ON :%s(%s)", kind, property) //CREATE INDEX ON :Pod(_uid)"
	_, err := Store.Query(query)
	glog.V(4).Info("Insert index query: ", query)
	return err
}
