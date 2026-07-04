package dataframe

// Select returns a new Frame containing only the named columns, in the
// order given. Unknown column names are ignored (mirroring a defensive
// "project what exists" semantic useful for optional fields).
func (f *Frame) Select(names ...string) *Frame {
	fields := make([]Field, 0, len(names))
	for _, name := range names {
		if field, ok := f.schema.FieldByName(name); ok {
			fields = append(fields, field)
		}
	}
	out := New(fields)
	for _, field := range fields {
		src := f.cols[field.Name]
		dst := make([]any, len(src))
		copy(dst, src)
		out.cols[field.Name] = dst
	}
	out.numRows = f.numRows
	out.Meta = f.Meta
	out.Meta.ColumnCount = len(fields)
	return out
}

// Rename returns a new Frame with columns renamed per the given map
// (old name -> new name); columns not mentioned keep their name.
func (f *Frame) Rename(names map[string]string) *Frame {
	fields := make([]Field, len(f.schema.Fields))
	cols := make(map[string][]any, len(f.schema.Fields))
	for i, field := range f.schema.Fields {
		newName := field.Name
		if renamed, ok := names[field.Name]; ok {
			newName = renamed
		}
		fields[i] = Field{Name: newName, Type: field.Type, Nullable: field.Nullable}
		cols[newName] = f.cols[field.Name]
	}
	out := &Frame{schema: Schema{Fields: fields}, cols: cols, numRows: f.numRows, Meta: f.Meta}
	return out
}

// Filter returns a new Frame containing only the rows for which predicate
// returns true. Predicate receives the row-index and the reconstructed row.
func (f *Frame) Filter(predicate func(i int, row map[string]any) bool) *Frame {
	out := New(f.schema.clone().Fields)
	for i := 0; i < f.numRows; i++ {
		row := f.Row(i)
		if predicate(i, row) {
			out.AppendRow(row)
		}
	}
	out.Meta = f.Meta
	out.Meta.RowCount = out.numRows
	return out
}

// Concat appends the rows of other onto a copy of f. Schemas are unioned
// (as in AppendRow) so frames with slightly different shapes can still be
// combined - e.g. paginated REST results where later pages introduce an
// optional field.
func Concat(name string, frames ...*Frame) *Frame {
	out := New(nil)
	total := 0
	for _, fr := range frames {
		if fr == nil {
			continue
		}
		total += fr.numRows
		for i := 0; i < fr.numRows; i++ {
			out.AppendRow(fr.Row(i))
		}
	}
	out.Meta.Name = name
	out.Meta.RowCount = total
	return out
}
