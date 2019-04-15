/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package handlers

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"
	db "github.ibm.com/IBMPrivateCloud/search-aggregator/pkg/dbconnector"
)

// LivenessProbe is used to check if this service is alive.
func LivenessProbe(w http.ResponseWriter, r *http.Request) {
	glog.Info("livenessProbe")
	fmt.Fprint(w, "OK")
}

// ReadinessProbe checks if Redis is available.
func ReadinessProbe(w http.ResponseWriter, r *http.Request) {
	glog.Info("readinessProbe - Checking Redis connection.")

	conn, connError := db.Pool.Dial() // Go straight to the pool's Dial because we don't actually want to play by the pool's rules here - just want a connection unrelated to all the other ones

	if connError != nil {
		// Respond with error.
		glog.Warning("Unable to reach Redis.")
		http.Error(w, "Unable to reach Redis.", 503)
	} else {
		// Respond with success.
		fmt.Fprint(w, "OK")
	}
	conn.Close()
}
