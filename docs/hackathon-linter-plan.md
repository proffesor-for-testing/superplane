# Track B — Workflow Linter: Implementation, Integration & Verification Plan

**Owner:** Braca | **Date:** 2026-03-28 | **Event:** Hackathon, Novi Sad

---

## Goal

A `pkg/linter` Go package that performs deep semantic validation of any SuperPlane canvas, exposed via a new `LintCanvas` gRPC endpoint. Runs on-demand, returns structured pass/fail with issues categorised by severity.

**Approach:** Option A — Go backend package. Gives access to the component registry, config schemas, and the full graph model.

---

## What Already Exists (don't duplicate)

All of this lives in `pkg/grpc/actions/canvases/serialization.go:197-326` and runs at **save time**:

- Unique node IDs, non-empty names
- Component/trigger references exist in registry
- Edge source/target nodes exist
- Widget nodes not used as edge endpoints
- Nested group validation
- Cycle detection (`CheckForCycles`)
- Basic required-field validation via `configuration.ValidateConfiguration`

The linter adds **semantic depth** on top — it does not re-run the above.

---

## Data Model Quick Reference

```go
// pkg/models/blueprint.go
type Node struct {
    ID            string
    Name          string
    Type          string         // "trigger" | "component" | "blueprint" | "widget"
    Ref           NodeRef
    Configuration map[string]any
}
type NodeRef struct {
    Component *ComponentRef  // .Name e.g. "if", "approval", "http"
    Trigger   *TriggerRef    // .Name e.g. "pagerduty.onIncident"
    Blueprint *BlueprintRef
    Widget    *WidgetRef
}
type Edge struct {
    SourceID string
    TargetID string
    Channel  string  // "default", "true", "false", "approved", "rejected", ...
}

// Loading live spec — one call does it all:
// models.FindLiveCanvasSpecInTransaction(tx, canvasID) → ([]Node, []Edge, error)
```

**Key component names** (verified from source):

| Component | `Name()` / registry key |
|-----------|------------------------|
| If / branch | `"if"` |
| Approval | `"approval"` |
| Merge | `"merge"` |
| HTTP request | `"http"` |
| SSH | `"ssh"` |
| Send email | `"send_email"` |
| If output channels | `"true"`, `"false"` |
| Approval output channels | `"approved"`, `"rejected"` |

---

## Phase 1 — Core Linter Package

### 1.1 `pkg/linter/linter.go` — Types and entry point

- [ ] Define types:

```go
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
    Status   string    `json:"status"`   // "pass" = zero errors; "fail" = one or more errors
    Errors   []Issue   `json:"errors"`
    Warnings []Issue   `json:"warnings"`
    Info     []Issue   `json:"info"`
    Summary  Summary   `json:"summary"`
}
```

- [ ] Main entry point:

```go
func LintCanvas(nodes []models.Node, edges []models.Edge, reg *registry.Registry) *LintResult
```

- [ ] `LintCanvas` calls each rule function and collects issues into the result
- [ ] `status` is `"fail"` if `len(result.Errors) > 0`, otherwise `"pass"` — warnings and info never fail
- [ ] Helper methods: `(r *LintResult) addError(...)`, `addWarning(...)`, `addInfo(...)`

---

### 1.2 `pkg/linter/helpers.go` — Graph utilities

- [ ] `buildOutgoing(edges []models.Edge) map[string][]models.Edge`
- [ ] `buildIncoming(edges []models.Edge) map[string][]models.Edge`
- [ ] `bfsForward(startIDs []string, outgoing map[string][]models.Edge) map[string]bool`
- [ ] `bfsBackward(startID string, incoming map[string][]models.Edge) map[string]bool`
- [ ] `triggerIDs(nodes []models.Node) []string` — nodes where `node.Ref.Trigger != nil`
- [ ] `nodeNameSet(nodes []models.Node) map[string]bool` — name → exists

---

### 1.3 `pkg/linter/rules.go` — All lint rules

#### Rule `orphan-node` · severity: **error**

Nodes not reachable from any trigger.

```
Algorithm:
1. Collect all trigger node IDs (node.Ref.Trigger != nil)
2. BFS forward over edges from those roots → reachable set
3. Any node with Type != "widget" and not in reachable set → orphan
```

- [ ] Widgets (groups) are skipped — they are visual containers, not flow nodes
- [ ] A canvas with no triggers at all → every non-widget node is an orphan

---

#### Rule `missing-approval-gate` · severity: **warning**

Destructive component with no `approval` node upstream.

- [ ] Destructive component list:
  ```go
  var destructiveComponents = []string{
      "pagerduty.resolveIncident",
      "pagerduty.escalateIncident",
      "github.deleteRelease",
      "github.createRelease",
      "ssh",
  }
  ```
- [ ] For HTTP nodes: also check if `node.Configuration["method"]` is `"DELETE"` or `"PUT"`
- [ ] Algorithm: reverse BFS from the destructive node; if no ancestor has `node.Ref.Component.Name == "approval"` → warn

