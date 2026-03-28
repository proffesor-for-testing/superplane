package linter

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/superplanehq/superplane/pkg/configuration"
	"github.com/superplanehq/superplane/pkg/core"
	"github.com/superplanehq/superplane/pkg/models"
	"github.com/superplanehq/superplane/pkg/registry"
)

// ── stub component for dead-end-node tests ───────────────────────────────────

type stubComponent struct {
	name    string
	outputs []core.OutputChannel
}

func (s *stubComponent) Name() string          { return s.name }
func (s *stubComponent) Label() string         { return s.name }
func (s *stubComponent) Description() string   { return "" }
func (s *stubComponent) Documentation() string { return "" }
func (s *stubComponent) Icon() string          { return "" }
func (s *stubComponent) Color() string         { return "" }
func (s *stubComponent) ExampleOutput() map[string]any                                                  { return nil }
func (s *stubComponent) OutputChannels(_ any) []core.OutputChannel                                      { return s.outputs }
func (s *stubComponent) Configuration() []configuration.Field                                           { return nil }
func (s *stubComponent) Setup(_ core.SetupContext) error                                                { return nil }
func (s *stubComponent) ProcessQueueItem(_ core.ProcessQueueContext) (*uuid.UUID, error)               { return nil, nil }
func (s *stubComponent) Execute(_ core.ExecutionContext) error                                          { return nil }
func (s *stubComponent) Actions() []core.Action                                                         { return nil }
func (s *stubComponent) HandleAction(_ core.ActionContext) error                                        { return nil }
func (s *stubComponent) HandleWebhook(_ core.WebhookRequestContext) (int, *core.WebhookResponseBody, error) {
	return 0, nil, nil
}
func (s *stubComponent) Cancel(_ core.ExecutionContext) error  { return nil }
func (s *stubComponent) Cleanup(_ core.SetupContext) error     { return nil }

// registryWith returns a Registry whose Components map is seeded with the given components.
func registryWith(comps ...*stubComponent) *registry.Registry {
	m := make(map[string]core.Component, len(comps))
	for _, c := range comps {
		m[c.name] = c
	}
	return &registry.Registry{Components: m}
}

// ── Fixtures ──────────────────────────────────────────────────────────────────

func triggerNode(id, name string) models.Node {
	return models.Node{
		ID:   id,
		Name: name,
		Type: "trigger",
		Ref:  models.NodeRef{Trigger: &models.TriggerRef{Name: "pagerduty.onIncident"}},
	}
}

func componentNode(id, name, compName string) models.Node {
	return models.Node{
		ID:            id,
		Name:          name,
		Type:          "component",
		Ref:           models.NodeRef{Component: &models.ComponentRef{Name: compName}},
		Configuration: map[string]any{},
	}
}

func componentNodeWithConfig(id, name, compName string, config map[string]any) models.Node {
	return models.Node{
		ID:            id,
		Name:          name,
		Type:          "component",
		Ref:           models.NodeRef{Component: &models.ComponentRef{Name: compName}},
		Configuration: config,
	}
}

func widgetNode(id, name string) models.Node {
	return models.Node{
		ID:   id,
		Name: name,
		Type: "widget",
		Ref:  models.NodeRef{Widget: &models.WidgetRef{Name: "group"}},
	}
}

func edge(src, tgt, ch string) models.Edge {
	return models.Edge{SourceID: src, TargetID: tgt, Channel: ch}
}

// ── orphan-node ───────────────────────────────────────────────────────────────

func TestOrphanNode_Isolated(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Fetch Logs", "noop"),
	}
	edges := []models.Edge{} // c1 not connected to anything

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "fail", result.Status)
	orphanErrs := filterRule(result.Errors, "orphan-node")
	assert.Len(t, orphanErrs, 1)
	assert.Equal(t, "c1", orphanErrs[0].NodeID)
}

func TestOrphanNode_Connected(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Fetch Logs", "noop"),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "pass", result.Status)
	assert.Empty(t, result.Errors)
}

func TestOrphanNode_WidgetIgnored(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		widgetNode("w1", "Group"),
	}
	edges := []models.Edge{}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "pass", result.Status)
	assert.Empty(t, result.Errors)
}

func TestOrphanNode_NoTriggers_AllOrphans(t *testing.T) {
	nodes := []models.Node{
		componentNode("c1", "Node A", "noop"),
		componentNode("c2", "Node B", "noop"),
	}
	edges := []models.Edge{edge("c1", "c2", "default")}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "fail", result.Status)
	orphanErrs := filterRule(result.Errors, "orphan-node")
	assert.Len(t, orphanErrs, 2)
}

