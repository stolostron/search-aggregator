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
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	searchv1alpha1 "github.com/open-cluster-management/search-operator/api/v1alpha1"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
	db "github.com/open-cluster-management/search-aggregator/pkg/dbconnector"
)

// LivenessProbe is used to check if this service is alive.
func LivenessProbe(w http.ResponseWriter, r *http.Request) {
	glog.V(2).Info("livenessProbe")
	fmt.Fprint(w, "OK")
}

// ReadinessProbe checks if Redis is available.
func ReadinessProbe(w http.ResponseWriter, r *http.Request) {

	glog.V(2).Info("readinessProbe - Checking Redis connection.")

	if isRedisDeployed() {
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
	}
	// Respond with success
	fmt.Fprint(w, "OK")
}

func isRedisDeployed() bool {
	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		ns = "open-cluster-management"
	}
	client := config.GetKubeClient()
	operatorPath := "/apis/search.open-cluster-management.io/v1alpha1/namespaces/" + ns + "/searchoperators/searchoperator"
	srchoBytes, err := client.RESTClient().Get().
		AbsPath(operatorPath).DoRaw()
	instance := &searchv1alpha1.SearchOperator{}
	err = json.Unmarshal(srchoBytes, instance)

	if err != nil {
		glog.Infof("Error unmarshaling searchoperator %v: ", err)
	}
	glog.Info("Is Redisgraph deployed? ", *instance.Status.DeployRedisgraph)
	return *instance.Status.DeployRedisgraph
}
