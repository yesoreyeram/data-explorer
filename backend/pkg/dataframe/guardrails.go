package dataframe

const (
	DefaultMaxCellBytes       = 256 * 1024
	DefaultMaxColumns         = 512
	DefaultMaxStringCellBytes = 1 * 1024 * 1024
	DefaultMaxBytesCellBytes  = 5 * 1024 * 1024
)

func (f *Frame) TruncateCells(maxBytes int) int {
	return f.TruncateCellsByType(maxBytes, maxBytes)
}

func (f *Frame) TruncateCellsByType(maxStringBytes, maxBytesBytes int) int {
	affected := 0
	for name, col := range f.cols {
		for i, v := range col {
			switch value := v.(type) {
			case string:
				if maxStringBytes > 0 && len(value) > maxStringBytes {
					col[i] = value[:maxStringBytes] + "…(truncated)"
					affected++
				}
			case []byte:
				if maxBytesBytes > 0 && len(value) > maxBytesBytes {
					clipped := append([]byte(nil), value[:maxBytesBytes]...)
					col[i] = clipped
					affected++
				}
			}
		}
		f.cols[name] = col
	}
	if affected > 0 {
		f.Meta = f.Meta.WithWarning("%d cell(s) exceeded per-type cell limits and were truncated", affected)
	}
	return affected
}

func (f *Frame) LimitRows(maxRows int) *Frame {
	if maxRows <= 0 || f.numRows <= maxRows {
		return f
	}
	for name, col := range f.cols {
		f.cols[name] = col[:maxRows]
	}
	f.numRows = maxRows
	f.Meta.Truncated = true
	f.Meta.RowCount = maxRows
	return f
}

func (f *Frame) LimitColumns(maxCols int) *Frame {
	if maxCols <= 0 || len(f.schema.Fields) <= maxCols {
		return f
	}
	kept := make(map[string]struct{}, maxCols)
	for _, field := range f.schema.Fields[:maxCols] {
		kept[field.Name] = struct{}{}
	}
	for name := range f.cols {
		if _, ok := kept[name]; !ok {
			delete(f.cols, name)
			delete(f.dictCols, name)
		}
	}
	f.schema.Fields = append([]Field(nil), f.schema.Fields[:maxCols]...)
	f.Meta.Truncated = true
	f.Meta.ColumnCount = len(f.schema.Fields)
	f.Meta = f.Meta.WithWarning("Column cap reached at %d columns.", maxCols)
	return f
}
