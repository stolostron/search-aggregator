/*
 * Copyright (c) 2020 Red Hat, Inc.
 * Copyright Contributors to the Open Cluster Management project
 */
package handlers

import (
	"testing"
)

func Test_getEdgeUID(t *testing.T) {

	result := getEdgeUID("source", "type", "dest")

	if result != "source-type->dest" {
		t.Errorf("Failed building edge UID. Expected: source-type->dest but got: %s", result)
	}
}
