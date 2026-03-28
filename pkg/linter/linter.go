package linter

import (
	"github.com/superplanehq/superplane/pkg/models"
	"github.com/superplanehq/superplane/pkg/registry"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type Issue struct {
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	NodeID   string   `json:"nodeId"`
	NodeName string   `json:"nodeName"`
	Message  string   `json:"message"`
}

type Summary struct {
	Total    int `json:"total"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Info     int `json:"info"`
}

type LintResult struct {
	// "pass" = zero errors; "fail" = one or more errors. Warnings and info never flip to fail.
	Status   string  `json:"status"`
	Errors   []Issue `json:"errors"`
	Warnings []Issue `json:"warnings"`
	Info     []Issue `json:"info"`
	Summary  Summary `json:"summary"`
}

func (r *LintResult) addError(rule, nodeID, nodeName, message string) {
	r.Errors = append(r.Errors, Issue{
		Severity: SeverityError,
		Rule:     rule,
		NodeID:   nodeID,
		NodeName: nodeName,
		Message:  message,
	})
}

func (r *LintResult) addWarning(rule, nodeID, nodeName, message string) {
	r.Warnings = append(r.Warnings, Issue{
		Severity: SeverityWarning,
		Rule:     rule,
		NodeID:   nodeID,
		NodeName: nodeName,
		Message:  message,
	})
}

func (r *LintResult) addInfo(rule, nodeID, nodeName, message string) {
	r.Info = append(r.Info, Issue{
		Severity: SeverityInfo,
		Rule:     rule,
		NodeID:   nodeID,
		NodeName: nodeName,
		Message:  message,
	})
}

func (r *LintResult) finalize() {
	total := len(r.Errors) + len(r.Warnings) + len(r.Info)
	r.Summary = Summary{
		Total:    total,
		Errors:   len(r.Errors),
		Warnings: len(r.Warnings),
		Info:     len(r.Info),
	}

	if len(r.Errors) > 0 {
		r.Status = "fail"
	} else {
		r.Status = "pass"
	}
}

// LintCanvas runs all lint rules against the provided nodes and edges.
// reg may be nil — rules that require registry access will be skipped gracefully.
func LintCanvas(nodes []models.Node, edges []models.Edge, reg *registry.Registry) *LintResult {
	result := &LintResult{
		Errors:   []Issue{},
		Warnings: []Issue{},
		Info:     []Issue{},
	}

	if len(nodes) == 0 {
		result.finalize()
		return result
	}

	out := buildOutgoing(edges)
	in := buildIncoming(edges)

	checkOrphanNodes(result, nodes, out)
	checkMissingApprovalGate(result, nodes, in)
	checkUnreachableBranch(result, nodes, out)
	checkDeadEndNodes(result, nodes, out, reg)
	checkSingleInputMerge(result, nodes, in)
	checkEmptyExpressionFields(result, nodes)
	checkInvalidNodeReferences(result, nodes)
	checkUnbalancedBraces(result, nodes)
	checkCycleDetected(result, nodes, out)

	result.finalize()
	return result
}
