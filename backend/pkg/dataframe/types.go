// Package dataframe is a small, dependency-free, pandas-style tabular data
// library: a Frame is a named, typed, columnar table with rich metadata
// (provenance, timing, warnings) attached. It is the single data contract
// every workflow node and every connector produces and consumes, so any two
// nodes compose regardless of what produced their input (a SQL table, a
// REST/GraphQL response, or another node's transform).
//
// This package is intentionally standalone: it has zero imports from this
// module's internal/* packages, and nothing in here knows about HTTP,
// SQL, or workflows. Callers convert their domain data into a Frame at the
// boundary (see internal/connections/connectors for examples) and the rest
// of the pipeline only ever sees Frame.
package dataframe

import "fmt"

// Type is the logical type of a column. Values are stored as `any`
// internally (Go has no generic numeric tower that cleanly spans
// int64/float64/decimal), but every column is tagged with the Type its
// values were inferred as or declared to be, and value coercion is checked
// against it.
type Type string

const (
	TypeString  Type = "string"
	TypeInt64   Type = "int64"
	TypeFloat64 Type = "float64"
	TypeBool    Type = "bool"
	TypeTime    Type = "time"
	// TypeJSON holds values that are themselves structured (nested object or
	// array) - e.g. a jsonb column or a nested REST field. Stored as-is
	// (map[string]any / []any) rather than flattened.
	TypeJSON Type = "json"
	// TypeNull is used for a column whose every observed value is nil; the
	// real type is unknown until a non-null value is appended.
	TypeNull Type = "null"
)

func (t Type) valid() bool {
	switch t {
	case TypeString, TypeInt64, TypeFloat64, TypeBool, TypeTime, TypeJSON, TypeNull:
		return true
	default:
		return false
	}
}

// Field describes one column: its name, logical type, and whether it may
// contain nulls.
type Field struct {
	Name     string `json:"name"`
	Type     Type   `json:"type"`
	Nullable bool   `json:"nullable"`
}

// Schema is an ordered list of fields. Order matters - it defines column
// display/iteration order throughout the Frame.
type Schema struct {
	Fields []Field `json:"fields"`
}

// FieldByName returns the field with the given name, if present.
func (s Schema) FieldByName(name string) (Field, bool) {
	for _, f := range s.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// Names returns the column names in schema order.
func (s Schema) Names() []string {
	names := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		names[i] = f.Name
	}
	return names
}

func (s Schema) clone() Schema {
	out := Schema{Fields: make([]Field, len(s.Fields))}
	copy(out.Fields, s.Fields)
	return out
}

// ErrUnknownColumn is returned when an operation references a column that
// does not exist in the frame's schema.
type ErrUnknownColumn struct{ Name string }

func (e ErrUnknownColumn) Error() string { return fmt.Sprintf("dataframe: unknown column %q", e.Name) }
