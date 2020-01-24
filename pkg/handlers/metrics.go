/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package handlers

import (
	"sync"
	"time"

	"github.com/golang/glog"
)

// Track clusters with pending requests.
var (
	PendingRequests      = make(map[string]*SyncMetrics)
	PendingRequestsMutex = sync.RWMutex{}
)

// Used to collect sync performance metrics
type SyncMetrics struct {
	clusterName   string
	syncStart     time.Time // Entire cluster sync event
	SyncEnd       time.Time // Entire cluster sync event
	NodeSyncStart time.Time
	NodeSyncEnd   time.Time
	EdgeSyncStart time.Time
	EdgeSyncEnd   time.Time
}

func InitSyncMetrics(clusterName string) SyncMetrics {
	s := SyncMetrics{clusterName: clusterName, syncStart: time.Now()}
	PendingRequestsMutex.Lock()
	PendingRequests[clusterName] = &s
	PendingRequestsMutex.Unlock()
	return s
}

func (m SyncMetrics) CompleteSyncEvent() {
	glog.V(2).Info("Completed sync of cluster: ", m.clusterName)
	PendingRequestsMutex.Lock()
	delete(PendingRequests, m.clusterName)
	PendingRequestsMutex.Unlock()
}

func (m SyncMetrics) LogPerformanceMetrics(syncEvent SyncEvent) {
	elapsed := time.Since(m.syncStart)
	if int(elapsed.Seconds()) > 1 {
		glog.Warningf("SyncResources from %s took %s", m.clusterName, elapsed)
		glog.Warningf("Increased Processing time with { request: %d, add: %d, update: %d, delete: %d edge add: %d edge delete: %d }", syncEvent.RequestId, len(syncEvent.AddResources), len(syncEvent.UpdateResources), len(syncEvent.DeleteResources), len(syncEvent.AddEdges), len(syncEvent.DeleteEdges))
		glog.Warning("  > Nodes sync took: ", m.NodeSyncEnd.Sub(m.NodeSyncStart))
		glog.Warning("  > Edges sync took: ", m.EdgeSyncEnd.Sub(m.EdgeSyncStart))
	} else {
		glog.V(4).Infof("SyncResources from %s took %s", m.clusterName, elapsed)
		glog.V(4).Info("  > Nodes sync took: ", m.NodeSyncEnd.Sub(m.NodeSyncStart))
		glog.V(4).Info("  > Edges sync took: ", m.EdgeSyncEnd.Sub(m.EdgeSyncStart))
	}
}
