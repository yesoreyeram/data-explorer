package dataframe

import "encoding/json"

// wireFrame is the JSON envelope every API response carrying tabular data
// uses: schema (typed columns), rows (the data, still row-oriented for easy
// consumption by any JSON client, including the frontend), and meta
// (provenance/quality). Kept row-oriented rather than columnar on the wire
// because it is dramatically simpler for JavaScript consumers to render
// directly into a table with no transposition step.
type wireFrame struct {
	Schema Schema           `json:"schema"`
	Rows   []map[string]any `json:"rows"`
	Meta   Metadata         `json:"meta"`
}

func (f *Frame) MarshalJSON() ([]byte, error) {
	return json.Marshal(wireFrame{
		Schema: f.schema,
		Rows:   f.Rows(),
		Meta:   f.Meta,
	})
}

func (f *Frame) UnmarshalJSON(data []byte) error {
	var w wireFrame
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*f = *New(w.Schema.Fields)
	for _, row := range w.Rows {
		f.AppendRow(row)
	}
	f.Meta = w.Meta
	f.Meta.RowCount = f.numRows
	f.Meta.ColumnCount = len(f.schema.Fields)
	return nil
}
