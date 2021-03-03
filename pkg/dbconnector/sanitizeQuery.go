// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package dbconnector

import (
	"fmt"
	"strings"
)

// Escape any characters that could break the openCypher query.
func sanitizeValue(value string) string {
	res1 := strings.Replace(value, "\"", "\\\"", -1) // Escape all double quotes.
	res2 := strings.Replace(res1, "'", "\\'", -1)    // Escape all single quotes.
	return res2
}

// Sanitizes openCypher query.
// Similar to SQL injection, an attacker could inject malicious code into the openCypher query.
func SanitizeQuery(queryTemplate string, values ...interface{}) string {
	sanitizedValues := make([]interface{}, len(values))
	for i, value := range values {
		switch typedVal := value.(type) {
		case string:
			sanitizedValues[i] = sanitizeValue(typedVal)
		default:
			sanitizedValues[i] = typedVal
		}
	}
	return fmt.Sprintf(queryTemplate, sanitizedValues...)
}