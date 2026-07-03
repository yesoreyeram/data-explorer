package dataframe

import "fmt"

type JoinType string

const (
	JoinInner JoinType = "inner"
	JoinLeft  JoinType = "left"
)

// JoinOptions configures Join.
type JoinOptions struct {
	LeftKey  string
	RightKey string
	Type     JoinType
	// RightPrefix is prepended to right-side column names that collide with
	// a left-side column name, so a join never silently drops data.
	RightPrefix string
}

// Join combines two frames on equal key values, SQL-JOIN style.
func Join(left, right *Frame, opts JoinOptions) (*Frame, error) {
	if opts.LeftKey == "" || opts.RightKey == "" {
		return nil, fmt.Errorf("dataframe: join requires LeftKey and RightKey")
	}
	if _, ok := left.schema.FieldByName(opts.LeftKey); !ok {
		return nil, ErrUnknownColumn{Name: opts.LeftKey}
	}
	if _, ok := right.schema.FieldByName(opts.RightKey); !ok {
		return nil, ErrUnknownColumn{Name: opts.RightKey}
	}
	if opts.Type == "" {
		opts.Type = JoinInner
	}
	if opts.RightPrefix == "" {
		opts.RightPrefix = "right_"
	}

	rightByKey := make(map[any][]int, right.numRows)
	for i := 0; i < right.numRows; i++ {
		key := right.cols[opts.RightKey][i]
		rightByKey[key] = append(rightByKey[key], i)
	}

	leftCols := make(map[string]bool, len(left.schema.Fields))
	for _, field := range left.schema.Fields {
		leftCols[field.Name] = true
	}

	out := New(nil)
	for li := 0; li < left.numRows; li++ {
		leftRow := left.Row(li)
		matches := rightByKey[left.cols[opts.LeftKey][li]]

		if len(matches) == 0 {
			if opts.Type == JoinLeft {
				out.AppendRow(leftRow)
			}
			continue
		}

		for _, ri := range matches {
			merged := make(map[string]any, len(leftRow)+right.NumCols())
			for k, v := range leftRow {
				merged[k] = v
			}
			rightRow := right.Row(ri)
			for k, v := range rightRow {
				outKey := k
				if leftCols[k] {
					outKey = opts.RightPrefix + k
				}
				merged[outKey] = v
			}
			out.AppendRow(merged)
		}
	}

	out.Meta.SourceType = "dataframe:join"
	out.Meta.Lineage = append(append([]string{}, left.Meta.Name), right.Meta.Name)
	return out, nil
}
