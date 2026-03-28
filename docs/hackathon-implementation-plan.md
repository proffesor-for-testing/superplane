# Hackathon Implementation Plan — Professorianci

**Team:** Dragan, Braca, Fedja
**Date:** March 28, 2026 — Novi Sad
**Tracking:** Check off tasks as completed during the hackathon

---

## Phase 0 — Setup & Verification (0:00–0:15)

- [ ] **ALL:** `make dev.setup && make dev.start` — confirm local env is running
- [ ] **ALL:** Confirm Canvas UI loads in browser
- [ ] **ALL:** Agree on a Slack channel for demo output (everyone joins it)
- [ ] **ALL:** Confirm each person can post to the chosen Slack channel

> **Done when:** All three have a running local env, Canvas UI is accessible, and a shared Slack channel is confirmed.

---

## Phase 1 — Design & Prep (0:15–0:30)

### Track A — Incident Copilot (Dragan)
- [x] Review Canvas API: how to create trigger nodes, HTTP components, AI components
- [x] Review available integration components (PagerDuty, GitHub, Datadog, Slack)
  - Native: `pagerduty.OnIncident`, `slack.SendTextMessage`, `github.ListReleases`, `approval`, `merge`, `http`
  - No native Datadog query or K8s pod status — use `http` component
- [x] Sketch the canvas flow → `docs/incident-copilot-canvas.json` (9-node full flow) + `docs/incident-copilot-canvas-simple.json` (4-node fallback)
- [x] Identify which components exist vs need HTTP workarounds → see `docs/demo/canvas-setup-guide.md`

### Track B — Workflow Linter (Braca)
- [x] Study canvas data model: `protos/canvases.proto`, `/pkg/models/` canvas structs
- [x] Review `docs/contributing/canvas-change-requests.md` for data model conventions
- [x] Understand how nodes and edges are stored (graph structure)
- [x] Identify the node/edge types and their required fields
- [x] Decide: implement in Go (closer to codebase) or TypeScript (faster iteration) → **Go chosen**
- [x] List linter rules to implement (priority order):
  1. Orphan node detection (nodes with no edges)
  2. Missing required configuration on nodes
  3. Approval gate before destructive actions
  4. Cycle detection in non-loop paths
  5. Expression syntax validation

### Track C — Demo & Glue (Fedja)
- [x] Prepare mock payloads as static JSON files:
  - `docs/mock-incident.json` — PagerDuty incident trigger payload
  - `docs/mock-github-release.json` — GitHub deploy data
  - `docs/mock-datadog-metrics.json` — Datadog metrics response
  - `docs/mock-pd-logs.json` — PagerDuty log entries
- [x] Draft Slack evidence pack template (Block Kit) → `docs/slack-evidence-pack.json`
- [x] Prepare the `curl` command for triggering the webhook → `docs/demo/run-demo.sh act2`

> **Done when:** Dragan has a canvas flow sketch, Braca understands the graph data model and has chosen Go or TS, Fedja has all 4 mock JSON files saved.

---

## Phase 2 — Core Build (0:30–1:30)

### Track A — Incident Copilot (Dragan)
- [ ] Create new canvas in SuperPlane UI
- [ ] Add webhook trigger node, configure to accept PagerDuty payload
- [ ] Add parallel fan-out with 4 branches:
  - [ ] Branch 1: HTTP node → GitHub recent deploys (mock or real)
  - [ ] Branch 2: HTTP node → Datadog metrics (mock or real)
  - [ ] Branch 3: HTTP node → PagerDuty logs (mock or real)
  - [ ] Branch 4: HTTP node → Pod/service status (mock or real)
- [ ] Add merge/join node to collect all fan-out results
- [ ] Add AI (Claude) component node:
  - [x] Write triage prompt → `docs/demo/ai-triage-prompt.md`
  - [x] Define output schema: severity, root cause ranking, affected systems, recommended actions → see prompt doc
- [ ] Add approval gate node before any remediation action
- [ ] Add Slack output node with evidence pack template
- [ ] Wire all edges between nodes

### Track B — Workflow Linter (Braca)
- [x] Set up linter project structure (file/package) → `pkg/linter/`
- [x] Implement canvas graph loader (read canvas definition into in-memory graph) → `pkg/linter/graph.go`
- [x] Implement Rule 1: **Orphan node detection**
  - Detects fully orphaned nodes (no edges) AND unreachable nodes (outgoing only, no incoming)
  - Detects dangling edges referencing non-existent node IDs
  - Return: list of orphan/unreachable node IDs + names
