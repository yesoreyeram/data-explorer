package workflow

import (
	"encoding/json"
	"regexp"
	"sort"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

var identifierPattern = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\b`)

func collectProjectedColumns(sourceID string, dag map[string][]string, nodeByID map[string]Node) ([]string, bool) {
	seen := map[string]struct{}{}
	queue := append([]string(nil), dag[sourceID]...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		node, ok := nodeByID[id]
		if !ok {
			continue
		}
		switch node.Type {
		case NodeTypeFilter:
			var cfg struct {
				Expression string `json:"expression"`
			}
			if json.Unmarshal(node.Config, &cfg) != nil {
				return nil, false
			}
			for _, name := range expressionColumns(cfg.Expression) {
				seen[name] = struct{}{}
			}
			queue = append(queue, dag[id]...)
		case NodeTypeAggregate:
			var cfg struct {
				GroupBy      []string        `json:"groupBy"`
				Aggregations []dataframe.Agg `json:"aggregations"`
			}
			if json.Unmarshal(node.Config, &cfg) != nil {
				return nil, false
			}
			for _, name := range cfg.GroupBy {
				seen[name] = struct{}{}
			}
			for _, agg := range cfg.Aggregations {
				if agg.Field != "" {
					seen[agg.Field] = struct{}{}
				}
			}
			queue = append(queue, dag[id]...)
		case NodeTypeJoin:
			var cfg struct {
				LeftKey  string `json:"leftKey"`
				RightKey string `json:"rightKey"`
			}
			if json.Unmarshal(node.Config, &cfg) != nil {
				return nil, false
			}
			if cfg.LeftKey != "" {
				seen[cfg.LeftKey] = struct{}{}
			}
			if cfg.RightKey != "" {
				seen[cfg.RightKey] = struct{}{}
			}
		case NodeTypeTransform:
			return nil, false
		default:
			queue = append(queue, dag[id]...)
		}
	}
	out := make([]string, 0, len(seen))
	for col := range seen {
		out = append(out, col)
	}
	sort.Strings(out)
	return out, true
}

func expressionColumns(expression string) []string {
	matches := identifierPattern.FindAllString(expression, -1)
	seen := map[string]struct{}{}
	for _, match := range matches {
		switch match {
		case "true", "false", "null", "and", "or":
			continue
		}
		seen[match] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
