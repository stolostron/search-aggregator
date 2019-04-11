/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package main

import (
	"crypto/tls"
	"log"
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/clustermgmt"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/config"
	"github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/handlers"
)

func main() {
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
		Addr:         config.Cfg.AggregatorAddress,
		Handler:      router,
		TLSConfig:    cfg,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	glog.Info("Listening on: ", config.Cfg.AggregatorAddress)
	log.Fatal(srv.ListenAndServeTLS("./sslcert/tls.crt", "./sslcert/tls.key"))
}
