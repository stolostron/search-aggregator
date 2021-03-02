/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets,
irrespective of what has been deposited with the U.S. Copyright Office.
Copyright (c) 2020 Red Hat, Inc.
*/
// Copyright Contributors to the Open Cluster Management project

package dbconnector

import (
	"github.com/golang/glog"
	rg2 "github.com/redislabs/redisgraph-go"
)

var Store DBStore

// Interface for the DB dependency. Used for mocking rg.
type DBStore interface {
	Query(q string) (*rg2.QueryResult, error)
}

type QueryResult struct {
	Results    [][]string
	Statistics []string
}

//type QueryResult rg2.QueryResult

type RedisGraphStoreV2 struct{}

// Executes the given query against redisgraph.
// Called by the other functions in this file
// Not fully implemented
func (RedisGraphStoreV2) Query(q string) (*rg2.QueryResult, error) {
	// Get connection from the pool
	conn := Pool.Get() // This will block if there aren't any valid connections that are available.
	defer conn.Close()
	g := rg2.Graph{
		Conn: conn,
		Id:   GRAPH_NAME,
	}
	result, err := g.Query(q)
	if err != nil {
		glog.Error("Error fetching results from RedisGraph V2 : ", err)
		glog.V(4).Info("Failed query: ", q)
	}
	return result, err

}
