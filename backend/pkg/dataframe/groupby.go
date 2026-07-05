package dataframe

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type AggOp string

const (
	AggSum   AggOp = "sum"
	AggAvg   AggOp = "avg"
	AggCount AggOp = "count"
	AggMin   AggOp = "min"
	AggMax   AggOp = "max"
)

// Agg describes one aggregation to compute per group: Op applied to Field,
// exposed in the output under column name As (defaults to "<op>_<field>").
type Agg struct {
	Field string
	Op    AggOp
	As    string
}

func (a Agg) outName() string {
	if a.As != "" {
		return a.As
	}
	return string(a.Op) + "_" + a.Field
}

// GroupBy groups rows by the given key columns and computes the given
// aggregations per group, SQL GROUP BY style. Groups are emitted in
// ascending key order for deterministic output.
func (f *Frame) GroupBy(keys []string, aggs []Agg) (*Frame, error) {
	for _, k := range keys {
		if _, ok := f.schema.FieldByName(k); !ok {
			return nil, ErrUnknownColumn{Name: k}
		}
	}

	type group struct {
		keyValues map[string]any
		values    map[string][]float64
		count     int
	}

	groups := make(map[string]*group)
	var order []string

	for i := 0; i < f.numRows; i++ {
		parts := make([]string, len(keys))
		keyValues := make(map[string]any, len(keys))
		for k, key := range keys {
			v := f.cols[key][i]
			parts[k] = toKeyString(v)
			keyValues[key] = v
		}
		groupKey := strings.Join(parts, "\x1f")

		g, ok := groups[groupKey]
		if !ok {
			g = &group{keyValues: keyValues, values: map[string][]float64{}}
			groups[groupKey] = g
			order = append(order, groupKey)
		}
		g.count++
		for _, agg := range aggs {
			if agg.Op == AggCount {
				continue
			}
			if v, ok := toFloat(f.cols[agg.Field][i]); ok {
				name := agg.outName()
				g.values[name] = append(g.values[name], v)
			}
		}
	}

	sort.Strings(order)

	out := New(nil)
	for _, key := range order {
		g := groups[key]
		row := make(map[string]any, len(keys)+len(aggs))
		for k, v := range g.keyValues {
			row[k] = v
		}
		for _, agg := range aggs {
			name := agg.outName()
			if agg.Op == AggCount {
				row[name] = int64(g.count)
				continue
			}
			row[name] = reduceFloat(agg.Op, g.values[name])
		}
		out.AppendRow(row)
	}

	out.Meta.SourceType = "dataframe:groupby"
	out.Meta.Lineage = []string{f.Meta.Name}
	return out, nil
}

func reduceFloat(op AggOp, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	switch op {
	case AggSum, AggAvg:
		var sum float64
		for _, v := range values {
			sum += v
		}
		if op == AggAvg {
			return sum / float64(len(values))
		}
		return sum
	case AggMin:
		m := values[0]
		for _, v := range values {
			if v < m {
				m = v
			}
		}
		return m
	case AggMax:
		m := values[0]
		for _, v := range values {
			if v > m {
				m = v
			}
		}
		return m
	default:
		return 0
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func toKeyString(v any) string {
	if v == nil {
		return "\x00null"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