// ── missing-approval-gate ─────────────────────────────────────────────────────

func TestMissingApprovalGate_Destructive_NoApproval(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Delete Release", "github.deleteRelease"),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, "missing-approval-gate", result.Warnings[0].Rule)
	assert.Equal(t, "c1", result.Warnings[0].NodeID)
}

func TestMissingApprovalGate_Destructive_WithApproval(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("a1", "Approve", "approval"),
		componentNode("c1", "Delete Release", "github.deleteRelease"),
	}
	edges := []models.Edge{
		edge("t1", "a1", "default"),
		edge("a1", "c1", "approved"),
	}

	result := LintCanvas(nodes, edges, nil)
	noApprovalWarnings := filterRule(result.Warnings, "missing-approval-gate")
	assert.Empty(t, noApprovalWarnings)
}

func TestMissingApprovalGate_HTTP_DELETE(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Delete Resource", "http", map[string]any{"method": "DELETE", "url": "https://api.example.com/resource"}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	noApprovalWarnings := filterRule(result.Warnings, "missing-approval-gate")
	assert.Len(t, noApprovalWarnings, 1)
}

func TestMissingApprovalGate_SSH(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Run Script", "ssh", map[string]any{"host": "10.0.0.1", "command": "rm -rf /tmp/old"}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	noApprovalWarnings := filterRule(result.Warnings, "missing-approval-gate")
	assert.Len(t, noApprovalWarnings, 1)
}

func TestMissingApprovalGate_HTTP_PUT(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Update Resource", "http", map[string]any{"method": "PUT", "url": "https://api.example.com/resource"}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	noApprovalWarnings := filterRule(result.Warnings, "missing-approval-gate")
	assert.Len(t, noApprovalWarnings, 1)
}

// ── unreachable-branch ────────────────────────────────────────────────────────

func TestUnreachableBranch_OneBranchMissing(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("if1", "Check Severity", "if", map[string]any{"expression": "severity == 'high'"}),
		componentNode("c1", "Page Engineer", "noop"),
	}
	edges := []models.Edge{
		edge("t1", "if1", "default"),
		edge("if1", "c1", "true"), // "false" branch not wired
	}

	result := LintCanvas(nodes, edges, nil)
	branches := filterRule(result.Warnings, "unreachable-branch")
	assert.Len(t, branches, 1)
	assert.Contains(t, branches[0].Message, "false")
}

func TestUnreachableBranch_BothBranchesMissing(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("if1", "Check Severity", "if", map[string]any{"expression": "severity == 'high'"}),
	}
	edges := []models.Edge{
		edge("t1", "if1", "default"),
	}

	result := LintCanvas(nodes, edges, nil)
	branches := filterRule(result.Warnings, "unreachable-branch")
	assert.Len(t, branches, 2)
}

func TestUnreachableBranch_BothWired(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("if1", "Check Severity", "if", map[string]any{"expression": "severity == 'high'"}),
		componentNode("c1", "High Path", "noop"),
		componentNode("c2", "Low Path", "noop"),
	}
	edges := []models.Edge{
		edge("t1", "if1", "default"),
		edge("if1", "c1", "true"),
		edge("if1", "c2", "false"),
	}

	result := LintCanvas(nodes, edges, nil)
	branches := filterRule(result.Warnings, "unreachable-branch")
	assert.Empty(t, branches)
}

// ── single-input-merge ────────────────────────────────────────────────────────

func TestSingleInputMerge(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Step A", "noop"),
		componentNode("m1", "Wait for all", "merge"),
	}
	edges := []models.Edge{
		edge("t1", "c1", "default"),
		edge("c1", "m1", "default"), // only 1 incoming
	}

	result := LintCanvas(nodes, edges, nil)
	mergeInfo := filterRule(result.Info, "single-input-merge")
	assert.Len(t, mergeInfo, 1)
}

func TestSingleInputMerge_TwoInputs(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Step A", "noop"),
		componentNode("c2", "Step B", "noop"),
		componentNode("m1", "Wait for all", "merge"),
	}
	edges := []models.Edge{
		edge("t1", "c1", "default"),
		edge("t1", "c2", "default"),
		edge("c1", "m1", "default"),
		edge("c2", "m1", "default"),
	}

	result := LintCanvas(nodes, edges, nil)
	mergeInfo := filterRule(result.Info, "single-input-merge")
	assert.Empty(t, mergeInfo)
}

