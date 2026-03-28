package linter

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/superplanehq/superplane/pkg/models"
)

// helper to build a minimal valid canvas with a trigger -> component chain.
func validCanvas() *CanvasSpec {
	return &CanvasSpec{
		Nodes: []models.Node{
			{
				ID:   "trigger-1",
				Name: "Webhook",
				Type: models.NodeTypeTrigger,
				Ref:  models.NodeRef{Trigger: &models.TriggerRef{Name: "webhook"}},
			},
			{
				ID:   "http-1",
				Name: "Fetch Metrics",
				Type: models.NodeTypeComponent,
				Ref:  models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
				Configuration: map[string]any{
					"url": "https://api.datadog.com/metrics",
				},
			},
			{
				ID:   "slack-1",
				Name: "Send Slack",
				Type: models.NodeTypeComponent,
				Ref:  models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
			},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "http-1", Channel: "default"},
			{SourceID: "http-1", TargetID: "slack-1", Channel: "success"},
		},
	}
}

// repoRoot returns the root of the repo for loading fixture files.
func repoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestLinter_ValidCanvas_Passes(t *testing.T) {
	l := New()
	result := l.Lint(validCanvas())

	if !result.Pass {
		t.Errorf("expected valid canvas to pass, got %d issues:", len(result.Issues))
		for _, issue := range result.Issues {
			t.Errorf("  [%s] %s: %s", issue.Severity, issue.Rule, issue.Message)
		}
	}
}

func TestLinter_EmptyCanvas_Passes(t *testing.T) {
	l := New()
	result := l.Lint(&CanvasSpec{})
	if !result.Pass {
		t.Errorf("expected empty canvas to pass, got %d issues", len(result.Issues))
	}
}

// ---------------------------------------------------------------------------
// Rule 1: Orphan Node
// ---------------------------------------------------------------------------

func TestOrphanNodeRule_DetectsOrphan(t *testing.T) {
	spec := validCanvas()
	spec.Nodes = append(spec.Nodes, models.Node{
		ID:   "orphan-1",
		Name: "Orphan",
		Type: models.NodeTypeComponent,
		Ref:  models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
	})

	rule := &OrphanNodeRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 orphan issue, got %d: %v", len(issues), issues)
	}
	if issues[0].NodeID != "orphan-1" {
		t.Errorf("expected orphan node ID orphan-1, got %s", issues[0].NodeID)
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("expected error severity for orphan, got %s", issues[0].Severity)
	}
}

func TestOrphanNodeRule_TriggerNotOrphan(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{
				ID:   "trigger-1",
				Name: "Start",
				Type: models.NodeTypeTrigger,
				Ref:  models.NodeRef{Trigger: &models.TriggerRef{Name: "start"}},
			},
		},
		Edges: []models.Edge{},
	}

	rule := &OrphanNodeRule{}
	issues := rule.Run(spec)

	if len(issues) != 0 {
		t.Errorf("trigger with no edges should not be flagged, got %d issues", len(issues))
	}
}

func TestOrphanNodeRule_UnreachableNode(t *testing.T) {
	// A non-trigger node with outgoing edges but no incoming edges is unreachable.
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "trigger-1", Name: "Start", Type: models.NodeTypeTrigger, Ref: models.NodeRef{Trigger: &models.TriggerRef{Name: "start"}}},
			{ID: "unreachable-1", Name: "Unreachable", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{ID: "target-1", Name: "Target", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "target-1", Channel: "default"},
			{SourceID: "unreachable-1", TargetID: "target-1", Channel: "default"},
		},
	}

	rule := &OrphanNodeRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 unreachable issue, got %d: %v", len(issues), issues)
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("expected warning for unreachable node, got %s", issues[0].Severity)
	}
}

func TestOrphanNodeRule_DanglingEdge(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "a", Name: "A", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
		},
		Edges: []models.Edge{
			{SourceID: "a", TargetID: "nonexistent", Channel: "default"},
		},
	}

	rule := &OrphanNodeRule{}
	issues := rule.Run(spec)

	found := false
	for _, issue := range issues {
		if issue.Message == `edge references non-existent target node "nonexistent"` {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dangling edge issue, got: %v", issues)
	}
}

