package linter

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/superplanehq/superplane/pkg/models"
)

// Severity indicates how serious a lint issue is.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue represents a single lint finding.
type Issue struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	NodeID   string   `json:"node_id,omitempty"`
	Message  string   `json:"message"`
}

// Result is the output of running the linter against a canvas.
type Result struct {
	Pass   bool    `json:"pass"`
	Issues []Issue `json:"issues"`
}

// CanvasSpec is the JSON structure we lint against.
// It mirrors the canvas spec with nodes and edges.
type CanvasSpec struct {
	Nodes []models.Node `json:"nodes"`
	Edges []models.Edge `json:"edges"`
}

// Rule is the interface all lint rules implement.
type Rule interface {
	Name() string
	Run(spec *CanvasSpec) []Issue
}

// Linter holds the set of rules and runs them against a canvas spec.
type Linter struct {
	rules []Rule
}

// New creates a linter with all built-in rules enabled.
func New() *Linter {
	return &Linter{
		rules: []Rule{
			&OrphanNodeRule{},
			&MissingRefRule{},
			&CycleDetectionRule{},
			&ApprovalGateRule{},
			&ExpressionSyntaxRule{},
		},
	}
}

// Lint runs all rules against the given canvas spec.
func (l *Linter) Lint(spec *CanvasSpec) Result {
	var issues []Issue
	for _, rule := range l.rules {
		issues = append(issues, rule.Run(spec)...)
	}

	return Result{
		Pass:   len(issues) == 0,
		Issues: issues,
	}
}

// LintFile loads a canvas JSON file and lints it.
func (l *Linter) LintFile(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("reading canvas file: %w", err)
	}

	var spec CanvasSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return Result{}, fmt.Errorf("parsing canvas JSON: %w", err)
	}

	return l.Lint(&spec), nil
}
