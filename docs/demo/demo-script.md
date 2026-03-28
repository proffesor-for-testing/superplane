# Hackathon Demo Script — Professorianci
## Incident Copilot + Workflow Linter

**Team:** Dragan · Braca · Fedja
**Duration:** 3 minutes
**Date:** March 28, 2026 — Novi Sad

---

## Setup Checklist (Before Demo)

- [ ] **Choose your copilot version:**
  - **Option A (Full)**: `incident-copilot.yaml` — Requires Datadog, GitHub, K8s, Slack, Claude APIs
  - **Option B (Simple)**: `incident-copilot-simple.yaml` — Only requires Claude + Slack (DEMO-SAFE)
- [ ] Canvas is imported and live in SuperPlane
  - Find canvas ID: `psql -d superplane -c "SELECT id, name FROM workflows WHERE name LIKE 'Incident Copilot%';"`
- [ ] Slack integration connected to `#incidents` channel (or demo channel)
- [ ] Claude integration connected (required for both versions)
- [ ] For full version only: Datadog, GitHub, and K8s integrations configured
- [x] Mock data files ready in `docs/demo/`:
  - [x] `mock-incident-payload.json` — PagerDuty webhook payload
  - [x] `mock-github-release.json` — GitHub release API response
  - [x] `mock-datadog-metrics.json` — Datadog timeseries response
  - [x] `mock-datadog-logs.json` — Datadog logs response
  - [x] `mock-k8s-pods.json` — Kubernetes pod list response
- [ ] Browser tab open on the Incident Copilot canvas
- [ ] Terminal ready for `grpcurl` linter call
- [ ] Second browser tab on linter results

**RECOMMENDATION:** Use **Option B (Simple)** for the demo. The full version requires 5 external APIs that may not be available in the demo environment. The simple version (trigger → Claude → Slack) demonstrates the core value without infrastructure dependencies.

---

## Slide 1 — The Problem (30 seconds)

> "It's 3am. PagerDuty fires. Your engineer wakes up, spends 20 minutes
> across 5 dashboards gathering context before they even understand what
> is happening. They check GitHub for recent deploys. They check Datadog
> for metrics. They grep logs. They check pod status in Kubernetes.
> Then they write a Slack message to the team. By the time they have
> the full picture — 20 minutes gone, customer already hurting."

**Show:** The blank canvas before it runs.

---

## Live Demo 1 — The Incident Copilot (60 seconds)

**Step 1 — Show the canvas**
> "We built this canvas in SuperPlane. It is an automated incident
> responder. One PagerDuty trigger fires all of this."

Walk through the nodes visually:
1. PagerDuty trigger → filter (P1/P2 only)
2. Fan-out: GitHub, Datadog metrics, Datadog logs, Kubernetes pods — all in parallel
3. Merge — waits for all four data sources
4. Claude AI triage — receives everything, produces an evidence pack
5. Slack — posts structured report to #incidents
6. Approval gate — human in the loop before any action
7. Acknowledge incident in PagerDuty — only after approval

**Step 2 — Trigger it**

Emit the mock incident event using the PagerDuty trigger node:
```json
{
  "incident": {
    "id": "Q2R4X8Z6Y5",
    "title": "[P1] Error rate on production-api exceeded 5%",
    "priority": { "summary": "P1" },
    "status": "triggered",
    "html_url": "https://superplane-demo.pagerduty.com/incidents/Q2R4X8Z6Y5"
  }
}
```

**Step 3 — Watch it run**
> "Watch the nodes light up as data flows in — GitHub release, Datadog
> metrics, logs, pod status — all collected in under 30 seconds."

**Step 4 — Show the Slack output**

Switch to the Slack channel. Show the evidence pack that arrived:
> "This arrived in Slack while the engineer was still rubbing their eyes.
> Severity assessment. Blast radius. Root cause hypothesis. Recommended
> actions. What would have taken 20 minutes, done in 45 seconds."

---

## Live Demo 2 — The Safety Net (60 seconds)

> "But how do we know this workflow is safe before it goes live?
> That is Track B — the Workflow Linter."

**Step 5 — Run the linter (canvas is green)**

```bash
grpcurl -plaintext \
  -d '{"canvas_id": "<incident-copilot-canvas-id>"}' \
  localhost:9000 \
  superplane.v1.Canvases/LintCanvas
```

