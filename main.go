package main

import (
	"crypto/sha1"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
// TODO: Handle these errors.
//	- Problem connecting to Redis
//	- The received hash doesn't match the current hash.
//	- Add, Update, or Delete operations returned an error.

// type SyncErrorResponse struct {
// 	Message string
// }

// ComputeHash computes a new hash using the hashes from all the current resources.
func ComputeHash(graph *rg.Graph, clusterName string) (totalResources int, hash string) {
	query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' RETURN n._hash ORDER BY n._hash ASC" // TODO: I'll worry about strings later.
	rs, _ := graph.Query(query)

	allHashes := rs.Results[1:] // Start at index 1 because index 0 has the header.

	h := sha1.New()
	_, err := h.Write([]byte(fmt.Sprintf("%x", allHashes))) // TODO: I'll worry about strings later.
	if err != nil {
		fmt.Println("Error generating hash.")
	}
	bs := h.Sum(nil)

	totalResources = len(allHashes)
	hash = fmt.Sprintf("%x", bs)
	fmt.Println("Total resources:", totalResources)
	fmt.Println("All hashes:", fmt.Sprintf("%s", allHashes)) // TODO: I'll worry about strings later.
	fmt.Println("Current hash:", hash)
	return totalResources, hash
}

// livenessProbe
func livenessProbe(w http.ResponseWriter, r *http.Request) {
	fmt.Println("livenessProbe()")
	var status = Status{
		Message: "OK",
	}
	err := json.NewEncoder(w).Encode(status)
	if err != nil {
		fmt.Println("Error responding to livenessProbe", err)
	}
}

// readinessProbe
func readinessProbe(w http.ResponseWriter, r *http.Request) {
	fmt.Println("readinessProbe() - TODO: Check RedisGraph is available.")
	// TODO: Check RedisGraph is available.
	var status = Status{
		Message: "OK",
	}
	err := json.NewEncoder(w).Encode(status)
	if err != nil {
		fmt.Println("Error responding to readinessProbe", err)
	}
}

// GetStatus responds with the global status.
func GetStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Println("GetStatus() - TODO: Respond with all clusters and their last sync time and current hash.")
	var status = Status{
		Message:       "TODO: This will respond with all clusters and their last sync time and current hash.",
		TotalClusters: 99, // TODO: Get total clusters from Redis
	}
	err := json.NewEncoder(w).Encode(status)
	if err != nil {
		fmt.Println("Error responding to GetStatus", err, status)
	}
}

// GetClusterStatus responds with the cluster status.
func GetClusterStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		fmt.Println("Error responding to GetClusterStatus:", encodeError, response)
	}
}