// ---------------------------------------------------------------------------
// Rule 2: Missing Ref
// ---------------------------------------------------------------------------

func TestMissingRefRule_DetectsMissingComponentRef(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "comp-1", Name: "Bad Component", Type: models.NodeTypeComponent, Ref: models.NodeRef{}},
		},
	}

	rule := &MissingRefRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Rule != "missing-ref" {
		t.Errorf("expected rule missing-ref, got %s", issues[0].Rule)
	}
}

func TestMissingRefRule_DetectsUnknownType(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "weird-1", Name: "Weird", Type: "unknown-type"},
		},
	}

	rule := &MissingRefRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}

func TestMissingRefRule_BlueprintRef(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "bp-1", Name: "Bad Blueprint", Type: models.NodeTypeBlueprint, Ref: models.NodeRef{}},
		},
	}

	rule := &MissingRefRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing blueprint ref, got %d", len(issues))
	}
}

func TestMissingRefRule_WidgetRef(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "w-1", Name: "Bad Widget", Type: models.NodeTypeWidget, Ref: models.NodeRef{}},
		},
	}

	rule := &MissingRefRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing widget ref, got %d", len(issues))
	}
}

func TestMissingRefRule_EmptyRefName(t *testing.T) {
	// Ref pointer is non-nil but Name is empty — should still flag.
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "comp-1", Name: "Empty Ref", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: ""}}},
		},
	}

	rule := &MissingRefRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for empty ref name, got %d", len(issues))
	}
}

// ---------------------------------------------------------------------------
// Rule 3: Cycle Detection
// ---------------------------------------------------------------------------

func TestCycleDetectionRule_DetectsCycle(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "a", Name: "A", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{ID: "b", Name: "B", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{ID: "c", Name: "C", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
		},
		Edges: []models.Edge{
			{SourceID: "a", TargetID: "b", Channel: "default"},
			{SourceID: "b", TargetID: "c", Channel: "default"},
			{SourceID: "c", TargetID: "a", Channel: "default"},
		},
	}

	rule := &CycleDetectionRule{}
	issues := rule.Run(spec)

	if len(issues) == 0 {
		t.Fatal("expected cycle to be detected, got 0 issues")
	}
	// Should contain the cycle path.
	if issues[0].Message == "" {
		t.Error("expected cycle message to contain path")
	}
}

func TestCycleDetectionRule_SelfLoop(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "a", Name: "A", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
		},
		Edges: []models.Edge{
			{SourceID: "a", TargetID: "a", Channel: "default"},
		},
	}

	rule := &CycleDetectionRule{}
	issues := rule.Run(spec)

	if len(issues) == 0 {
		t.Fatal("expected self-loop to be detected")
	}
}

func TestCycleDetectionRule_MultipleCycles(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "a", Name: "A", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{ID: "b", Name: "B", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{ID: "c", Name: "C", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{ID: "d", Name: "D", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
		},
		Edges: []models.Edge{
			{SourceID: "a", TargetID: "b", Channel: "default"},
			{SourceID: "b", TargetID: "a", Channel: "default"}, // cycle 1: a->b->a
			{SourceID: "c", TargetID: "d", Channel: "default"},
			{SourceID: "d", TargetID: "c", Channel: "default"}, // cycle 2: c->d->c
		},
	}

	rule := &CycleDetectionRule{}
	issues := rule.Run(spec)

	if len(issues) < 2 {
		t.Fatalf("expected at least 2 cycle issues, got %d: %v", len(issues), issues)
	}
}

func TestCycleDetectionRule_NoCycle(t *testing.T) {
	rule := &CycleDetectionRule{}
	issues := rule.Run(validCanvas())

	if len(issues) != 0 {
		t.Errorf("expected no cycles, got %d issues", len(issues))
	}
}

func TestCycleDetectionRule_ReportsPath(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "x", Name: "X", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{ID: "y", Name: "Y", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
		},
		Edges: []models.Edge{
			{SourceID: "x", TargetID: "y", Channel: "default"},
			{SourceID: "y", TargetID: "x", Channel: "default"},
		},
	}

	rule := &CycleDetectionRule{}
	issues := rule.Run(spec)

	if len(issues) == 0 {
		t.Fatal("expected cycle")
	}
	// The message should contain " → " indicating a path.
	if !containsStr(issues[0].Message, " → ") {
		t.Errorf("expected cycle path with arrows, got: %s", issues[0].Message)
	}
}

