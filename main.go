package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

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
	// TODO: AddEdges, DeleteEdges
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
	Hash             string
	TotalAdded       int
	TotalChanged     int
	TotalDeleted     int
	TotalResources   int
	UpdatedTimestamp time.Time
	Message          string
}

// SyncErrorResponse - Used to report errors during sync.
type SyncErrorResponse struct {
	Message string
}

// ComputeHash computes a new hash using the hashes from all the current resources.
func ComputeHash(graph *rg.Graph, clusterName string) (totalResources int, hash string) {
	query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' RETURN n._hash ORDER BY n._hash ASC" // TODO: I'll worry about strings later.
	rs, _ := graph.Query(query)

	allHashes := rs.Results[1:] // Start at index 1 because index 0 has the header.

	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("%x", allHashes))) // TODO: I'll worry about strings later.
	bs := h.Sum(nil)

	totalResources = len(allHashes)
	hash = fmt.Sprintf("%x", bs)
	fmt.Println("Total resources:", totalResources)
	fmt.Println("All hashes:", fmt.Sprintf("%s", allHashes)) // TODO: I'll worry about strings later.
	fmt.Println("Current hash:", hash)
	return totalResources, hash
}

// GetStatus responds with the global status.
func GetStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GetStatus() - TODO: Respond with all clusters and their last sync time and current hash.")
	var status = Status{
		Message:       "TODO: This will respond with all clusters and their last sync time and current hash.",
		TotalClusters: 99, // TODO: Get total clusters from Redis
	}
	json.NewEncoder(w).Encode(status)
}

// GetClusterStatus responds with the cluster status.
func GetClusterStatus(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	clusterName := params["id"]
	fmt.Println("GetClusterStatus() for cluster:", clusterName)

	conn, _ := redis.Dial("tcp", "0.0.0.0:6379") //TODO: Make configurable
	defer conn.Close()
	graph := rg.Graph{}.New("icp-search", conn)

	clusterStatus, err := conn.Do("HGETALL", fmt.Sprintf("cluster:%s", clusterName)) // TODO: I'll worry about strings later.
	if err != nil {
		fmt.Println("Error getting status of cluster"+clusterName+" from Redis.", err)
	}
	var status = []interface{}{clusterStatus}
	// fmt.Println("Cluster status:", clusterStatus)
	fmt.Println("Cluster status:", status[0])

	totalResources, currentHash := ComputeHash(&graph, clusterName)

	var response = ClusterStatus{
		Hash:           currentHash,
		Message:        "ClusterStatus",
		LastUpdated:    "TODO: Get lastUpdated timestamp from Redis.",
		TotalResources: totalResources,
	}
	json.NewEncoder(w).Encode(response)
}

// SyncResources - Process Add, Update, and Delete events.
func SyncResources(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	clusterName := params["id"]
	fmt.Println("SyncResources() for cluster:", clusterName)

	conn, _ := redis.Dial("tcp", "0.0.0.0:6379") //TODO: Make configurable
	defer conn.Close()
	graph := rg.Graph{}.New("icp-search", conn)

	var syncEvent SyncEvent
	_ = json.NewDecoder(r.Body).Decode(&syncEvent)

	if syncEvent.ClearAll == true {
		query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' DELETE n"
		graph.Query(query)
		fmt.Println("!!! Deleted all previous resources for cluster:", clusterName)
	}

	addResources := syncEvent.AddResources
	updateResources := syncEvent.UpdateResources
	deleteResources := syncEvent.DeleteResources

	// ADD resources
	for _, resource := range addResources {
		fmt.Println("Adding resource: ", resource)

		// TODO: Enforce required values (Kind, UID, Hash)
		// TODO: Do I need to sanitize inputs?
		// TODO: Need special processing for labels and roles.

		resource.Properties["kind"] = resource.Kind
		resource.Properties["cluster"] = clusterName
		resource.Properties["_uid"] = resource.UID
		resource.Properties["_hash"] = resource.Hash
		resource.Properties["_resourceVersion"] = resource.Hash // TODO: Temporary, remove after migrating to use `_hash`.

		graph.AddNode(&rg.Node{
			ID:         resource.UID,
			Label:      resource.Kind,
			Properties: resource.Properties,
		})
	}
	_, error := graph.Flush()
	if error != nil {
		fmt.Println("Error adding nodes in RedisGraph.", error)
	}

	// UPDATE resources
	for _, resource := range updateResources {
		fmt.Println("Updating resource: ", resource)
		// FIXME: Properly update resource. Deleting and recreating is very lazy.
		query := "MATCH (n) WHERE n._uid = '" + resource.UID + "' DELETE n"

		graph.Query(query)
		resource.Properties["kind"] = resource.Kind
		resource.Properties["cluster"] = clusterName
		resource.Properties["_uid"] = resource.UID
		resource.Properties["_hash"] = resource.Hash
		resource.Properties["_resourceVersion"] = resource.Hash // FIXME: Temporary, remove after migrating to use hash.

		graph.AddNode(&rg.Node{
			ID:         resource.UID,
			Label:      resource.Kind,
			Properties: resource.Properties,
		})
	}
	_, updateErr := graph.Flush()
	if updateErr != nil {
		fmt.Println("Error updating nodes in RedisGraph.", updateErr)
	}

	// DELETE resources
	for _, resource := range deleteResources {
		fmt.Println("Deleting resource: ", resource)
		query := "MATCH (n) WHERE n._uid = '" + resource.UID + "' DELETE n"
		graph.Query(query)
	}

	// Updating cluster status in cache.
	updatedTimestamp := time.Now()
	totalResources, currentHash := ComputeHash(&graph, clusterName)
	var clusterStatus = []interface{}{fmt.Sprintf("cluster:%s", clusterName)} // TODO: I'll worry about strings later.
	clusterStatus = append(clusterStatus, "hash", currentHash)
	clusterStatus = append(clusterStatus, "lastUpdated", updatedTimestamp)

	_, err := conn.Do("HMSET", clusterStatus...)

	if err != nil {
		fmt.Println("error", err)
	}

	var response = SyncResponse{
		Hash:             currentHash,
		TotalAdded:       len(addResources),
		TotalChanged:     0, //len(updateResources),
		TotalDeleted:     0, //len(deleteResources),
		TotalResources:   totalResources,
		UpdatedTimestamp: updatedTimestamp,
		Message:          "TODO: Maybe we don't need this message field.",
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
