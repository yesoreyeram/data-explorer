package dataframe

import "time"

type dictionaryEncodedColumn struct {
	dictionary []string
	indexes    []int
}

// Frame is a named, typed, columnar table: parallel column slices, all the
// same length, described by a Schema, plus Metadata about how the data was
// produced. It is the single value type passed between workflow nodes.
type Frame struct {
	schema   Schema
	cols     map[string][]any
	dictCols map[string]dictionaryEncodedColumn
	numRows  int
	Meta     Metadata
}

func New(fields []Field) *Frame {
	f := &Frame{
		schema:   Schema{Fields: append([]Field(nil), fields...)},
		cols:     make(map[string][]any, len(fields)),
		dictCols: make(map[string]dictionaryEncodedColumn),
	}
	for _, field := range fields {
		f.cols[field.Name] = []any{}
	}
	f.Meta.GeneratedAt = time.Now()
	return f
}

func FromRecords(rows []map[string]any) *Frame {
	f := New(nil)
	for _, row := range rows {
		f.AppendRow(row)
	}
	return f
}

func (f *Frame) Schema() Schema        { return f.schema.clone() }
func (f *Frame) NumRows() int          { return f.numRows }
func (f *Frame) NumCols() int          { return len(f.schema.Fields) }
func (f *Frame) ColumnNames() []string { return f.schema.Names() }

func (f *Frame) Column(name string) ([]any, bool) {
	col, ok := f.cols[name]
	return col, ok
}

func (f *Frame) Row(i int) map[string]any {
	row := make(map[string]any, len(f.schema.Fields))
	for _, field := range f.schema.Fields {
		row[field.Name] = f.cols[field.Name][i]
	}
	return row
}

func (f *Frame) Rows() []map[string]any {
	rows := make([]map[string]any, f.numRows)
	for i := 0; i < f.numRows; i++ {
		rows[i] = f.Row(i)
	}
	return rows
}

func (f *Frame) ensureColumn(name string, t Type) {
	if _, ok := f.cols[name]; ok {
		return
	}
	col := make([]any, f.numRows)
	f.cols[name] = col
	f.schema.Fields = append(f.schema.Fields, Field{Name: name, Type: t, Nullable: f.numRows > 0})
}

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

func (f *Frame) SetMeta(meta Metadata) *Frame {
	meta.RowCount = f.numRows
	meta.ColumnCount = len(f.schema.Fields)
	f.Meta = meta
	return f
}

func (f *Frame) ApplyDictionaryEncoding(threshold int) int {
	if threshold <= 0 {
		return 0
	}
	encoded := 0
	for _, field := range f.schema.Fields {
		if field.Type != TypeString {
			continue
		}
		col := f.cols[field.Name]
		if len(col) == 0 {
			continue
		}
		dictIndex := map[string]int{}
		dict := make([]string, 0, threshold)
		indexes := make([]int, len(col))
		ok := true
		for i, raw := range col {
			if raw == nil {
				indexes[i] = -1
				continue
			}
			s, isString := raw.(string)
			if !isString {
				ok = false
				break
			}
			idx, exists := dictIndex[s]
			if !exists {
				if len(dict) >= threshold {
					ok = false
					break
				}
				idx = len(dict)
				dictIndex[s] = idx
				dict = append(dict, s)
			}
			indexes[i] = idx
		}
		if !ok || len(dict) == 0 {
			continue
		}
		f.dictCols[field.Name] = dictionaryEncodedColumn{dictionary: dict, indexes: indexes}
		encoded++
	}
	return encoded
}

func (f *Frame) GetString(col string, row int) string {
	if dict, ok := f.dictCols[col]; ok && row >= 0 && row < len(dict.indexes) {
		idx := dict.indexes[row]
		if idx >= 0 && idx < len(dict.dictionary) {
			return dict.dictionary[idx]
		}
		return ""
	}
	values, ok := f.cols[col]
	if !ok || row < 0 || row >= len(values) || values[row] == nil {
		return ""
	}
	s, _ := values[row].(string)
	return s
}
