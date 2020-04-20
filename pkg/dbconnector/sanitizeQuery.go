// Copyright (c) 2020 Red Hat, Inc.

package dbconnector

import (
	"fmt"
	"strings"
)

// Escape or remove any characters that could break the query.
func sanitizeValue(value string) string {
	res1 := strings.Replace(value, "\"", "\\\"", -1) // Escape all double quotes. A double quote would finalize the openCypher query.
	res2 := strings.Replace(res1, "'", "\\'", -1)    // Escape all single quotes. A single quote would finalize the openCypher query.
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
		case int:
			sanitizedValues[i] = typedVal
		}
	}
	return fmt.Sprintf(queryTemplate, sanitizedValues...)
}