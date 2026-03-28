package linter

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/superplanehq/superplane/pkg/models"
	"github.com/superplanehq/superplane/pkg/registry"
)

// destructiveComponents are component names that should have an approval gate upstream.
var destructiveComponents = []string{
	"pagerduty.resolveIncident",
	"pagerduty.escalateIncident",
	"github.deleteRelease",
	"github.createRelease",
	"ssh",
}

// nodeRefExpr matches $['NodeName'] and $["NodeName"] patterns in expression strings.
var nodeRefExpr = regexp.MustCompile(`\$\[['"]([^'"]+)['"]\]`)

// knownContextVars are well-known non-node context variables available in expressions.
// References to these names should not be flagged as invalid node references.
var knownContextVars = map[string]bool{
	"secrets": true,
}

// ── Rule: orphan-node ────────────────────────────────────────────────────────

// checkOrphanNodes flags any non-widget node not reachable from any trigger.
func checkOrphanNodes(result *LintResult, nodes []models.Node, outgoing map[string][]models.Edge) {
	roots := triggerIDs(nodes)

	// If there are no triggers at all, every non-widget node is an orphan.
	reachable := bfsForward(roots, outgoing)

	for _, n := range nodes {
		if n.Type == "widget" {
			continue
		}
		if !reachable[n.ID] {
			result.addError(
				"orphan-node",
				n.ID,
				n.Name,
				fmt.Sprintf("Node %q is not reachable from any trigger", n.Name),
			)
		}
	}
}

// ── Rule: missing-approval-gate ─────────────────────────────────────────────

// checkMissingApprovalGate warns when a destructive component has no approval upstream.
func checkMissingApprovalGate(result *LintResult, nodes []models.Node, incoming map[string][]models.Edge) {
	// Build a quick lookup of node ID → node for ancestor walks.
	byID := make(map[string]models.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}

	for _, n := range nodes {
		if !isDestructive(n) {
			continue
		}

		ancestors := bfsBackward(n.ID, incoming)
		hasApproval := false
		for ancestorID := range ancestors {
			if ancestorID == n.ID {
				continue
			}
			ancestor, ok := byID[ancestorID]
			if !ok {
				continue
			}
			if ancestor.Ref.Component != nil && ancestor.Ref.Component.Name == "approval" {
				hasApproval = true
				break
			}
		}

		if !hasApproval {
			result.addWarning(
				"missing-approval-gate",
				n.ID,
				n.Name,
				fmt.Sprintf("Destructive action %q has no upstream approval gate", componentName(n)),
			)
		}
	}
}

func isDestructive(n models.Node) bool {
	if n.Ref.Component == nil {
		return false
	}
	name := n.Ref.Component.Name
	if slices.Contains(destructiveComponents, name) {
		return true
	}
	// HTTP DELETE or PUT
	if name == "http" {
		method, _ := n.Configuration["method"].(string)
		method = strings.ToUpper(method)
		return method == "DELETE" || method == "PUT"
	}
	return false
}

// ── Rule: unreachable-branch ────────────────────────────────────────────────

// checkUnreachableBranch warns when an if-node has a branch with no outgoing edge.
func checkUnreachableBranch(result *LintResult, nodes []models.Node, outgoing map[string][]models.Edge) {
	for _, n := range nodes {
		if n.Ref.Component == nil || n.Ref.Component.Name != "if" {
			continue
		}

		wiredChannels := make(map[string]bool)
		for _, e := range outgoing[n.ID] {
			wiredChannels[e.Channel] = true
		}

		for _, ch := range []string{"true", "false"} {
			if !wiredChannels[ch] {
				result.addWarning(
					"unreachable-branch",
					n.ID,
					n.Name,
					fmt.Sprintf("If node %q has no outgoing edge on channel %q", n.Name, ch),
				)
			}
		}
	}
}

// ── Rule: dead-end-node ──────────────────────────────────────────────────────

// checkDeadEndNodes warns when a component has defined output channels but none are wired.
// If reg is nil the rule is skipped.
func checkDeadEndNodes(result *LintResult, nodes []models.Node, outgoing map[string][]models.Edge, reg *registry.Registry) {
	if reg == nil {
		return
	}

	for _, n := range nodes {
		if n.Ref.Component == nil {
			continue
		}

		comp, err := reg.GetComponent(n.Ref.Component.Name)
		if err != nil {
			continue
		}

		channels := comp.OutputChannels(n.Configuration)
		if len(channels) == 0 {
			// Terminal by design — no outputs expected.
			continue
		}

		if len(outgoing[n.ID]) == 0 {
			result.addWarning(
				"dead-end-node",
				n.ID,
				n.Name,
				fmt.Sprintf("Node %q has output channels but no outgoing edges", n.Name),
			)
		}
	}
}

// ── Rule: single-input-merge ─────────────────────────────────────────────────

// checkSingleInputMerge emits info when a merge node has fewer than 2 incoming edges.
func checkSingleInputMerge(result *LintResult, nodes []models.Node, incoming map[string][]models.Edge) {
	for _, n := range nodes {
		if n.Ref.Component == nil || n.Ref.Component.Name != "merge" {
			continue
		}

		if len(incoming[n.ID]) < 2 {
			result.addInfo(
				"single-input-merge",
				n.ID,
				n.Name,
				fmt.Sprintf("Merge node %q has only %d incoming edge(s) — consider removing it", n.Name, len(incoming[n.ID])),
			)
		}
	}
}

// ── Rule: empty-expression-field ─────────────────────────────────────────────

