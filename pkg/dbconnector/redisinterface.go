/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

/*import (
	"fmt"

	"github.com/golang/glog"
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
	glog.Info("Query: ", q)
	qr := QueryResult{}

	g := rg.GraphNew(GRAPH_NAME, conn)
	result, err := g.Query(q)
	glog.Info("Result: ", result)
	result.PrettyPrint()
	//var qResults map[string]interface{}
	for result.Next() { // Next returns true until the iterator is depleted.
		// Get the current Record.
		r := result.Record()
		for _, key := range r.Keys() {
			println("key: ", key)

		}
		println("values: ", r.Values())

		// Entries in the Record can be accessed by index or key.
		p_name := r.GetByIndex(0)
		fmt.Printf("\nName: %s\n", p_name)
		p_age, _ := r.Get("name")
		fmt.Printf("\nAge: %s\n", p_age)
	}

	r := result.Record()
	glog.Info("Result record: ", r.Values())

	p := r.GetByIndex(0)
	fmt.Printf("%s", p)
	// qr.Results = result.PrettyPrint()
	// qr.Statistics = result.Statistics
	glog.Info("Err: ", err)

	return qr, err
} */
