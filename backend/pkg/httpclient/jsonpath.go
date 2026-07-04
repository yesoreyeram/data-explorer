package httpclient

import "strconv"

// JSONPath is the exported form of getPath, for callers outside this
// package (e.g. a connector) that need to pull a value - such as a list of
// result items - out of an arbitrary decoded JSON response using the same
// dotted-path convention used for pagination cursors.
func JSONPath(data any, path string) (any, bool) {
	return getPath(data, path)
}

// getPath walks a decoded JSON value (map[string]any / []any / scalars)
// following a dot-separated path such as "data.repository.issues" or
// "items.0.id" (numeric segments index into arrays). It's intentionally a
// minimal subset of JSONPath - just enough to locate a pagination cursor or
// item count inside an arbitrary API response shape - not a general query
// language.
func getPath(data any, path string) (any, bool) {
	if path == "" {
		return data, true
	}
	cur := data
	for _, segment := range splitPath(path) {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[segment]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(segment)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

func splitPath(path string) []string {
	var segments []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			segments = append(segments, path[start:i])
			start = i + 1
		}
	}
	segments = append(segments, path[start:])
	return segments
}

// asString coerces a decoded JSON scalar to a string, for use as a cursor
// or query parameter value.
func asString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, val != ""
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(val), true
	default:
		return "", false
	}
}

// asBool coerces a decoded JSON scalar to a bool.
func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// asArrayLen returns the length of v if it is a JSON array, else 0.
func asArrayLen(v any) int {
	if arr, ok := v.([]any); ok {
		return len(arr)
	}
	return 0
}