// SyncResources - Process Add, Update, and Delete events.
func SyncResources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	fmt.Println("SyncResources() for cluster:", clusterName)

	conn, _ := redis.Dial("tcp", "0.0.0.0:6379") //TODO: Make configurable
	defer conn.Close()
	graph := rg.Graph{}.New("icp-search", conn)

	var syncEvent SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		fmt.Println("Error decoding body of syncEvent:", err)
	}

	if syncEvent.ClearAll {
		query := "MATCH (n) WHERE n.cluster = '" + clusterName + "' DELETE n"
		_, err := graph.Query(query)
		if err != nil {
			fmt.Println("Error running RedisGraph delete query:", err, query)
		}
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
		// TODO: Need special processing for lists (labels and roles).

		resource.Properties["kind"] = resource.Kind
		resource.Properties["cluster"] = clusterName
		resource.Properties["_uid"] = resource.UID
		resource.Properties["_hash"] = resource.Hash
		resource.Properties["_rbac"] = "UNKNOWN" // TODO: This must be the namespace of the cluster.

		err := graph.AddNode(&rg.Node{
			ID:         resource.UID, // FIXME: This is supported by RedisGraph but doesn't work in the redisgraph-go client.
			Label:      resource.Kind,
			Properties: resource.Properties,
		})
		if err != nil {
			fmt.Println("Error adding resource node:", err, resource)
		}
	}
	_, error := graph.Flush()
	if error != nil {
		fmt.Println("Error adding nodes in RedisGraph.", error)
		// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
	}

	// UPDATE resources
	for _, resource := range updateResources {
		fmt.Println("Updating resource: ", resource)
		// FIXME: Properly update resource. Deleting and recreating is very lazy.
		query := "MATCH (n) WHERE n._uid = '" + resource.UID + "' DELETE n"

		_, err := graph.Query(query)
		if err != nil {
			fmt.Println("Error executing query:", err, query)
		}
		resource.Properties["kind"] = resource.Kind
		resource.Properties["cluster"] = clusterName
		resource.Properties["_uid"] = resource.UID
		resource.Properties["_hash"] = resource.Hash
		resource.Properties["_rbac"] = "UNKNOWN" // TODO: This must be the namespace of the cluster.

		error := graph.AddNode(&rg.Node{
			ID:         resource.UID, // FIXME: This doesn't work in the redisgraph-go client.
			Label:      resource.Kind,
			Properties: resource.Properties,
		})
		if error != nil {
			fmt.Println("Error updating resource node:", error, resource)
		}
	}
	_, updateErr := graph.Flush()
	if updateErr != nil {
		fmt.Println("Error updating nodes in RedisGraph.", updateErr)
		// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
	}

	// DELETE resources
	for _, resource := range deleteResources {
		fmt.Println("Deleting resource: ", resource)
		query := "MATCH (n) WHERE n._uid = '" + resource.UID + "' DELETE n"
		_, deleteErr := graph.Query(query)
		if deleteErr != nil {
			fmt.Println("Error deleting nodes in RedisGraph.", deleteErr)
			// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
		}
	}

	// Updating cluster status in cache.
	updatedTimestamp := time.Now()
	totalResources, currentHash := ComputeHash(&graph, clusterName)
	var clusterStatus = []interface{}{fmt.Sprintf("cluster:%s", clusterName)} // TODO: I'll worry about strings later.
	clusterStatus = append(clusterStatus, "hash", currentHash)
	clusterStatus = append(clusterStatus, "lastUpdated", updatedTimestamp)

	_, deleteErr := conn.Do("HMSET", clusterStatus...)

	if deleteErr != nil {
		fmt.Println("Error deleting nodes in RedisGraph.", deleteErr)
		// TODO: Error handling.  We should allow partial failures, but this will add complexity to the sync logic.
	}

	var response = SyncResponse{
		Hash:             currentHash,
		TotalAdded:       len(addResources),
		TotalChanged:     len(updateResources),
		TotalDeleted:     len(deleteResources),
		TotalResources:   totalResources,
		UpdatedTimestamp: updatedTimestamp,
		Message:          "TODO: Maybe we don't need this message field.",
	}
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		fmt.Println("Error responding to SyncEvent:", encodeError, response)
	}
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/liveness", livenessProbe).Methods("GET")
	router.HandleFunc("/readiness", readinessProbe).Methods("GET")

	router.HandleFunc("/aggregator/status", GetStatus).Methods("GET")
	router.HandleFunc("/aggregator/clusters/{id}/status", GetClusterStatus).Methods("GET")
	router.HandleFunc("/aggregator/clusters/{id}/sync", SyncResources).Methods("POST")

	// Configure TLS
	cfg := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			// TODO: Update list with acceptable FIPS ciphers.
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
	srv := &http.Server{
		Addr:         ":3010",
		Handler:      router,
		TLSConfig:    cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	fmt.Println("Starting search-aggregator")
	fmt.Println("Listening on: https://localhost:3010") // TODO: Use hostname and port from env config.

	if os.Getenv("DEVELOPMENT") == "true" {
		log.Fatal(http.ListenAndServe(":3010", router))
	} else {
		log.Fatal(srv.ListenAndServeTLS("./sslcert/search.crt", "./sslcert/search.key"))
	}
}
