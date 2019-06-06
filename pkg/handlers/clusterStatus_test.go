/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEntries(t *testing.T) {
	req, err := http.NewRequest("GET", "/aggregator/clusters/local-,cluster/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(GetClusterStatus)
	handler.ServeHTTP(recorder, req)
	expected := `{"Hash":"","LastUpdated":"","TotalResources":0,"MaxQueueTime":0}`
	assert.Equal(t, expected, strings.Trim(recorder.Body.String(), "\n"), "verify Body")
}
