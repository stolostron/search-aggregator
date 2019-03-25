package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/clustermgmt"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/handlers"
)

func main() {
	// parse flags
	flag.Parse()
	err := flag.Lookup("logtostderr").Value.Set("true") // Glog is weird in that by default it logs to a file. Change it so that by default it all goes to stderr. (no option for stdout).
	if err != nil {
		fmt.Println("Error configuring logger with logtostderr=true flag: ", err) // Using fmt.Println() so we can see this error in the console.
		glog.Error("Error configuring logger with logtostderr=true flag: ", err)
	}
	defer glog.Flush() // This should ensure that everything makes it out on to the console if the program crashes.

	glog.Info("Starting search-aggregator")

	// Watch clusters and sync status to Redis.
	go clustermgmt.WatchClusters()

	router := mux.NewRouter()

	router.HandleFunc("/liveness", handlers.LivenessProbe).Methods("GET")
	router.HandleFunc("/readiness", handlers.ReadinessProbe).Methods("GET")

	router.HandleFunc("/aggregator/status", handlers.GetStatus).Methods("GET")
	router.HandleFunc("/aggregator/clusters/{id}/status", handlers.GetClusterStatus).Methods("GET")
	router.HandleFunc("/aggregator/clusters/{id}/sync", handlers.SyncResources).Methods("POST")

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

	glog.Info("Listening on: https://localhost:3010") // TODO: Use hostname and port from env config.

	if os.Getenv("DEVELOPMENT") == "true" {
		log.Fatal(http.ListenAndServe(":3010", router))
	} else {
		log.Fatal(srv.ListenAndServeTLS("./sslcert/search.crt", "./sslcert/search.key"))
	}
}
