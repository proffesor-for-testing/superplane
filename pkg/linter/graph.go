package linter

import "github.com/superplanehq/superplane/pkg/models"

// graph is an internal adjacency-list representation used by rules.
type graph struct {
	nodes    map[string]*models.Node
	outgoing map[string][]string // nodeID -> list of target nodeIDs
	incoming map[string][]string // nodeID -> list of source nodeIDs
}

func buildGraph(spec *CanvasSpec) *graph {
	g := &graph{
		nodes:    make(map[string]*models.Node, len(spec.Nodes)),
		outgoing: make(map[string][]string),
		incoming: make(map[string][]string),
	}

	for i := range spec.Nodes {
		n := &spec.Nodes[i]
		g.nodes[n.ID] = n
	}

	for _, e := range spec.Edges {
		g.outgoing[e.SourceID] = append(g.outgoing[e.SourceID], e.TargetID)
		g.incoming[e.TargetID] = append(g.incoming[e.TargetID], e.SourceID)
	}

	return g
}

// ancestors returns all node IDs that can reach the given node via incoming edges.
func (g *graph) ancestors(nodeID string) map[string]bool {
	visited := make(map[string]bool)
	var walk func(id string)
	walk = func(id string) {
		for _, src := range g.incoming[id] {
			if !visited[src] {
				visited[src] = true
				walk(src)
			}
		}
	}
	walk(nodeID)
	return visited
}
