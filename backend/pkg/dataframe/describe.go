package dataframe

// ColumnSummary is the per-column output of Describe, mirroring the shape
// of pandas' DataFrame.describe(): counts plus, where applicable, numeric
// min/max/mean or string length bounds.
type ColumnSummary struct {
	Name      string   `json:"name"`
	Type      Type     `json:"type"`
	Count     int      `json:"count"` // non-null values
	NullCount int      `json:"nullCount"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Mean      *float64 `json:"mean,omitempty"`
	MinLen    *int     `json:"minLen,omitempty"` // for string columns
	MaxLen    *int     `json:"maxLen,omitempty"`
}

// Describe computes summary statistics for every column, in schema order.
func (f *Frame) Describe() []ColumnSummary {
	summaries := make([]ColumnSummary, 0, len(f.schema.Fields))
	for _, field := range f.schema.Fields {
		col := f.cols[field.Name]
		s := ColumnSummary{Name: field.Name, Type: field.Type}

		var sum float64
		var numCount int
		var minF, maxF float64
		var minLen, maxLen int
		haveNum, haveLen := false, false

		for _, v := range col {
			if v == nil {
				s.NullCount++
				continue
			}
			s.Count++
			if fv, ok := toFloat(v); ok {
				sum += fv
				numCount++
				if !haveNum || fv < minF {
					minF = fv
				}
				if !haveNum || fv > maxF {
					maxF = fv
				}
				haveNum = true
			}
			if sv, ok := v.(string); ok {
				l := len(sv)
				if !haveLen || l < minLen {
					minLen = l
				}
				if !haveLen || l > maxLen {
					maxLen = l
				}
				haveLen = true
			}
		}

		if haveNum {
			mean := sum / float64(numCount)
			s.Min, s.Max, s.Mean = ptr(minF), ptr(maxF), ptr(mean)
		}
		if haveLen {
			s.MinLen, s.MaxLen = ptrInt(minLen), ptrInt(maxLen)
		}

		summaries = append(summaries, s)
	}
	return summaries
}

func ptr(v float64) *float64 { return &v }
func ptrInt(v int) *int      { return &v }