// ---------------------------------------------------------------------------
// Rule 4: Approval Gate
// ---------------------------------------------------------------------------

func TestApprovalGateRule_MissingApproval(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "trigger-1", Name: "Start", Type: models.NodeTypeTrigger, Ref: models.NodeRef{Trigger: &models.TriggerRef{Name: "webhook"}}},
			{ID: "delete-1", Name: "Delete LB", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "digitalocean.deleteLoadBalancer"}}},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "delete-1", Channel: "default"},
		},
	}

	rule := &ApprovalGateRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing approval, got %d", len(issues))
	}
}

func TestApprovalGateRule_WithApproval(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "trigger-1", Name: "Start", Type: models.NodeTypeTrigger, Ref: models.NodeRef{Trigger: &models.TriggerRef{Name: "webhook"}}},
			{ID: "approval-1", Name: "Confirm", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "approval"}}},
			{ID: "delete-1", Name: "Delete LB", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "digitalocean.deleteLoadBalancer"}}},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "approval-1", Channel: "default"},
			{SourceID: "approval-1", TargetID: "delete-1", Channel: "approved"},
		},
	}

	rule := &ApprovalGateRule{}
	issues := rule.Run(spec)

	if len(issues) != 0 {
		t.Errorf("expected no issues with approval gate, got %d", len(issues))
	}
}

func TestApprovalGateRule_RollbackDetected(t *testing.T) {
	// The demo is about rollbacks — ensure they're flagged.
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "trigger-1", Name: "Start", Type: models.NodeTypeTrigger, Ref: models.NodeRef{Trigger: &models.TriggerRef{Name: "webhook"}}},
			{ID: "rollback-1", Name: "Rollback Deploy", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "deploy.rollback"}}},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "rollback-1", Channel: "default"},
		},
	}

	rule := &ApprovalGateRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected rollback to be flagged as destructive, got %d issues", len(issues))
	}
}

func TestApprovalGateRule_RestartDetected(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "trigger-1", Name: "Start", Type: models.NodeTypeTrigger, Ref: models.NodeRef{Trigger: &models.TriggerRef{Name: "webhook"}}},
			{ID: "restart-1", Name: "Restart Service", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "kubernetes.restartDeployment"}}},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "restart-1", Channel: "default"},
		},
	}

	rule := &ApprovalGateRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected restart to be flagged as destructive, got %d issues", len(issues))
	}
}

func TestApprovalGateRule_KillDetected(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "trigger-1", Name: "Start", Type: models.NodeTypeTrigger, Ref: models.NodeRef{Trigger: &models.TriggerRef{Name: "webhook"}}},
			{ID: "kill-1", Name: "Kill Process", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "ssh.killProcess"}}},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "kill-1", Channel: "default"},
		},
	}

	rule := &ApprovalGateRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected kill to be flagged as destructive, got %d issues", len(issues))
	}
}

// ---------------------------------------------------------------------------
// Rule 5: Expression Syntax
// ---------------------------------------------------------------------------

func TestExpressionSyntaxRule_EmptyExpression(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{
				ID: "http-1", Name: "Fetch", Type: models.NodeTypeComponent,
				Ref:           models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
				Configuration: map[string]any{"url": "https://api.example.com/{{ }}"},
			},
		},
	}

	rule := &ExpressionSyntaxRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for empty expression, got %d", len(issues))
	}
}

func TestExpressionSyntaxRule_UnbalancedBraces(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{
				ID: "http-1", Name: "Fetch", Type: models.NodeTypeComponent,
				Ref:           models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
				Configuration: map[string]any{"body": "Hello {{ event.name"},
			},
		},
	}

	rule := &ExpressionSyntaxRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for unbalanced braces, got %d", len(issues))
	}
}

func TestExpressionSyntaxRule_ValidExpression(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{
				ID: "http-1", Name: "Fetch", Type: models.NodeTypeComponent,
				Ref:           models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
				Configuration: map[string]any{"url": "https://api.example.com/{{ event.id }}"},
			},
		},
	}

	rule := &ExpressionSyntaxRule{}
	issues := rule.Run(spec)

	if len(issues) != 0 {
		t.Errorf("expected no issues for valid expression, got %d", len(issues))
	}
}