- [x] Implement Rule 2: **Missing node reference** (descoped from "missing required config" — full config validation requires component registry integration which is out of scope for hackathon; checks that each node type has the correct ref set)
  - Return: list of nodes with missing/empty refs or unknown types
- [x] Implement Rule 3: **Cycle detection**
  - DFS-based cycle detection — finds ALL cycles including self-loops
  - Reports full cycle path (e.g., "a → b → c → a")
  - Return: list of cycles found
- [x] Implement linter runner: takes canvas ID/definition, runs all rules, returns pass/fail + issues list
- [x] Write unit tests for each rule (30 tests, all passing — includes fixture-based, edge cases, self-loops)

### Track C — Demo & Glue (Fedja)
- [x] Create static mock API server → `cmd/mockserver/main.go` (GitHub, Datadog, PagerDuty, K8s on port 9999)
- [x] Build Slack message formatting using Block Kit → `docs/slack-evidence-pack.json`:
  - Rotating light + incident title header
  - Priority / Service / Assignee metadata
  - Severity assessment with customer impact
  - Ranked root causes with confidence percentages
  - Affected systems list
  - Numbered recommended actions with inline commands
  - Escalation path (current + next)
  - Timing footer ("Triage generated in X seconds")
  - PagerDuty deep link
  - Approve/Reject/View Canvas action buttons
- [ ] Test Slack message formatting in the agreed channel
- [x] Help Dragan with AI prompt tuning → prompt written at `docs/demo/ai-triage-prompt.md`
- [x] Help Braca with canvas data model questions if needed

> **Done when:** Copilot canvas executes end-to-end with mock data and posts to Slack. Linter catches orphan nodes on a test fixture. Slack evidence pack renders correctly.

---

## Phase 3 — Integration & Polish (1:30–2:15)

### Track A — Incident Copilot (Dragan)
- [x] Tune AI triage prompt for quality output → `docs/demo/ai-triage-prompt.md`
  - Produces structured JSON severity with P1/P2/P3
  - Ranks root causes with confidence %
  - Gives actionable recommendations with CLI commands and ETAs
- [ ] Test full end-to-end flow: webhook → fan-out → AI → approval → Slack *(needs running SuperPlane)*
- [ ] Iterate on prompt based on output quality *(needs live AI calls)*

### Track B — Workflow Linter (Braca)
- [x] Implement Rule 4: **Approval gate check**
  - Detect destructive patterns: delete, destroy, terminate, remove, drop, rollback, restart, kill, shutdown, purge, truncate
  - Verify an approval node exists upstream of each destructive node
  - Return: list of unguarded destructive actions
- [x] Implement Rule 5: **Expression syntax validation**
  - Recursively scans nested maps and arrays in configuration
  - Checks for empty `{{}}` and unbalanced braces with full field path reporting
  - Return: list of invalid expressions
- [x] Run linter against the Incident Copilot canvas (**eat our own dogfood**) → PASS
- [x] Fix any issues the linter finds in the copilot canvas → None found

### Track C — Demo & Glue (Fedja)
- [x] Integrate linter output into demo flow → `docs/demo/run-demo.sh act3`
- [ ] Verify end-to-end: trigger → copilot runs → Slack message arrives
- [ ] Time the full flow (target: under 60 seconds)
- [x] Prepare "break the canvas" scenarios for linter demo → `docs/incident-copilot-canvas-broken.json`:
  - [x] Scenario 1: Delete an edge → linter catches orphan node
  - [x] Scenario 2: Remove approval gate → linter catches unguarded destructive action
  - [x] Scenario 3: Add bad expression → linter catches syntax error
- [ ] **Stretch goal:** Green/red linter badge on canvas UI showing pass/fail status (requires `web_src/` changes)

> **Done when:** AI triage output is high quality. Linter has all 5 rules and passes against the copilot canvas. All 3 "break the canvas" demo scenarios work. Full flow completes in under 60 seconds.

---

## Phase 4 — Demo Prep (2:15–2:45)

### Capture & Polish (ALL)
- [ ] **Screenshot:** Full canvas view with all nodes connected
- [ ] **Screenshot:** Canvas with nodes executing (green highlights)
- [ ] **Screenshot:** Slack evidence pack message
- [x] **Screenshot:** Linter output — passing (green) → `docs/demo/screenshots/linter-pass.txt`
- [x] **Screenshot:** Linter output — failing (red) → `docs/demo/screenshots/linter-fail.txt`
- [ ] **Screenshot:** Before/after comparison *(needs canvas UI screenshots)*