---

#### Rule `unreachable-branch` · severity: **warning**

An `if` node with one branch unwired.

- [ ] Find nodes where `node.Ref.Component.Name == "if"`
- [ ] Check outgoing edges for channels `"true"` and `"false"` (confirmed channel names)
- [ ] If either channel has zero outgoing edges → warn, naming which channel

---

#### Rule `dead-end-node` · severity: **warning**

A component node that has defined output channels but none are wired.

- [ ] For each component node, call `reg.GetComponent(name)` → `comp.OutputChannels(node.Configuration)`
- [ ] If `OutputChannels` returns a non-empty list but no outgoing edges exist at all → warn
- [ ] This is self-maintaining: terminal components (approval, send_email) that have no output channels defined will not be flagged

---

#### Rule `single-input-merge` · severity: **info**

A merge node with fewer than 2 incoming edges is pointless.

- [ ] Find nodes where `node.Ref.Component.Name == "merge"`
- [ ] Count incoming edges in the incoming map
- [ ] If count < 2 → info

---

#### Rule `empty-expression-field` · severity: **error**

Semantic config checks that `ValidateConfiguration` does not cover:

- [ ] Node where component is `"if"`: check `node.Configuration["expression"]` is non-empty string
- [ ] Node with a `claude.*` component: check `node.Configuration["prompt"]` is non-empty
- [ ] Node where component is `"http"`: check `node.Configuration["url"]` is non-empty
- [ ] Node with `slack.sendTextMessage`: check `node.Configuration["channel"]` is non-empty

> Note: do NOT call `configuration.ValidateConfiguration` here — serialization.go already does that at save time. Only add the semantic checks above.

---

#### Rule `invalid-node-reference` · severity: **warning**

Expression strings referencing non-existent node names.

- [ ] Build `nodeNameSet` from all node names in the canvas
- [ ] Walk all string values in `node.Configuration` recursively
- [ ] For each value containing `$['...']` pattern: extract the name inside quotes via regex `\$\['([^']+)'\]`
- [ ] If the extracted name is not in `nodeNameSet` → warn

---

#### Rule `unbalanced-braces` · severity: **error**

Malformed expression delimiters.

- [ ] For each string config value, tokenize: scan character by character tracking open/close `{{` `}}`
- [ ] If opens ≠ closes at end of string → error
- [ ] Report the field key in the message

---

### 1.4 `pkg/linter/linter_test.go` — Unit tests

All tests use inline fixtures (`[]models.Node` + `[]models.Edge` literals) — no database.

- [ ] Orphan: isolated component node → `orphan-node` error
- [ ] No orphan: trigger → component (connected) → clean
- [ ] Missing approval: `github.deleteRelease` with no upstream `approval` → warning
- [ ] Approval present: `approval` → `github.deleteRelease` → no warning
- [ ] If branch: `if` node with only `"true"` wired → `unreachable-branch` warning for `"false"`
- [ ] Merge: merge node with 1 incoming edge → `single-input-merge` info
- [ ] Empty expression: `if` node with empty `"expression"` config → `empty-expression-field` error
- [ ] Invalid node ref: expression `$['GhostNode'].value` → `invalid-node-reference` warning
- [ ] Unbalanced: config value `{{ foo }` (one close brace missing) → `unbalanced-braces` error
- [ ] Status: canvas with only warnings → `status == "pass"`
- [ ] Status: canvas with one error → `status == "fail"`
- [ ] Empty canvas (no nodes) → clean pass, no panic

---

## Phase 2 — gRPC Integration

### 2.1 `protos/canvases.proto` — Add RPC

- [ ] Add to the `Canvases` service:

```protobuf
rpc LintCanvas(LintCanvasRequest) returns (LintCanvasResponse);
```

- [ ] Add messages:

```protobuf
message LintCanvasRequest {
  string canvas_id = 1;
}

message LintIssue {
  string severity  = 1;
  string rule      = 2;
  string node_id   = 3;
  string node_name = 4;
  string message   = 5;
}

message LintCanvasSummary {
  int32 total    = 1;
  int32 errors   = 2;
  int32 warnings = 3;
  int32 info     = 4;
}

message LintCanvasResponse {
  string                 status   = 1;
  repeated LintIssue     errors   = 2;
  repeated LintIssue     warnings = 3;
  repeated LintIssue     info     = 4;
  LintCanvasSummary      summary  = 5;
}
```

- [ ] Run `make pb.gen`

---

### 2.2 `pkg/grpc/actions/canvases/lint_canvas.go` — Action function

```go
func LintCanvas(
    ctx context.Context,
    reg *registry.Registry,
    organizationID string,
    canvasID string,
) (*pb.LintCanvasResponse, error)
```

Implementation:
1. Parse and validate `canvasID` UUID
2. `models.FindCanvas(uuid.MustParse(organizationID), canvasID)` — verify ownership
3. `models.FindLiveCanvasSpecInTransaction(database.Conn(), canvasID)` → `nodes, edges`
   - If canvas has no live version yet → return clean pass (nothing to lint)
