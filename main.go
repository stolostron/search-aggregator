package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	rg "github.com/redislabs/redisgraph-go"
)

// Status - Global status of all clusters.
type Status struct {
	TotalClusters int
	Message       string
}

// ClusterStatus - Status of a single cluster.
type ClusterStatus struct {
	Hash           string
	LastUpdated    string
	TotalResources int
	MaxQueueTime   int
	Message        string
}

// SyncEvent - Object sent by the collector with the resourcces to change.
type SyncEvent struct {
	Hash            string `json:"hash,omitempty"`
	ClearAll        bool
	AddResources    []AddResourceEvent
	UpdateResources []UpdateResourceEvent
	DeleteResources []DeleteResourceEvent
}

// AddResourceEvent - Contains the information needed to add a new resource.
type AddResourceEvent struct {
	Kind       string `json:"kind,omitempty"`
	UID        string `json:"uid,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Properties map[string]interface{}
}

// UpdateResourceEvent - Contains the information needed to update an existing resource.
type UpdateResourceEvent struct {
	Kind       string `json:"kind,omitempty"`
	UID        string `json:"uid,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Properties map[string]interface{}
}

// DeleteResourceEvent - Contains the information needed to delete an existing resource.
type DeleteResourceEvent struct {
	UID string `json:"uid,omitempty"`
}

// TODO:
// AddEdgeEvent
// DeleteEdgeEvent

// SyncResponse - Response to a SyncEvent
type SyncResponse struct {
	Hash           string
	TotalAdded     int
	TotalChanged   int
	TotalDeleted   int
	TotalResources int
	Message        string
}

// SyncErrorResponse - Used to report errors during sync.
type SyncErrorResponse struct {
	Message string
}

// GetStatus responds with the global status.
func GetStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetStatus() - TODO: Will return all clusters with their last sync time and current hash.")
	var status = Status{
		Message:       "TODO: Will return all clusters with their last sync time and current hash.",
		TotalClusters: 99, // TODO: Get total clusters from Redis
	}
	json.NewEncoder(w).Encode(status)
}

// GetClusterStatus responds with the cluster status.
func GetClusterStatus(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	clusterName := params["id"]
	fmt.Println("GetClusterStatus() for cluster:", clusterName)

	conn, _ := redis.Dial("tcp", "0.0.0.0:6379")
	defer conn.Close()
	graph := rg.Graph{}.New("mcm-search", conn)

	query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' RETURN n._resourceVersion" // TODO: Change resourceVersion to hash
	rs, _ := graph.Query(query)

	allHashes := rs.Results[0:]

	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("%x", allHashes)))
	bs := h.Sum(nil)

	fmt.Println("Total objects: ", len(allHashes))
	fmt.Println(fmt.Sprintf("%s", allHashes))
	fmt.Printf("Current Hash: %x\n", bs)

	var response = ClusterStatus{
		Hash:           fmt.Sprintf("%x", bs),
		Message:        "ClusterStatus",
		LastUpdated:    "TODO: Save lastUpdated to management db and return value here.",
		TotalResources: len(allHashes),
	}
	json.NewEncoder(w).Encode(response)
}

// SyncResources - Process Add, Update, and Delete events.
func SyncResources(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	clusterName := params["id"]
	fmt.Println("SyncResources() for cluster:", clusterName)

	var syncEvent SyncEvent
	_ = json.NewDecoder(r.Body).Decode(&syncEvent)

	addResources := syncEvent.AddResources
	updateResources := syncEvent.UpdateResources
	deleteResources := syncEvent.DeleteResources

	fmt.Println("Adding resources: ", addResources)
	fmt.Println("Updating resources: ", updateResources)
	fmt.Println("Deleting resources: ", deleteResources)

	var response = SyncResponse{
		Hash:           "TODO: return newHash",
		TotalAdded:     len(addResources),
		TotalChanged:   len(updateResources),
		TotalDeleted:   len(deleteResources),
		TotalResources: 99, // TODO: Get from RedisGraph
		Message:        "TODO: Synchronize resources with RedisGraph.",
	}
	json.NewEncoder(w).Encode(response)
}

// main function
func main() {
	router := mux.NewRouter()
	router.HandleFunc("/status", GetStatus).Methods("GET")
	router.HandleFunc("/clusters/{id}/status", GetClusterStatus).Methods("GET")
	router.HandleFunc("/clusters/{id}/sync", SyncResources).Methods("POST")

	log.Fatal(http.ListenAndServe(":8000", router))
}