Expected output:
```json
{
  "status": "pass",
  "errors": [],
  "warnings": [],
  "info": [],
  "summary": { "total": 0, "errors": 0, "warnings": 0, "info": 0 }
}
```

> "Green. The linter checked: no orphan nodes, approval gate is present
> before the destructive action, all expressions are valid, no broken
> references. Safe to go live."

**Step 6 — Break something (disconnect the trigger)**

In the canvas UI:
1. Delete the edge from "New Incident" to "P1 or P2 only"
2. Save the canvas

**Step 7 — Run the linter again (now red)**

```bash
grpcurl -plaintext \
  -d '{"canvas_id": "<incident-copilot-canvas-id>"}' \
  localhost:9000 \
  superplane.v1.Canvases/LintCanvas
```

Expected output:
```json
{
  "status": "fail",
  "errors": [
    {
      "severity": "error",
      "rule": "orphan-node",
      "nodeId": "filter-priority",
      "nodeName": "P1 or P2 only",
      "message": "Node \"P1 or P2 only\" is not reachable from any trigger"
    }
  ],
  "summary": { "total": 1, "errors": 1, "warnings": 0, "info": 0 }
}
```

> "Red. Caught immediately. Six downstream nodes are now unreachable.
> Before this workflow could run in production, the linter blocked it."

**Step 8 — Remove the approval gate (yellow)**

Restore the trigger edge. Then delete the edge from "Post Evidence Pack"
to "Approve Remediation". Save.

```bash
grpcurl -plaintext \
  -d '{"canvas_id": "<incident-copilot-canvas-id>"}' \
  localhost:9000 \
  superplane.v1.Canvases/LintCanvas
```

Expected output:
```json
{
  "status": "pass",
  "warnings": [
    {
      "severity": "warning",
      "rule": "missing-approval-gate",
      "nodeId": "acknowledge-incident",
      "nodeName": "Acknowledge Incident",
      "message": "Destructive action \"pagerduty.acknowledgeIncident\" has no upstream approval gate"
    }
  ]
}
```

> "Yellow. Status is still pass — warnings don't block — but the linter
> is telling us: there is now no human in the loop before the incident
> is auto-acknowledged. This is the kind of issue that causes 3am
> surprises of a different kind."

**Step 9 — Restore to green**
Restore the approval gate edge → linter passes clean.

---

## Slide 2 — What We Built (30 seconds)

| | |
|---|---|
| **Incident Copilot** | AI triage in < 60 seconds vs 20 minutes manual. Two versions: full (7 nodes) and simple (3 nodes). Zero new backend code. |
| **Workflow Linter** | 9 lint rules (including cycle detection), gRPC endpoint, catches errors before they hit production. 42 unit tests, 12 integration tests. |
| **Together** | We built the feature AND the safety net. |

---

## Slide 3 — What's Next (15 seconds)

- Linter as a pre-publish hook (block deploy if linter fails)
- UI badge: green/red linter status on every canvas
- Copilot templates for common incident types (database, API, deployment)
- Self-healing: Claude suggests workflow fixes when linter finds issues

---

## Backup Plan

### Option A: Use Simple Copilot (RECOMMENDED)
- Import `templates/canvases/incident-copilot-simple.yaml` instead of the full version
- This is a 3-node canvas: trigger → Claude → Slack
- **No external APIs required** - only needs Claude + Slack integrations
- Demonstrates the same core value (AI triage in < 60s) without infrastructure risk

### Option B: Linter-Only Demo
If copilot demo is not possible:
- Focus entirely on the linter (Demo 2) - this is fully functional and demo-ready
- Show the canvas design and explain the data flow visually
- Run the grpcurl linter commands (these work without the full stack)
- Show the mock payload JSON files and explain what the copilot would have received

### Option C: Static Demo
If grpcurl is unavailable:
- Show `go test ./pkg/linter/... -v` running with all 42 tests passing
- Explain the 9 lint rules verbally with the canvas as visual aid
- Show the badge component code in `Header.tsx` and explain the three-state UI

### Running Tests
```bash
# Linter unit tests (no DB required)
go test ./pkg/linter/... -v

# Integration tests (requires DB)
# go test ./pkg/grpc/actions/canvases/... -run Test__LintCanvas -v
```

All 42 linter unit tests pass. Integration tests require database setup.
