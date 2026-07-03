package connectors

import "github.com/yesoreyeram/data-explorer/backend/pkg/httpclient"

// extractItems normalizes an arbitrary decoded JSON payload (object, array,
// or scalar) found at path into a slice of "row" values, so REST/GraphQL
// responses of any shape can be turned into dataframe rows the same way:
// an array is used as-is, a single object becomes a one-item slice, and a
// bare scalar is wrapped so it still produces a (single-column) row rather
// than being silently dropped.
func extractItems(decoded any, path string) []any {
	node, ok := httpclient.JSONPath(decoded, path)
	if !ok {
		return nil
	}
	switch v := node.(type) {
	case []any:
		return v
	case nil:
		return nil
	default:
		return []any{v}
	}
}

// toRowMap coerces a single extracted item into a dataframe row. Non-object
// items (e.g. a plain string/number in a JSON array) are wrapped under a
// "value" column so they still round-trip through the tabular pipeline.
func toRowMap(item any) map[string]any {
	if m, ok := item.(map[string]any); ok {
		return m
	}
	return map[string]any{"value": item}
}