func TestExpressionSyntaxRule_NestedMapExpression(t *testing.T) {
	// Expression inside a nested config map should be found.
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{
				ID: "http-1", Name: "Fetch", Type: models.NodeTypeComponent,
				Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
				Configuration: map[string]any{
					"headers": map[string]any{
						"Authorization": "Bearer {{ }}",
					},
				},
			},
		},
	}

	rule := &ExpressionSyntaxRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for nested empty expression, got %d", len(issues))
	}
	if !containsStr(issues[0].Message, "headers.Authorization") {
		t.Errorf("expected field path to include nested key, got: %s", issues[0].Message)
	}
}

func TestExpressionSyntaxRule_ArrayExpression(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{
				ID: "http-1", Name: "Fetch", Type: models.NodeTypeComponent,
				Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}},
				Configuration: map[string]any{
					"tags": []any{"valid", "{{ }}"},
				},
			},
		},
	}

	rule := &ExpressionSyntaxRule{}
	issues := rule.Run(spec)

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for array expression, got %d", len(issues))
	}
}

// ---------------------------------------------------------------------------
// Integration: Full linter run
// ---------------------------------------------------------------------------

func TestLinter_MultipleIssues(t *testing.T) {
	spec := &CanvasSpec{
		Nodes: []models.Node{
			{ID: "trigger-1", Name: "Start", Type: models.NodeTypeTrigger, Ref: models.NodeRef{Trigger: &models.TriggerRef{Name: "webhook"}}},
			{ID: "orphan-1", Name: "Orphan", Type: models.NodeTypeComponent, Ref: models.NodeRef{Component: &models.ComponentRef{Name: "http"}}},
			{
				ID: "delete-1", Name: "Delete", Type: models.NodeTypeComponent,
				Ref:           models.NodeRef{Component: &models.ComponentRef{Name: "aws.deleteInstance"}},
				Configuration: map[string]any{"region": "{{ }}"},
			},
		},
		Edges: []models.Edge{
			{SourceID: "trigger-1", TargetID: "delete-1", Channel: "default"},
		},
	}

	l := New()
	result := l.Lint(spec)

	if result.Pass {
		t.Fatal("expected lint to fail with multiple issues")
	}

	ruleHits := map[string]bool{}
	for _, issue := range result.Issues {
		ruleHits[issue.Rule] = true
	}

	for _, expected := range []string{"orphan-node", "approval-before-destructive", "expression-syntax"} {
		if !ruleHits[expected] {
			t.Errorf("expected rule %q to fire, but it did not. Got issues: %v", expected, result.Issues)
		}
	}
}

// ---------------------------------------------------------------------------
// Fixture-based tests — load actual canvas JSON files
// ---------------------------------------------------------------------------

func TestFixture_ValidCopilotCanvas_Passes(t *testing.T) {
	path := filepath.Join(repoRoot(), "docs", "incident-copilot-canvas.json")
	l := New()
	result, err := l.LintFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}
	if !result.Pass {
		t.Errorf("expected valid copilot canvas to pass, got %d issues:", len(result.Issues))
		for _, issue := range result.Issues {
			t.Errorf("  [%s] %s: %s", issue.Severity, issue.Rule, issue.Message)
		}
	}
}

func TestFixture_BrokenCopilotCanvas_Fails(t *testing.T) {
	path := filepath.Join(repoRoot(), "docs", "incident-copilot-canvas-broken.json")
	l := New()
	result, err := l.LintFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}
	if result.Pass {
		t.Fatal("expected broken copilot canvas to fail")
	}

	ruleHits := map[string]bool{}
	for _, issue := range result.Issues {
		ruleHits[issue.Rule] = true
	}

	// The broken canvas should trigger all 3 demo scenarios.
	for _, expected := range []string{"orphan-node", "approval-before-destructive", "expression-syntax"} {
		if !ruleHits[expected] {
			t.Errorf("expected rule %q to fire on broken fixture, got: %v", expected, result.Issues)
		}
	}
}

func TestFixture_NonexistentFile_ReturnsError(t *testing.T) {
	l := New()
	_, err := l.LintFile("/nonexistent/canvas.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
