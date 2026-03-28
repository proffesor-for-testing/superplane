package linter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/superplanehq/superplane/pkg/models"
)

// ---------------------------------------------------------------------------
// Rule 1: Orphan Node Detection
// ---------------------------------------------------------------------------

// OrphanNodeRule finds nodes that are disconnected or unreachable.
// It detects two cases:
//   - Fully orphaned: no incoming AND no outgoing edges
//   - Unreachable: no incoming edges (but has outgoing) for non-trigger nodes
//
// Trigger nodes are excluded because they are valid entry points with no incoming edges.
type OrphanNodeRule struct{}

func (r *OrphanNodeRule) Name() string { return "orphan-node" }

func (r *OrphanNodeRule) Run(spec *CanvasSpec) []Issue {
	g := buildGraph(spec)
	var issues []Issue

	for _, node := range spec.Nodes {
		// Trigger nodes are expected to have no incoming edges.
		if node.Type == models.NodeTypeTrigger {
			continue
		}

		hasIncoming := len(g.incoming[node.ID]) > 0
		hasOutgoing := len(g.outgoing[node.ID]) > 0

		if !hasIncoming && !hasOutgoing {
			issues = append(issues, Issue{
				Rule:     r.Name(),
				Severity: SeverityError,
				NodeID:   node.ID,
				Message:  fmt.Sprintf("node %q (%s) is orphaned — not connected to any other node", node.Name, node.ID),
			})
		} else if !hasIncoming && hasOutgoing {
			issues = append(issues, Issue{
				Rule:     r.Name(),
				Severity: SeverityWarning,
				NodeID:   node.ID,
				Message:  fmt.Sprintf("node %q (%s) is unreachable — has outgoing edges but no incoming edges", node.Name, node.ID),
			})
		}
	}

	// Check for dangling edges referencing non-existent nodes.
	for _, edge := range spec.Edges {
		if _, ok := g.nodes[edge.SourceID]; !ok {
			issues = append(issues, Issue{
				Rule:     r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge references non-existent source node %q", edge.SourceID),
			})
		}
		if _, ok := g.nodes[edge.TargetID]; !ok {
			issues = append(issues, Issue{
				Rule:     r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge references non-existent target node %q", edge.TargetID),
			})
		}
	}

	return issues
}

// ---------------------------------------------------------------------------
// Rule 2: Missing Node Reference
// ---------------------------------------------------------------------------

// MissingRefRule checks that every node has the appropriate ref set for its type.
type MissingRefRule struct{}

func (r *MissingRefRule) Name() string { return "missing-ref" }

func (r *MissingRefRule) Run(spec *CanvasSpec) []Issue {
	var issues []Issue

	for _, node := range spec.Nodes {
		switch node.Type {
		case models.NodeTypeTrigger:
			if node.Ref.Trigger == nil || node.Ref.Trigger.Name == "" {
				issues = append(issues, Issue{
					Rule:     r.Name(),
					Severity: SeverityError,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("trigger node %q is missing a trigger reference", node.Name),
				})
			}
		case models.NodeTypeComponent:
			if node.Ref.Component == nil || node.Ref.Component.Name == "" {
				issues = append(issues, Issue{
					Rule:     r.Name(),
					Severity: SeverityError,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("component node %q is missing a component reference", node.Name),
				})
			}
		case models.NodeTypeBlueprint:
			if node.Ref.Blueprint == nil || node.Ref.Blueprint.ID == "" {
				issues = append(issues, Issue{
					Rule:     r.Name(),
					Severity: SeverityError,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("blueprint node %q is missing a blueprint reference", node.Name),
				})
			}
		case models.NodeTypeWidget:
			if node.Ref.Widget == nil || node.Ref.Widget.Name == "" {
				issues = append(issues, Issue{
					Rule:     r.Name(),
					Severity: SeverityError,
					NodeID:   node.ID,
					Message:  fmt.Sprintf("widget node %q is missing a widget reference", node.Name),
				})
			}
		default:
			issues = append(issues, Issue{
				Rule:     r.Name(),
				Severity: SeverityError,
				NodeID:   node.ID,
				Message:  fmt.Sprintf("node %q has unknown type %q", node.Name, node.Type),
			})
		}
	}

	return issues
}

// ---------------------------------------------------------------------------
// Rule 3: Cycle Detection
// ---------------------------------------------------------------------------

// CycleDetectionRule uses DFS to find all cycles in the canvas graph.
// It reports every cycle found, including self-loops, with the full cycle path.
type CycleDetectionRule struct{}

func (r *CycleDetectionRule) Name() string { return "cycle-detected" }

func (r *CycleDetectionRule) Run(spec *CanvasSpec) []Issue {
	g := buildGraph(spec)
	var issues []Issue

	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully processed
	)

	color := make(map[string]int, len(spec.Nodes))
	for _, n := range spec.Nodes {
		color[n.ID] = white
	}

	// Track the current DFS path for reporting.
	var path []string

	var dfs func(id string)
	dfs = func(id string) {
		color[id] = gray
		path = append(path, id)

		for _, next := range g.outgoing[id] {
			if color[next] == gray {
				// Build cycle path from the cycle start node to current.
				cycleStart := -1
				for i, p := range path {
					if p == next {
						cycleStart = i
						break
					}
				}
				cyclePath := append(path[cycleStart:], next)
				issues = append(issues, Issue{
					Rule:     r.Name(),
					Severity: SeverityError,
					NodeID:   next,
					Message:  fmt.Sprintf("cycle detected: %s", strings.Join(cyclePath, " → ")),
				})
			} else if color[next] == white {
				dfs(next)
			}
		}

		path = path[:len(path)-1]
		color[id] = black
	}

	for _, n := range spec.Nodes {
		if color[n.ID] == white {
			dfs(n.ID)
		}
	}

	return issues
}