// checkEmptyExpressionFields catches semantic config gaps not covered by ValidateConfiguration.
func checkEmptyExpressionFields(result *LintResult, nodes []models.Node) {
	for _, n := range nodes {
		if n.Ref.Component == nil {
			continue
		}
		name := n.Ref.Component.Name

		switch {
		case name == "if":
			checkEmptyField(result, n, "expression", "If node")
		case name == "filter":
			checkEmptyField(result, n, "expression", "Filter node")
		case strings.HasPrefix(name, "claude.") || name == "claude":
			checkEmptyField(result, n, "prompt", "Claude node")
		case name == "http":
			checkEmptyField(result, n, "url", "HTTP node")
		case name == "slack.sendTextMessage":
			checkEmptyField(result, n, "channel", "Slack node")
		}
	}
}

func checkEmptyField(result *LintResult, n models.Node, field, label string) {
	val, _ := n.Configuration[field].(string)
	if strings.TrimSpace(val) == "" {
		result.addError(
			"empty-expression-field",
			n.ID,
			n.Name,
			fmt.Sprintf("%s %q has an empty %q field", label, n.Name, field),
		)
	}
}

// ── Rule: invalid-node-reference ─────────────────────────────────────────────

// checkInvalidNodeReferences warns when $['NodeName'] in an expression points to an unknown node.
func checkInvalidNodeReferences(result *LintResult, nodes []models.Node) {
	names := nodeNameSet(nodes)
	for _, n := range nodes {
		seen := make(map[string]bool) // deduplicate: ref+field combos per node
		walkStrings(n.Configuration, "", func(key, value string) {
			for _, match := range nodeRefExpr.FindAllStringSubmatch(value, -1) {
				ref := match[1]
				dedupKey := ref + "\x00" + key
				if names[ref] || seen[dedupKey] || knownContextVars[ref] {
					continue
				}
				seen[dedupKey] = true
				result.addWarning(
					"invalid-node-reference",
					n.ID,
					n.Name,
					fmt.Sprintf("Node %q references unknown node %q in field %q", n.Name, ref, key),
				)
			}
		})
	}
}

// ── Rule: unbalanced-braces ───────────────────────────────────────────────────

// checkUnbalancedBraces errors when {{ }} delimiters are not balanced in config values.
func checkUnbalancedBraces(result *LintResult, nodes []models.Node) {
	for _, n := range nodes {
		walkStrings(n.Configuration, "", func(key, value string) {
			if !balancedBraces(value) {
				result.addError(
					"unbalanced-braces",
					n.ID,
					n.Name,
					fmt.Sprintf("Node %q has unbalanced {{ }} in field %q", n.Name, key),
				)
			}
		})
	}
}

// balancedBraces returns true if {{ and }} are balanced in s.
func balancedBraces(s string) bool {
	depth := 0
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '{' && s[i+1] == '{' {
			depth++
			i++
		} else if s[i] == '}' && s[i+1] == '}' {
			if depth == 0 {
				return false
			}
			depth--
			i++
		}
	}
	return depth == 0
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// walkStrings calls fn for every string value found recursively in a config map.
// prefix is the dot-separated key path of the parent (empty at the top level).
func walkStrings(config map[string]any, prefix string, fn func(key, value string)) {
	for k, v := range config {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			fn(fullKey, val)
		case map[string]any:
			walkStrings(val, fullKey, fn)
		case []any:
			for _, item := range val {
				switch elem := item.(type) {
				case string:
					fn(fullKey, elem)
				case map[string]any:
					walkStrings(elem, fullKey, fn)
				}
			}
		}
	}
}

func componentName(n models.Node) string {
	if n.Ref.Component != nil {
		return n.Ref.Component.Name
	}
	return n.Name
}

// ── Rule: cycle-detected ─────────────────────────────────────────────────────

// checkCycleDetected errors when a cycle exists in the graph (excluding loop nodes).
// Loops are intentional cycles, so we skip edges from loop nodes when detecting accidental cycles.
func checkCycleDetected(result *LintResult, nodes []models.Node, outgoing map[string][]models.Edge) {
	// Build a set of loop node IDs to exclude from cycle detection
	loopNodes := make(map[string]bool)
	for _, n := range nodes {
		if n.Ref.Component != nil && n.Ref.Component.Name == "loop" {
			loopNodes[n.ID] = true
		}
	}

	// Build adjacency list excluding edges from loop nodes
	adj := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize all nodes in the maps
	for _, n := range nodes {
		adj[n.ID] = nil
		inDegree[n.ID] = 0
	}

	// Build graph excluding edges from loop nodes
	for srcID, edges := range outgoing {
		if loopNodes[srcID] {
			continue // Skip edges from loop nodes - intentional cycles
		}
		for _, e := range edges {
			adj[srcID] = append(adj[srcID], e.TargetID)
			inDegree[e.TargetID]++
		}
	}

	// Kahn's algorithm for topological sort
	// Nodes with in-degree 0 are added to queue
	queue := []string{}
	for nodeID := range inDegree {
		if inDegree[nodeID] == 0 {
			queue = append(queue, nodeID)
		}
	}

	processed := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		processed++

		for _, neighbor := range adj[cur] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// If not all nodes were processed, there's a cycle
	if processed < len(nodes) {
		// Find nodes in the cycle (nodes with remaining in-degree > 0)
		cycleNodeIDs := []string{}
		for nodeID, degree := range inDegree {
			if degree > 0 {
				cycleNodeIDs = append(cycleNodeIDs, nodeID)
			}
		}

		// Report error for each node in the cycle
		for _, n := range nodes {
			if slices.Contains(cycleNodeIDs, n.ID) {
				result.addError(
					"cycle-detected",
					n.ID,
					n.Name,
					fmt.Sprintf("Node %q is part of a cycle in the workflow graph", n.Name),
				)
			}
		}
	}
}