// ── nil-configuration ─────────────────────────────────────────────────────────

func TestNilConfiguration_NoPanic(t *testing.T) {
	// A component node with nil Configuration must not panic.
	n := models.Node{
		ID:   "c1",
		Name: "No Config",
		Type: "component",
		Ref:  models.NodeRef{Component: &models.ComponentRef{Name: "noop"}},
		// Configuration intentionally left nil
	}
	nodes := []models.Node{triggerNode("t1", "Trigger"), n}
	edges := []models.Edge{edge("t1", "c1", "default")}

	assert.NotPanics(t, func() {
		LintCanvas(nodes, edges, nil)
	})
}

// ── empty-expression-field ────────────────────────────────────────────────────

func TestEmptyExpressionField_IfNode(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("if1", "Branch", "if", map[string]any{"expression": ""}),
	}
	edges := []models.Edge{edge("t1", "if1", "default")}

	result := LintCanvas(nodes, edges, nil)
	emptyErrs := filterRule(result.Errors, "empty-expression-field")
	assert.Len(t, emptyErrs, 1)
	assert.Equal(t, "if1", emptyErrs[0].NodeID)
}

func TestEmptyExpressionField_HTTPNode(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Call API", "http", map[string]any{"url": "  "}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	emptyErrs := filterRule(result.Errors, "empty-expression-field")
	assert.Len(t, emptyErrs, 1)
}

// ── invalid-node-reference ────────────────────────────────────────────────────

func TestInvalidNodeReference(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Triage", "claude.complete", map[string]any{
			"prompt": "Summarize $['GhostNode'].output",
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	refs := filterRule(result.Warnings, "invalid-node-reference")
	assert.Len(t, refs, 1)
	assert.Contains(t, refs[0].Message, "GhostNode")
}

func TestInvalidNodeReference_ValidRef(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Triage", "claude.complete", map[string]any{
			"prompt": "Summarize $['Trigger'].output",
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	refs := filterRule(result.Warnings, "invalid-node-reference")
	assert.Empty(t, refs)
}

// ── unbalanced-braces ─────────────────────────────────────────────────────────

func TestUnbalancedBraces_Error(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Call API", "http", map[string]any{
			"url": "https://api.example.com/{{ path }",
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	braceErrs := filterRule(result.Errors, "unbalanced-braces")
	assert.Len(t, braceErrs, 1)
}

func TestUnbalancedBraces_OK(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Call API", "http", map[string]any{
			"url": "https://api.example.com/{{ path }}",
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	braceErrs := filterRule(result.Errors, "unbalanced-braces")
	assert.Empty(t, braceErrs)
}

// ── status ────────────────────────────────────────────────────────────────────

func TestStatus_WarningsOnlyIsPass(t *testing.T) {
	// Destructive action without approval → warning only, should still be "pass"
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Resolve Incident", "pagerduty.resolveIncident"),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "pass", result.Status)
	assert.NotEmpty(t, result.Warnings)
	assert.Empty(t, result.Errors)
}

func TestStatus_OneErrorIsFail(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Orphan", "http"), // not connected
	}
	edges := []models.Edge{}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "fail", result.Status)
}

func TestStatus_EmptyCanvas_Pass(t *testing.T) {
	result := LintCanvas(nil, nil, nil)
	assert.Equal(t, "pass", result.Status)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.Warnings)
	assert.Empty(t, result.Info)
}

// ── summary ───────────────────────────────────────────────────────────────────

func TestSummary(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("orphan", "Orphan", "http"),
		componentNode("c1", "Resolve Incident", "pagerduty.resolveIncident"),
		componentNodeWithConfig("c2", "Wait for all", "merge", map[string]any{}),
	}
	edges := []models.Edge{
		edge("t1", "c1", "default"),
		edge("t1", "c2", "default"),
	}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, result.Summary.Errors, len(result.Errors))
	assert.Equal(t, result.Summary.Warnings, len(result.Warnings))
	assert.Equal(t, result.Summary.Info, len(result.Info))
	assert.Equal(t, result.Summary.Total, result.Summary.Errors+result.Summary.Warnings+result.Summary.Info)
}

