package workflow

import "github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"

func buildDAG(def Definition) map[string][]string {
	dag := make(map[string][]string)
	for _, edge := range def.Edges {
		dag[edge.Source] = append(dag[edge.Source], edge.Target)
	}
	return dag
}

func detectFusionGroups(nodesInOrder []string, dag map[string][]string) [][]string {
	var groups [][]string
	for i := 0; i < len(nodesInOrder)-1; i++ {
		current := nodesInOrder[i]
		if len(dag[current]) != 1 {
			continue
		}
		next := dag[current][0]
		if nodesInOrder[i+1] != next || len(dag[next]) > 1 {
			continue
		}
		groups = append(groups, []string{current, next})
	}
	return groups
}

func (e *Engine) projectionHints(def Definition) map[string]nodes.ProjectionHint {
	nodeByID := map[string]Node{}
	for _, node := range def.Nodes {
		nodeByID[node.ID] = node
	}
	downstream := buildDAG(def)
	result := map[string]nodes.ProjectionHint{}
	for _, node := range def.Nodes {
		if node.Type != NodeTypeSource {
			continue
		}
		cols, ok := collectProjectedColumns(node.ID, downstream, nodeByID)
		if ok && len(cols) > 0 {
			result[node.ID] = nodes.ProjectionHint{Columns: cols}
		}
	}
	return result
}
