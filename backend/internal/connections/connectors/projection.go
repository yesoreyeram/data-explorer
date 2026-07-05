package connectors

import (
	"regexp"
	"strings"
)

var safeProjectionColumn = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func applyProjectionHint(sqlText string, columns []string) string {
	if sqlText == "" || len(columns) == 0 {
		return sqlText
	}
	seen := map[string]struct{}{}
	safe := make([]string, 0, len(columns))
	for _, col := range columns {
		if !safeProjectionColumn.MatchString(col) {
			return sqlText
		}
		if _, ok := seen[col]; ok {
			continue
		}
		seen[col] = struct{}{}
		safe = append(safe, col)
	}
	if len(safe) == 0 {
		return sqlText
	}
	return "SELECT " + strings.Join(safe, ", ") + " FROM (" + sqlText + ") AS de_projection"
}

func projectedReadOnlySQL(sqlText string, columns []string) (string, error) {
	projected := applyProjectionHint(sqlText, columns)
	if err := EnsureReadOnlySQL(projected); err != nil {
		return "", err
	}
	return projected, nil
}