func TestOrphanNode_TriggerOnlyCanvas_NoFalseOrphan(t *testing.T) {
	// A canvas with only a trigger and no downstream nodes should produce no errors.
	nodes := []models.Node{triggerNode("t1", "Webhook")}
	edges := []models.Edge{}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "pass", result.Status)
	assert.Empty(t, result.Errors)
}

// ── dead-end-node ─────────────────────────────────────────────────────────────

func TestDeadEndNode_NoOutgoingEdges(t *testing.T) {
	comp := &stubComponent{name: "has-output", outputs: []core.OutputChannel{{Name: "default", Label: "Default"}}}
	reg := registryWith(comp)
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Fetch Data", "has-output"),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, reg)
	deadEnds := filterRule(result.Warnings, "dead-end-node")
	assert.Len(t, deadEnds, 1)
	assert.Equal(t, "c1", deadEnds[0].NodeID)
}

func TestDeadEndNode_Wired(t *testing.T) {
	comp := &stubComponent{name: "has-output", outputs: []core.OutputChannel{{Name: "default", Label: "Default"}}}
	reg := registryWith(comp)
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Fetch Data", "has-output"),
		componentNode("c2", "Process", "noop"),
	}
	edges := []models.Edge{
		edge("t1", "c1", "default"),
		edge("c1", "c2", "default"),
	}

	result := LintCanvas(nodes, edges, reg)
	deadEnds := filterRule(result.Warnings, "dead-end-node")
	assert.Empty(t, deadEnds)
}

func TestDeadEndNode_TerminalComponent(t *testing.T) {
	// A component with no output channels is terminal by design — should not be flagged.
	comp := &stubComponent{name: "terminal", outputs: nil}
	reg := registryWith(comp)
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Send Alert", "terminal"),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, reg)
	deadEnds := filterRule(result.Warnings, "dead-end-node")
	assert.Empty(t, deadEnds)
}

func TestDeadEndNode_NilRegistry_Skipped(t *testing.T) {
	// When registry is nil the rule should be silently skipped.
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("c1", "Node", "noop"),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	deadEnds := filterRule(result.Warnings, "dead-end-node")
	assert.Empty(t, deadEnds)
}

// ── invalid-node-reference (double-quote + context vars) ─────────────────────

func TestInvalidNodeReference_DoubleQuote(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Triage", "claude.complete", map[string]any{
			"prompt": `Summarize $["GhostNode"].output`,
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	refs := filterRule(result.Warnings, "invalid-node-reference")
	assert.Len(t, refs, 1)
	assert.Contains(t, refs[0].Message, "GhostNode")
}

func TestInvalidNodeReference_DoubleQuoteValidRef(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Triage", "claude.complete", map[string]any{
			"prompt": `Summarize $["Trigger"].output`,
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	refs := filterRule(result.Warnings, "invalid-node-reference")
	assert.Empty(t, refs)
}

func TestInvalidNodeReference_SecretsNotFlagged(t *testing.T) {
	// $['secrets'] and $["secrets"] are well-known context variables, not node names.
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Call API", "http", map[string]any{
			"url": "https://api.example.com/data",
			"headers": map[string]any{
				"Authorization": `Bearer {{ $['secrets'].api_key }}`,
			},
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	refs := filterRule(result.Warnings, "invalid-node-reference")
	assert.Empty(t, refs)
}

// ── empty-expression-field (filter) ──────────────────────────────────────────

func TestEmptyExpressionField_FilterNode(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("f1", "Priority Filter", "filter", map[string]any{"expression": ""}),
	}
	edges := []models.Edge{edge("t1", "f1", "default")}

	result := LintCanvas(nodes, edges, nil)
	emptyErrs := filterRule(result.Errors, "empty-expression-field")
	assert.Len(t, emptyErrs, 1)
	assert.Equal(t, "f1", emptyErrs[0].NodeID)
}

func TestEmptyExpressionField_FilterNode_WithExpression(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("f1", "Priority Filter", "filter", map[string]any{
			"expression": `$["Trigger"].data.priority == "P1"`,
		}),
	}
	edges := []models.Edge{edge("t1", "f1", "default")}

	result := LintCanvas(nodes, edges, nil)
	emptyErrs := filterRule(result.Errors, "empty-expression-field")
	assert.Empty(t, emptyErrs)
}

// ── walkStrings: string values inside []any arrays ───────────────────────────

func TestUnbalancedBraces_InsideStringArray(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Send Message", "slack.sendTextMessage", map[string]any{
			"channel": "#alerts",
			"attachments": []any{
				"{{ open_brace_only }",
			},
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	braceErrs := filterRule(result.Errors, "unbalanced-braces")
	assert.Len(t, braceErrs, 1)
}

