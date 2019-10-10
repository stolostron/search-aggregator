/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	rg "github.com/redislabs/redisgraph-go"
)

var Store DBStore

// Interface for the DB dependency. Used for mocking rg.
type DBStore interface {
	Query(q string) (QueryResult, error)
}

//Real redis graph V1 version - structure for RedisGraph - Pool decides which version to use
type RedisGraphStoreV1 struct{} // No properties. It just has the method

type QueryResult struct {
	Results    [][]string
	Statistics []string
}

// Executes the given query against redisgraph.
// Called by the other functions
func (RedisGraphStoreV1) Query(q string) (QueryResult, error) {
	// Get connection from the pool
	conn := Pool.Get() // This will block if there aren't any valid connections that are available.
	defer conn.Close()
	qr := QueryResult{}
	g := rg.Graph{
		Conn: conn,
		Name: GRAPH_NAME,
	}
	result, err := g.Query(q)
	qr.Results = result.Results
	qr.Statistics = result.Statistics
	return qr, err
}
