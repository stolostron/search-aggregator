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

	"github.com/golang/glog"
)

// LivenessProbe is used to check if this service is alive.
func LivenessProbe(w http.ResponseWriter, r *http.Request) {
	glog.V(2).Info("livenessProbe")
	fmt.Fprint(w, "OK")
}

// ReadinessProbe checks if Redis is available.
func ReadinessProbe(w http.ResponseWriter, r *http.Request) {
	glog.V(2).Info("readinessProbe - Checking Redis connection.")

	// Go straight to the pool's Dial because we don't actually want to play by the pool's
	// rules here - just want a connection unrelated to all the other ones,
	// conn, err := db.Pool.Dial()
	// if err != nil {
	// 	// Respond with error.
	// 	glog.Warning("Unable to reach Redis.")
	// 	http.Error(w, "Unable to reach Redis.", 503)
	// 	return
	// }

	// defer conn.Close()
	// Respond with success
	fmt.Fprint(w, "OK")
}
