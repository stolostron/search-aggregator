/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.

Copyright (c) 2020 Red Hat, Inc.
*/
// Copyright Contributors to the Open Cluster Management project

package handlers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LivenessProbe is used to check if this service is alive.
func LivenessProbe(w http.ResponseWriter, r *http.Request) {
	glog.V(2).Info("livenessProbe")
	fmt.Fprint(w, "OK")
}

// ReadinessProbe checks if Redis is available.
func ReadinessProbe(w http.ResponseWriter, r *http.Request) {

	glog.V(2).Info("readinessProbe - Checking Redis connection.")
	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		ns = "open-cluster-management"
	}
	options := metav1.GetOptions{}
	_, err := config.GetKubeClient().CoreV1().Pods(ns).Get("search-redisgraph-0", options)
	if err != nil && errors.IsNotFound(err) {
		glog.V(2).Info("Redisgraph pod is not present - will re-check once it is enabled.")
		return
	}
	// Go straight to the pool's Dial because we don't actually want to play by the pool's
	// rules here - just want a connection unrelated to all the other ones,
	conn, err := db.Pool.Dial()
	if err != nil {
		// Respond with error.
		glog.Warning("Unable to reach Redis.")
		http.Error(w, "Unable to reach Redis.", 503)
		return
	}

	defer conn.Close()
	// Respond with success
	fmt.Fprint(w, "OK")
}
