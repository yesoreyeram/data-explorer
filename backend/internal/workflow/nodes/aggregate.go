package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
)

// AggregateNode groups rows by a set of key columns and computes summary
// statistics per group, similar to SQL GROUP BY.
type AggregateNode struct{}

type AggregateOp string

const (
	AggSum   AggregateOp = "sum"
	AggAvg   AggregateOp = "avg"
	AggCount AggregateOp = "count"
	AggMin   AggregateOp = "min"
	AggMax   AggregateOp = "max"
)

type Aggregation struct {
	Field string      `json:"field"`
	Op    AggregateOp `json:"op"`
	As    string      `json:"as"`
}

type AggregateConfig struct {
	GroupBy      []string      `json:"groupBy"`
	Aggregations []Aggregation `json:"aggregations"`
}

func (n *AggregateNode) Execute(ctx context.Context, deps Deps, in ExecInput) (connections.QueryResult, error) {
	var cfg AggregateConfig
	if err := json.Unmarshal(in.Config, &cfg); err != nil {
		return connections.QueryResult{}, fmt.Errorf("invalid aggregate config: %w", err)
	}
	if len(cfg.Aggregations) == 0 {
		return connections.QueryResult{}, fmt.Errorf("aggregate node requires at least one aggregation")
	}

	input, err := in.SingleInput()
	if err != nil {
		return connections.QueryResult{}, err
	}

	type group struct {
		keyValues map[string]any
		values    map[string][]float64 // numeric accumulator, per aggregation "as" name
		count     int
	}

	groups := map[string]*group{}
	var order []string

	for _, row := range input.Rows {
		keyParts := make([]string, len(cfg.GroupBy))
		keyValues := make(map[string]any, len(cfg.GroupBy))
		for i, g := range cfg.GroupBy {
			keyParts[i] = fmt.Sprintf("%v", row[g])
			keyValues[g] = row[g]
		}
		key := strings.Join(keyParts, "\x1f")

		g, ok := groups[key]
		if !ok {
			g = &group{keyValues: keyValues, values: map[string][]float64{}}
			groups[key] = g
			order = append(order, key)
		}
		g.count++
		for _, agg := range cfg.Aggregations {
			if agg.Op == AggCount {
				continue
			}
			if f, ok := toFloat(row[agg.Field]); ok {
				name := aggName(agg)
				g.values[name] = append(g.values[name], f)
			}
		}
	}

	sort.Strings(order)

	result := connections.QueryResult{Rows: []map[string]any{}}
	colSeen := map[string]bool{}
	addCol := func(c string) {
		if !colSeen[c] {
			colSeen[c] = true
			result.Columns = append(result.Columns, c)
		}
	}
	for _, g := range cfg.GroupBy {
		addCol(g)
	}
	for _, agg := range cfg.Aggregations {
		addCol(aggName(agg))
	}

	for _, key := range order {
		g := groups[key]
		row := make(map[string]any, len(cfg.GroupBy)+len(cfg.Aggregations))
		for k, v := range g.keyValues {
			row[k] = v
		}
		for _, agg := range cfg.Aggregations {
			name := aggName(agg)
			if agg.Op == AggCount {
				row[name] = g.count
				continue
			}
			row[name] = reduce(agg.Op, g.values[name])
		}
		result.Rows = append(result.Rows, row)
		result.RowCount++
	}

	return result, nil
}

func aggName(a Aggregation) string {
	if a.As != "" {
		return a.As
	}
	return string(a.Op) + "_" + a.Field
}

func reduce(op AggregateOp, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	switch op {
	case AggSum:
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum
	case AggAvg:
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	case AggMin:
		min := values[0]
		for _, v := range values {
			if v < min {
				min = v
			}
		}
		return min
	case AggMax:
		max := values[0]
		for _, v := range values {
			if v > max {
				max = v
			}
		}
		return max
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
	default:
		return 0, false
	}
}