// ---------------------------------------------------------------------------
// Rule 4: Approval Gate Before Destructive Actions
// ---------------------------------------------------------------------------

// destructivePatterns are component name patterns that indicate a destructive action.
var destructivePatterns = []string{
	"delete", "destroy", "terminate", "remove", "drop",
	"rollback", "restart", "kill", "shutdown", "purge", "truncate",
}

// isDestructiveComponent checks if a component name suggests a destructive action.
func isDestructiveComponent(name string) bool {
	lower := strings.ToLower(name)
	for _, p := range destructivePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// ApprovalGateRule checks that every destructive component has an approval node
// somewhere upstream in its execution path.
type ApprovalGateRule struct{}

func (r *ApprovalGateRule) Name() string { return "approval-before-destructive" }

func (r *ApprovalGateRule) Run(spec *CanvasSpec) []Issue {
	g := buildGraph(spec)
	var issues []Issue

	for _, node := range spec.Nodes {
		if node.Type != models.NodeTypeComponent {
			continue
		}
		if node.Ref.Component == nil {
			continue
		}
		if !isDestructiveComponent(node.Ref.Component.Name) {
			continue
		}

		// Walk all ancestors looking for an approval node.
		ancestors := g.ancestors(node.ID)
		hasApproval := false
		for ancestorID := range ancestors {
			if an, ok := g.nodes[ancestorID]; ok {
				if an.Type == models.NodeTypeComponent && an.Ref.Component != nil && an.Ref.Component.Name == "approval" {
					hasApproval = true
					break
				}
			}
		}

		if !hasApproval {
			issues = append(issues, Issue{
				Rule:     r.Name(),
				Severity: SeverityWarning,
				NodeID:   node.ID,
				Message:  fmt.Sprintf("destructive component %q (%s) has no approval gate upstream", node.Ref.Component.Name, node.ID),
			})
		}
	}

	return issues
}

// ---------------------------------------------------------------------------
// Rule 5: Expression Syntax Validation
// ---------------------------------------------------------------------------

// expressionRegex matches SuperPlane expression placeholders: {{ ... }}
var expressionRegex = regexp.MustCompile(`\{\{(.*?)\}\}`)

// ExpressionSyntaxRule checks that expression fields contain valid {{ }} syntax
// with non-empty content.
type ExpressionSyntaxRule struct{}

func (r *ExpressionSyntaxRule) Name() string { return "expression-syntax" }

func (r *ExpressionSyntaxRule) Run(spec *CanvasSpec) []Issue {
	var issues []Issue

	for _, node := range spec.Nodes {
		if node.Configuration == nil {
			continue
		}

		r.checkMap(&issues, node.ID, node.Name, "", node.Configuration)
	}

	return issues
}

// checkMap recursively scans a configuration map for expression issues.
func (r *ExpressionSyntaxRule) checkMap(issues *[]Issue, nodeID, nodeName, prefix string, m map[string]any) {
	for key, value := range m {
		fieldPath := key
		if prefix != "" {
			fieldPath = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			r.checkString(issues, nodeID, nodeName, fieldPath, v)
		case map[string]any:
			r.checkMap(issues, nodeID, nodeName, fieldPath, v)
		case []any:
			for i, item := range v {
				itemPath := fmt.Sprintf("%s[%d]", fieldPath, i)
				if s, ok := item.(string); ok {
					r.checkString(issues, nodeID, nodeName, itemPath, s)
				} else if nested, ok := item.(map[string]any); ok {
					r.checkMap(issues, nodeID, nodeName, itemPath, nested)
				}
			}
		}
	}
}

// checkString validates expression syntax in a single string value.
func (r *ExpressionSyntaxRule) checkString(issues *[]Issue, nodeID, nodeName, fieldPath, str string) {
	matches := expressionRegex.FindAllStringSubmatch(str, -1)
	for _, match := range matches {
		inner := strings.TrimSpace(match[1])
		if inner == "" {
			*issues = append(*issues, Issue{
				Rule:     r.Name(),
				Severity: SeverityError,
				NodeID:   nodeID,
				Message:  fmt.Sprintf("node %q field %q has an empty expression placeholder {{}}", nodeName, fieldPath),
			})
		}
	}

	// Check for unbalanced braces — opening {{ without closing }}.
	openCount := strings.Count(str, "{{")
	closeCount := strings.Count(str, "}}")
	if openCount != closeCount {
		*issues = append(*issues, Issue{
			Rule:     r.Name(),
			Severity: SeverityError,
			NodeID:   nodeID,
			Message:  fmt.Sprintf("node %q field %q has unbalanced expression braces (%d opening, %d closing)", nodeName, fieldPath, openCount, closeCount),
		})
	}
}
