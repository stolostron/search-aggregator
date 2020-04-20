/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

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

// Sanitizes openCypher query. Similar to SQL injection, an attacker could inject malicious code into the openCypher query.
func SanitizeQuery(queryTemplate string, value string) string {
	return fmt.Sprintf(queryTemplate, sanitizeValue(value))
}