### Demo Script Rehearsal (ALL)

| Act | Duration | Presenter | Content |
|-----|----------|-----------|---------|
| Act 1: The Problem | 30s | **Fedja** | "3am PagerDuty" narrative — set the scene |
| Act 2: The Copilot | 90s | **Dragan** | Walk through canvas, fire webhook, show Slack output |
| Act 3: The Safety Net | 60s | **Braca** | Run linter pass, break canvas, show linter catching errors |
| Act 4: What's Next | 30s | **Fedja** | Future vision — pre-publish hooks, templates, self-healing |

- [ ] Rehearse full demo at least once end-to-end
- [ ] Dry run the `curl` trigger → Slack output flow
- [x] Prepare backup plan → `docs/incident-copilot-canvas-simple.json` (4-node simplified flow, passes linter)

### Slides (Fedja)
- [x] Slide 1: The Problem — "3am, 5 dashboards, 20 minutes" → `docs/demo/slides.html`
- [x] Slide 2: What We Built — Copilot + Linter summary stats → `docs/demo/slides.html`
- [x] Slide 3: What's Next — pre-publish hooks, templates, self-healing → `docs/demo/slides.html`

> **Done when:** All screenshots captured. Full demo rehearsed at least once. Slides ready. Backup plan tested.

---

## Phase 5 — Present (2:45–3:00)

- [ ] **Present!**
- [ ] Have backup screenshots ready in case of live demo failure

---

## Fallback Plan (if things go wrong)

| Scenario | Fallback |
|----------|----------|
| Full fan-out doesn't work | Simplify to: Trigger → AI Triage → Slack (skip parallel branches) |
| AI output is poor quality | Use hardcoded triage output for demo, show prompt engineering |
| Linter can't read canvas model | Demo linter with hardcoded canvas JSON fixture |
| Slack integration fails | Show output in terminal/logs instead |
| Everything breaks | Linter standalone demo + Copilot design walkthrough with slides |

---

## Key Files & Paths

| What | Where |
|------|-------|
| Canvas protobuf definitions | `protos/canvases.proto` |
| Canvas models (Go) | `pkg/models/` |
| Components | `pkg/components/` |
| Expression runtime | `pkg/exprruntime/` |
| Canvas data model conventions | `docs/contributing/canvas-change-requests.md` |
| Frontend source | `web_src/` |
| Python workflow utilities | `agent/evaluators/workflow_utils.py` |
| **Linter package** | `pkg/linter/` (`linter.go`, `graph.go`, `rules.go`, `linter_test.go`) |
| **Linter CLI** | `cmd/linter/main.go` → `bin/linter` |
| **Copilot canvas (valid)** | `docs/incident-copilot-canvas.json` |
| **Copilot canvas (broken, for demo)** | `docs/incident-copilot-canvas-broken.json` |
| **Mock API server** | `cmd/mockserver/main.go` → `bin/mockserver` (port 9999) |
| **Slack Block Kit template** | `docs/slack-evidence-pack.json` |
| **Demo runner script** | `docs/demo/run-demo.sh` (`act1`, `act2`, `act3`, `linter-pass`, `linter-fail`, `mock-server`, `build`, `all`) |
| **AI triage prompt** | `docs/demo/ai-triage-prompt.md` (system prompt, user template, HTTP config, output schema) |
| **Canvas setup guide** | `docs/demo/canvas-setup-guide.md` (step-by-step for Dragan) |
| **Presentation slides** | `docs/demo/slides.html` (6 slides, arrow-key navigation, zero dependencies) |
| **Simplified fallback canvas** | `docs/incident-copilot-canvas-simple.json` (4 nodes: trigger → AI → approval → Slack) |
| **Linter output captures** | `docs/demo/screenshots/linter-pass.txt`, `linter-fail.txt` |
| **Makefile targets** | `make hackathon.build`, `hackathon.test`, `hackathon.lint.pass`, `hackathon.lint.fail`, `hackathon.demo` |
| Mock payloads | `docs/mock-*.json` |
| Slack evidence pack template | `docs/hackathon-reference-fedja.md` (lines 172–206) |
| Fedja's full reference doc | `docs/hackathon-reference-fedja.md` |
| This plan | `docs/hackathon-implementation-plan.md` |
