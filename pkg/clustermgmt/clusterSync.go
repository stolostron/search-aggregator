/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package clustermgmt

import (
	"time"

	"github.com/golang/glog"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type clusterResourceEvent struct {
	EventType string
	Resource  unstructured.Unstructured
}

// TODO: Commented out for linting, will enable soon.
// type clusterCacheRecord struct {
// 	Name      string
// 	Namespace string
// }

// var clustersCache map[string]clusterCacheRecord
var syncChannel chan *clusterResourceEvent

func syncRoutine(input chan *clusterResourceEvent) {
	glog.Info("Starting cluster sync routine.")
	dbconnector.Init()

	for {
		evt := <-input // Read from the clusterEvt channel

		if evt.EventType == "Add" && evt.Resource.GetKind() == "Cluster" {
			props := make(map[string]interface{})
			props["kind"] = evt.Resource.GetKind()
			props["name"] = evt.Resource.GetName()
			props["namespace"] = evt.Resource.GetNamespace()
			props["selfLink"] = evt.Resource.GetSelfLink()
			props["created"] = evt.Resource.GetCreationTimestamp().UTC().Format(time.RFC3339)

			// FIXME: Labels is causing problems because of the type:  map[string]string  vs. map[string]interface{}
			// props["label"] = evt.Resource.GetLabels()

			resource := dbconnector.Resource{
				Kind:       "Cluster",
				UID:        string(evt.Resource.GetUID()),
				Properties: props,
			}

			glog.Info("Inserting Cluster resource in RedisGraph. ", resource)
			err := dbconnector.Insert(&resource)
			if err != nil {
				glog.Error("Error inserting cluster object in database. ", err)
			}
			errFlush := dbconnector.Flush()
			if errFlush != nil {
				glog.Error("Error flushing records in RedisGraph. ", errFlush)
			}
		}
		if evt.EventType == "Add" && evt.Resource.GetKind() == "ClusterStatus" {
			glog.Warning("TODO: Process ClusterStatus add event.")
		}
		if evt.EventType == "Update" {
			glog.Warning("TODO: Process Cluster or ClusterStatus update event.")
		}
		if evt.EventType == "Delete" {
			glog.Warning("TODO: Process Cluster or ClusterStatus delete event.")
		}
	}
}
