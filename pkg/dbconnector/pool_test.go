// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
package dbconnector

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

// NOTE: These tests assume that RedisGraph is not running locally.
// We need a way to mock RedisGraph.

func Test_getRedisConnection(t *testing.T) {
	conn, err := getRedisConnection()

	assert.Nil(t, conn, "Redis Connection")
	assert.NotNil(t, err, "Redis Conn Error")
}
