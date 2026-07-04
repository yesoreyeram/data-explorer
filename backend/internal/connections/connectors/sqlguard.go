// sqlguard applies a defense-in-depth check to hand-written SQL coming from
// the API: the data explorer is a read-only exploration tool, so any
// statement that is not a single SELECT/WITH query is rejected before it
// ever reaches the driver. This is not a substitute for least-privilege
// database credentials (the primary control - connections should use a
// read-only DB role) but it stops accidental or malicious mutation attempts
// at the application layer too.
package connectors

import (
	"fmt"
	"regexp"
	"strings"
)

var disallowedKeyword = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|grant|revoke|create|replace|merge|call|exec|execute|vacuum|copy)\b`)

func EnsureReadOnlySQL(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return fmt.Errorf("sql must not be empty")
	}

	// Reject stacked statements (a semicolon anywhere but a single trailing one).
	body := strings.TrimSuffix(trimmed, ";")
	if strings.Contains(body, ";") {
		return fmt.Errorf("multiple statements are not allowed")
	}

	lower := strings.ToLower(body)
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	if disallowedKeyword.MatchString(body) {
		return fmt.Errorf("query contains a disallowed keyword")
	}

	return nil
}
