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
	rg2 "github.ibm.com/xrdharma/rg-2.0"
)

type RedisGraphStoreV2 struct{}

// Executes the given query against redisgraph.
// Called by the other functions in this file
// Not fully implemented
func (RedisGraphStoreV2) Query(q string) (QueryResult, error) {
	// Get connection from the pool
	conn := Pool.Get() // This will block if there aren't any valid connections that are available.
	defer conn.Close()
	glog.V(4).Infof("Query %s", q)
	qr := QueryResult{}
	var err error

	g := rg2.Graph{
		Conn: conn,
		Id:   GRAPH_NAME,
	}
	result, err2 := g.Query(q)
	glog.V(4).Infof("Result Len %d", len(result.Results))
	glog.V(4).Infof("Head Len %d", len(result.Header.Column_names))
	glog.V(4).Infof("Stat Len %d", len(result.Statistics))
	for k, v := range result.Statistics {
		glog.V(4).Infof("statK %s => statV %f", k, v)
	}
	for i := range result.Results {
		for j := range result.Results[i] {
			glog.V(4).Infof("arr  val %s", result.Results[i][j])
		}
	}
	headerPlusRecords := 0
	if len(result.Statistics) > 0 {
		headerPlusRecords = len(result.Results) + 1
		qr.Results = make([][]string, headerPlusRecords)
		qr.Results[0] = result.Header.Column_names
		for i := 0; i < len(result.Results); i++ {
			for j := 0; j < len(result.Results[i]); j++ {
				qr.Results[i+1][j] = result.Results[i][j].(string)
			}

		}
		qr.Statistics = make([]string, len(result.Statistics))
		for k, v := range result.Statistics {
			i := 0
			qr.Statistics[i] = k + ": " + fmt.Sprintf("%.6f", v)
			i++
		}
		glog.V(4).Infof("QR2Result Len %d", len(qr.Results))
		glog.V(4).Infof("QR2Stat Len %d", len(qr.Statistics))
		err = err2
	}
	return qr, err

} */