4. `result := linter.LintCanvas(nodes, edges, reg)`
5. Map `result` to proto response and return

---

### 2.3 `pkg/grpc/canvas_service.go` — Add method to CanvasService

- [ ] Add one method (same pattern as all existing methods in that file):

```go
func (s *CanvasService) LintCanvas(ctx context.Context, req *pb.LintCanvasRequest) (*pb.LintCanvasResponse, error) {
    organizationID := ctx.Value(authorization.OrganizationContextKey).(string)
    return canvases.LintCanvas(ctx, s.registry, organizationID, req.CanvasId)
}
```

No other registration needed — the generated interface is automatically satisfied.

---

### 2.4 `pkg/authorization/interceptor.go` — Add auth rule

- [ ] Add one entry in `NewAuthorizationInterceptor`, following the exact same pattern as `DescribeCanvas`:

```go
pbCanvases.Canvases_LintCanvas_FullMethodName: {
    Resource:         "canvases",
    Action:           "read",
    DomainType:       models.DomainTypeOrganization,
    ResourceResolver: canvasResourceResolver,
},
```

---

## Phase 3 — Demo Wiring

### 3.1 Verify build

- [ ] `make format.go`
- [ ] `make lint && make check.build.app`
- [ ] `make test PKG_TEST_PACKAGES=./pkg/linter`

### 3.2 Smoke test via grpcurl

```bash
# Green — fully wired copilot canvas
grpcurl -plaintext \
  -d '{"canvas_id": "<copilot_canvas_id>"}' \
  localhost:9000 \
  superplane.v1.Canvases/LintCanvas

# Red — disconnect one node from trigger first, then re-run
# Expected: status=fail, errors=[{rule:"orphan-node",...}]

# Yellow — remove the approval gate edge, re-run
# Expected: status=pass (warning only), warnings=[{rule:"missing-approval-gate",...}]

# Green again — restore → re-run → clean
```

### 3.3 Demo sequence (for Fedja)

| Step | Action | Expected output |
|------|--------|-----------------|
| 1 | Run linter on copilot canvas as-built | `status: pass` — green |
| 2 | Delete edge from trigger to fan-out node | `status: fail` — orphan-node error |
| 3 | Restore edge, delete approval gate edge | `status: pass` — missing-approval-gate warning |
| 4 | Restore approval gate | `status: pass` — clean |

### 3.4 Optional UI badge (Fedja, stretch goal)

- Call `LintCanvas` when canvas is opened or saved
- Show green / red badge in canvas header
- Clicking badge opens issue list panel

---

## Phase 4 — Verification Checklist

### Must pass before demo

- [ ] `make test PKG_TEST_PACKAGES=./pkg/linter` — all unit tests green
- [ ] `make lint` — zero lint errors
- [ ] `make check.build.app` — clean build
- [ ] All 4 demo steps above work end-to-end

### Edge cases verified by unit tests

- [ ] Empty canvas (no nodes, no edges) → `status: pass`, no panic
- [ ] Canvas with only trigger and no downstream → trigger has zero outgoing, no false orphan
- [ ] Widget/group nodes never flagged as orphans
- [ ] Multiple triggers → all treated as BFS roots
- [ ] Warnings-only canvas → `status: pass`
- [ ] Single error → `status: fail`

---

## File Map

| File | What it contains |
|------|-----------------|
| `pkg/linter/linter.go` | `LintResult`, `Issue`, `Summary` types; `LintCanvas()` entry point |
| `pkg/linter/helpers.go` | BFS forward/backward, incoming/outgoing maps, trigger/name index builders |
| `pkg/linter/rules.go` | All 7 rules: orphan, missing-approval, unreachable-branch, dead-end, single-input-merge, empty-expression-field, invalid-node-reference, unbalanced-braces |
| `pkg/linter/linter_test.go` | Fixture-based unit tests, no DB |
| `protos/canvases.proto` | `LintCanvas` RPC + 3 new messages |
| `pkg/grpc/actions/canvases/lint_canvas.go` | Action function: load live spec → call linter → map to proto |
| `pkg/grpc/canvas_service.go` | Add `LintCanvas` method to `CanvasService` |
| `pkg/authorization/interceptor.go` | Add read rule for `LintCanvas` |

**Total new files: 4** | **Modified files: 3**

---

## MVP Priority (if time runs short)

Implement in this order — stop when time forces it:

1. `orphan-node` rule + `LintCanvas()` types + unit test → **minimum for demo**
2. gRPC endpoint wired end-to-end → **can call it via grpcurl**
3. `missing-approval-gate` rule → **the money-shot demo moment**
4. `unreachable-branch`, `dead-end-node`, `single-input-merge`
5. `empty-expression-field`, `invalid-node-reference`, `unbalanced-braces`
6. UI badge (Fedja)

Steps 1–3 are sufficient for a compelling demo.
