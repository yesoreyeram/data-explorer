package dataframe

import "time"

// Frame is a named, typed, columnar table: parallel column slices, all the
// same length, described by a Schema, plus Metadata about how the data was
// produced. It is the single value type passed between workflow nodes.
type Frame struct {
	schema  Schema
	cols    map[string][]any
	numRows int
	Meta    Metadata
}

// New creates an empty Frame with the given column definitions.
func New(fields []Field) *Frame {
	f := &Frame{
		schema: Schema{Fields: append([]Field(nil), fields...)},
		cols:   make(map[string][]any, len(fields)),
	}
	for _, field := range fields {
		f.cols[field.Name] = []any{}
	}
	f.Meta.GeneratedAt = time.Now()
	return f
}

// FromRecords builds a Frame from row-oriented data (e.g. decoded JSON or a
// SQL driver's row maps), inferring column order (first-seen across rows),
// types, and nullability as it goes.
func FromRecords(rows []map[string]any) *Frame {
	f := New(nil)
	for _, row := range rows {
		f.AppendRow(row)
	}
	return f
}

// Schema returns a copy of the frame's current schema.
func (f *Frame) Schema() Schema { return f.schema.clone() }

// NumRows returns the number of rows in the frame.
func (f *Frame) NumRows() int { return f.numRows }

// NumCols returns the number of columns in the frame.
func (f *Frame) NumCols() int { return len(f.schema.Fields) }

// ColumnNames returns column names in schema order.
func (f *Frame) ColumnNames() []string { return f.schema.Names() }

// Column returns the raw values of a column, in row order, or false if the
// column doesn't exist. The returned slice must not be mutated.
func (f *Frame) Column(name string) ([]any, bool) {
	col, ok := f.cols[name]
	return col, ok
}

// Row reconstructs row i as a name -> value map.
func (f *Frame) Row(i int) map[string]any {
	row := make(map[string]any, len(f.schema.Fields))
	for _, field := range f.schema.Fields {
		row[field.Name] = f.cols[field.Name][i]
	}
	return row
}

// Rows materializes every row as name -> value maps, in row order. This is
// the boundary representation used by the JSON API and by node executors
// that need pandas-style "apply a function per row" semantics (e.g. the
// JSONata transform/filter nodes).
func (f *Frame) Rows() []map[string]any {
	rows := make([]map[string]any, f.numRows)
	for i := 0; i < f.numRows; i++ {
		rows[i] = f.Row(i)
	}
	return rows
}

// ensureColumn adds a column (backfilling prior rows with nil) if it
// doesn't already exist.
func (f *Frame) ensureColumn(name string, t Type) {
	if _, ok := f.cols[name]; ok {
		return
	}
	col := make([]any, f.numRows)
	f.cols[name] = col
	f.schema.Fields = append(f.schema.Fields, Field{Name: name, Type: t, Nullable: f.numRows > 0})
}

// AppendRow adds a row to the frame. Columns not present in the frame's
// schema are added on the fly (existing rows backfilled with nil); columns
// in the schema but absent from this row are treated as null. Column types
// widen automatically as new values are observed (see unifyType).
func (f *Frame) AppendRow(row map[string]any) {
	for name, value := range row {
		f.ensureColumn(name, InferType(value))
	}

	for i, field := range f.schema.Fields {
		value, present := row[field.Name]
		if !present || value == nil {
			f.cols[field.Name] = append(f.cols[field.Name], nil)
			f.schema.Fields[i].Nullable = true
			continue
		}
		f.cols[field.Name] = append(f.cols[field.Name], value)
		newType := unifyType(field.Type, InferType(value))
		if newType != field.Type {
			f.schema.Fields[i].Type = newType
		}
	}
	f.numRows++
	f.Meta.RowCount = f.numRows
	f.Meta.ColumnCount = len(f.schema.Fields)
}

// SetMeta replaces the frame's metadata (row/column counts are recomputed
// from the frame's actual shape so callers can't accidentally desync them).
func (f *Frame) SetMeta(meta Metadata) *Frame {
	meta.RowCount = f.numRows
	meta.ColumnCount = len(f.schema.Fields)
	f.Meta = meta
	return f
}
