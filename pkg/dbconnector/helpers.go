/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package dbconnector

import (
	"strings"

	rg2 "github.com/redislabs/redisgraph-go"
)

// Merges maps, putting values of b over top of values from a. In practice this doesn't matter because
// the error maps are keyed by UID and don't share any keys.
// If both are nil, returns nil.
func mergeErrorMaps(a, b map[string]error) map[string]error {
	if a == nil { // Notice this returns nil if both are nil
		return b
	}
	if b == nil {
		return a
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

// Returns the minimum of 2 ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Tells whether the error in question is representative of the redis connection dying.
// It gives EOF when it's cut off mid usage, otherwise does connection refused.
func IsBadConnection(e error) bool {
	if e == nil {
		return false
	}
	return strings.HasSuffix(e.Error(), "connection refused") || strings.HasSuffix(e.Error(), "EOF")
}

// Test for specific redis graph update error
func IsGraphMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "key doesn't contains a graph object")
}

func IsPropertySet(res *rg2.QueryResult) bool {
	return res.PropertiesSet() > 0
}