func TestInvalidNodeReference_InsideStringArray(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNodeWithConfig("c1", "Process", "http", map[string]any{
			"url":  "https://api.example.com",
			"tags": []any{`$["GhostNode"].id`},
		}),
	}
	edges := []models.Edge{edge("t1", "c1", "default")}

	result := LintCanvas(nodes, edges, nil)
	refs := filterRule(result.Warnings, "invalid-node-reference")
	assert.Len(t, refs, 1)
	assert.Contains(t, refs[0].Message, "GhostNode")
}

// ── helper ────────────────────────────────────────────────────────────────────

func filterRule(issues []Issue, rule string) []Issue {
	var out []Issue
	for _, i := range issues {
		if i.Rule == rule {
			out = append(out, i)
		}
	}
	return out
}

// ── cycle-detected ────────────────────────────────────────────────────────────

func TestCycleDetected_SimpleCycle(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("a", "Node A", "noop"),
		componentNode("b", "Node B", "noop"),
		componentNode("c", "Node C", "noop"),
	}
	edges := []models.Edge{
		edge("t1", "a", "default"),
		edge("a", "b", "default"),
		edge("b", "c", "default"),
		edge("c", "a", "default"), // Creates cycle: a -> b -> c -> a
	}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "fail", result.Status)
	cycleErrs := filterRule(result.Errors, "cycle-detected")
	assert.GreaterOrEqual(t, len(cycleErrs), 1)
	// All three nodes in the cycle should be reported
	cycleNodeIDs := make(map[string]bool)
	for _, err := range cycleErrs {
		cycleNodeIDs[err.NodeID] = true
	}
	assert.True(t, cycleNodeIDs["a"])
	assert.True(t, cycleNodeIDs["b"])
	assert.True(t, cycleNodeIDs["c"])
}

func TestCycleDetected_NoCycle(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("a", "Node A", "noop"),
		componentNode("b", "Node B", "noop"),
	}
	edges := []models.Edge{
		edge("t1", "a", "default"),
		edge("a", "b", "default"),
	}

	result := LintCanvas(nodes, edges, nil)
	cycleErrs := filterRule(result.Errors, "cycle-detected")
	assert.Empty(t, cycleErrs)
}

func TestCycleDetected_SelfLoop(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("a", "Node A", "noop"),
	}
	edges := []models.Edge{
		edge("t1", "a", "default"),
		edge("a", "a", "default"), // Self-loop
	}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "fail", result.Status)
	cycleErrs := filterRule(result.Errors, "cycle-detected")
	assert.GreaterOrEqual(t, len(cycleErrs), 1)
}

func TestCycleDetected_LoopNodeExcluded(t *testing.T) {
	// Loop nodes intentionally create cycles - these should NOT be flagged
	nodes := []models.Node{
		triggerNode("t1", "Trigger"),
		componentNode("a", "Node A", "noop"),
		componentNode("loop", "Loop Node", "loop"),
	}
	edges := []models.Edge{
		edge("t1", "a", "default"),
		edge("a", "loop", "default"),
		edge("loop", "a", "default"), // Edge from loop node creates intentional cycle
	}

	result := LintCanvas(nodes, edges, nil)
	cycleErrs := filterRule(result.Errors, "cycle-detected")
	assert.Empty(t, cycleErrs)
}

func TestCycleDetected_MultipleDisconnectedCycles(t *testing.T) {
	nodes := []models.Node{
		triggerNode("t1", "Trigger 1"),
		componentNode("a1", "Node A1", "noop"),
		componentNode("b1", "Node B1", "noop"),
		triggerNode("t2", "Trigger 2"),
		componentNode("a2", "Node A2", "noop"),
		componentNode("b2", "Node B2", "noop"),
	}
	edges := []models.Edge{
		edge("t1", "a1", "default"),
		edge("a1", "b1", "default"),
		edge("b1", "a1", "default"), // Cycle 1
		edge("t2", "a2", "default"),
		edge("a2", "b2", "default"),
		edge("b2", "a2", "default"), // Cycle 2
	}

	result := LintCanvas(nodes, edges, nil)
	assert.Equal(t, "fail", result.Status)
	cycleErrs := filterRule(result.Errors, "cycle-detected")
	assert.GreaterOrEqual(t, len(cycleErrs), 4) // All 4 nodes in both cycles
}
