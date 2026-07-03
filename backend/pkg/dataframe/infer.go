package dataframe

import (
	"encoding/json"
	"time"
)

// InferType maps a Go runtime value (as produced by a JSON decoder, a SQL
// driver, or hand-built row data) to a dataframe Type.
func InferType(v any) Type {
	switch val := v.(type) {
	case nil:
		return TypeNull
	case string:
		return TypeString
	case bool:
		return TypeBool
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return TypeInt64
	case float32, float64:
		return TypeFloat64
	case time.Time:
		return TypeTime
	case json.Number:
		if _, err := val.Int64(); err == nil {
			return TypeInt64
		}
		return TypeFloat64
	case map[string]any, []any:
		return TypeJSON
	default:
		// Unknown concrete type (e.g. a driver-specific struct); treat as an
		// opaque JSON value rather than guessing a scalar type for it.
		return TypeJSON
	}
}

// unifyType widens two column types to a common type as new values are
// observed, mirroring how pandas widens a column's dtype as it's built:
// int64 + float64 -> float64, anything + null -> anything, anything
// genuinely incompatible -> json (kept raw, never silently stringified).
func unifyType(a, b Type) Type {
	if a == b {
		return a
	}
	if a == TypeNull {
		return b
	}
	if b == TypeNull {
		return a
	}
	if (a == TypeInt64 && b == TypeFloat64) || (a == TypeFloat64 && b == TypeInt64) {
		return TypeFloat64
	}
	return TypeJSON
}
