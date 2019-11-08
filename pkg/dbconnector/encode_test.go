/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/
package dbconnector

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestValidateClusterName(t *testing.T) {
	error1 := ValidateClusterName("test")
	assert.Equal(t, error1, nil, "test")
	error2 := ValidateClusterName("te'st")
	assert.Error(t, error2)
	error3 := ValidateClusterName("te/st")
	assert.Error(t, error3)
	error4 := ValidateClusterName("te.st")
	assert.Error(t, error4)
	error5 := ValidateClusterName("=test")
	assert.Error(t, error5)

}
