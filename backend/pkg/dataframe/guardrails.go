package dataframe

// This file holds generic, data-integrity guardrails that belong at the
// data-structure level (as opposed to business-policy guardrails like "a
// connection may return at most N rows", which live with the caller that
// knows the policy - see internal/connections.MaxRowLimit). What belongs
// here is protection against a single Frame becoming pathological
// regardless of who built it: an unbounded cell, or more rows than can be
// reasonably held/serialized at once.

// DefaultMaxCellBytes bounds how large a single string cell's value may be
// before TruncateCells clips it. REST/GraphQL responses in particular can
// contain a single huge text/blob field that would otherwise dominate
// memory and JSON payload size disproportionately to the rest of the row.
const DefaultMaxCellBytes = 256 * 1024 // 256KB

// TruncateCells clips any string cell longer than maxBytes (in place) and
// returns how many cells were affected. A maxBytes <= 0 disables the guard.
func (f *Frame) TruncateCells(maxBytes int) int {
	if maxBytes <= 0 {
		return 0
	}
	affected := 0
	for name, col := range f.cols {
		for i, v := range col {
			s, ok := v.(string)
			if !ok || len(s) <= maxBytes {
				continue
			}
			col[i] = s[:maxBytes] + "…(truncated)"
			affected++
		}
		f.cols[name] = col
	}
	if affected > 0 {
		f.Meta = f.Meta.WithWarning("%d cell(s) exceeded %d bytes and were truncated", affected, maxBytes)
	}
	return affected
}

// LimitRows truncates the frame to at most maxRows (keeping the first
// maxRows rows) and marks Meta.Truncated if it had to cut anything. A
// maxRows <= 0 or a frame already within the limit is a no-op.
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
