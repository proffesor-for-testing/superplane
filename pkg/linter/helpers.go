package linter

import "github.com/superplanehq/superplane/pkg/models"

func buildOutgoing(edges []models.Edge) map[string][]models.Edge {
	m := make(map[string][]models.Edge)
	for _, e := range edges {
		m[e.SourceID] = append(m[e.SourceID], e)
	}
	return m
}

func buildIncoming(edges []models.Edge) map[string][]models.Edge {
	m := make(map[string][]models.Edge)
	for _, e := range edges {
		m[e.TargetID] = append(m[e.TargetID], e)
	}
	return m
}

// bfsForward returns the set of node IDs reachable from startIDs following outgoing edges.
func bfsForward(startIDs []string, outgoing map[string][]models.Edge) map[string]bool {
	visited := make(map[string]bool)
	queue := append([]string(nil), startIDs...)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, e := range outgoing[cur] {
			if !visited[e.TargetID] {
				queue = append(queue, e.TargetID)
			}
		}
	}
	return visited
}

// bfsBackward returns the set of node IDs that can reach startID following edges in reverse.
func bfsBackward(startID string, incoming map[string][]models.Edge) map[string]bool {
	visited := make(map[string]bool)
	queue := []string{startID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, e := range incoming[cur] {
			if !visited[e.SourceID] {
				queue = append(queue, e.SourceID)
			}
		}
	}
	return visited
}

func triggerIDs(nodes []models.Node) []string {
	var ids []string
	for _, n := range nodes {
		if n.Ref.Trigger != nil {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

func nodeNameSet(nodes []models.Node) map[string]bool {
	m := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		m[n.Name] = true
	}
	return m
}
