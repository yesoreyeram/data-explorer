package dataframe

import (
	"encoding/json"
	"time"
)

type Kind = Type

type typedColumn struct {
	kind     Kind
	int64s   []int64
	float64s []float64
	bools    []bool
	strings  []string
	times    []time.Time
	jsons    []any
	values   []any
	nulls    []bool
}

type ColumnarFrame struct {
	Schema  Schema
	Columns map[string]typedColumn
	NumRows int
	Meta    Metadata
}

func (f *Frame) ToColumnar() *ColumnarFrame {
	columns := make(map[string]typedColumn, len(f.schema.Fields))
	for _, field := range f.schema.Fields {
		col := typedColumn{kind: field.Type, nulls: make([]bool, f.numRows)}
		values := f.cols[field.Name]
		for i, raw := range values {
			if raw == nil {
				col.nulls[i] = true
				continue
			}
			switch field.Type {
			case TypeInt64:
				if n, ok := toInt64(raw); ok {
					col.int64s = append(col.int64s, n)
				} else {
					col.values = append(col.values, raw)
				}
			case TypeFloat64:
				if n, ok := toFloat(raw); ok {
					col.float64s = append(col.float64s, n)
				} else {
					col.values = append(col.values, raw)
				}
			case TypeBool:
				if b, ok := raw.(bool); ok {
					col.bools = append(col.bools, b)
				} else {
					col.values = append(col.values, raw)
				}
			case TypeString:
				col.strings = append(col.strings, f.GetString(field.Name, i))
			case TypeTime:
				if t, ok := raw.(time.Time); ok {
					col.times = append(col.times, t)
				} else {
					col.values = append(col.values, raw)
				}
			case TypeJSON:
				col.jsons = append(col.jsons, raw)
			default:
				col.values = append(col.values, raw)
			}
		}
		columns[field.Name] = col
	}
	return &ColumnarFrame{Schema: f.schema.clone(), Columns: columns, NumRows: f.numRows, Meta: f.Meta}
}

func (c *ColumnarFrame) ToFrame() *Frame {
	f := New(c.Schema.Fields)
	for i := 0; i < c.NumRows; i++ {
		row := make(map[string]any, len(c.Schema.Fields))
		for _, field := range c.Schema.Fields {
			col := c.Columns[field.Name]
			if i < len(col.nulls) && col.nulls[i] {
				row[field.Name] = nil
				continue
			}
			row[field.Name] = c.valueAt(field.Name, i)
		}
		f.AppendRow(row)
	}
	f.Meta = c.Meta
	f.Meta.RowCount = f.numRows
	f.Meta.ColumnCount = len(f.schema.Fields)
	return f
}

func (c *ColumnarFrame) valueAt(name string, row int) any {
	col := c.Columns[name]
	seen := 0
	for i := 0; i <= row && i < len(col.nulls); i++ {
		if col.nulls[i] {
			continue
		}
		if i != row {
			seen++
			continue
		}
		switch col.kind {
		case TypeInt64:
			if seen < len(col.int64s) {
				return col.int64s[seen]
			}
		case TypeFloat64:
			if seen < len(col.float64s) {
				return col.float64s[seen]
			}
		case TypeBool:
			if seen < len(col.bools) {
				return col.bools[seen]
			}
		case TypeString:
			if seen < len(col.strings) {
				return col.strings[seen]
			}
		case TypeTime:
			if seen < len(col.times) {
				return col.times[seen]
			}
		case TypeJSON:
			if seen < len(col.jsons) {
				return col.jsons[seen]
			}
		}
		if seen < len(col.values) {
			return col.values[seen]
		}
	}
	return nil
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case json.Number:
		parsed, err := n.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
