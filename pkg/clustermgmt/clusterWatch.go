package clustermgmt

import (
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// WatchClusters watches k8s cluster and clusterstatus objects and updates the search cache.
func WatchClusters() {
	defer handleRoutineExit()

	InitKubeConnector()

	stopper := make(chan struct{})
	defer close(stopper)

	// Create a channel to process Cluster and Clusterstatus events.
	syncChannel = make(chan *clusterResourceEvent)
	go syncRoutine(syncChannel)

	// Initialize the dynamic client, used for CRUD operations on nondeafult k8s resources
	dynamicClientset, err := dynamic.NewForConfig(config)
	if err != nil {
		glog.Fatal("Cannot Construct Dynamic Client From Config: ", err)
	}
	// Create informer factories
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientset, 0)

	// Watch Cluster resources
	clusterResource := schema.GroupVersionResource{
		Group:    "clusterregistry.k8s.io",
		Resource: "clusters",
		Version:  "v1alpha1",
	}

	clusterInformer := dynamicFactory.ForResource(clusterResource).Informer()
	clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			resource := obj.(*unstructured.Unstructured)
			event := clusterResourceEvent{
				EventType: "Add",
				Resource:  *resource,
			}
			syncChannel <- &event
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			resource := next.(*unstructured.Unstructured)
			event := clusterResourceEvent{
				EventType: "Update",
				Resource:  *resource,
			}
			syncChannel <- &event
		},
		DeleteFunc: func(obj interface{}) {
			resource := obj.(*unstructured.Unstructured)
			event := clusterResourceEvent{
				EventType: "Delete",
				Resource:  *resource,
			}
			syncChannel <- &event
		},
	})
	go clusterInformer.Run(stopper)

	// Watch clusterstatus resources.
	clusterStatusResource := schema.GroupVersionResource{
		Group:    "mcm.ibm.com",
		Resource: "clusterstatuses",
		Version:  "v1alpha1",
	}

	clusterStatusInformer := dynamicFactory.ForResource(clusterStatusResource).Informer()
	clusterStatusInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			resource := obj.(*unstructured.Unstructured)
			event := clusterResourceEvent{
				EventType: "Add",
				Resource:  *resource,
			}
			syncChannel <- &event
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			resource := next.(*unstructured.Unstructured)
			event := clusterResourceEvent{
				EventType: "Update",
				Resource:  *resource,
			}
			syncChannel <- &event
		},
		DeleteFunc: func(obj interface{}) {
			resource := obj.(*unstructured.Unstructured)
			event := clusterResourceEvent{
				EventType: "Delete",
				Resource:  *resource,
			}
			syncChannel <- &event
		},
	})

	clusterStatusInformer.Run(stopper)
}

// Handles a panic from inside WatchClusters routine.
// If the panic was due to an error, starts another WatchClusters routine.
func handleRoutineExit() {
	if r := recover(); r != nil { // Case where we got here from a panic
		glog.Errorf("Error in WatchClusters routine: %v\n", r)

		go WatchClusters()
	}
}
