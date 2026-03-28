#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# Incident Copilot + Workflow Linter — Demo Script
# Usage: ./docs/demo/run-demo.sh [act1|act2|act3|linter-pass|linter-fail|mock-server|build|all]
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
MOCK_PORT="${MOCK_PORT:-9999}"
WEBHOOK_URL="${WEBHOOK_URL:-http://localhost:8000/api/v1/webhooks/REPLACE_ME}"
MOCK_SERVER_PID=""

GREEN='\033[32m'
RED='\033[31m'
YELLOW='\033[33m'
CYAN='\033[36m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

banner() {
    echo ""
    echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════${RESET}"
    echo -e "${CYAN}${BOLD}  $1${RESET}"
    echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════${RESET}"
    echo ""
}

pause() {
    echo ""
    echo -e "${YELLOW}  Press ENTER to continue...${RESET}"
    read -r
}

# ---------- Build binaries ----------
build() {
    banner "BUILDING BINARIES"
    echo -e "  Building linter...   ${DIM}go build -o bin/linter ./cmd/linter/${RESET}"
    (cd "$ROOT_DIR" && go build -o bin/linter ./cmd/linter/)
    echo -e "  ${GREEN}✓${RESET} bin/linter"

    echo -e "  Building mockserver... ${DIM}go build -o bin/mockserver ./cmd/mockserver/${RESET}"
    (cd "$ROOT_DIR" && go build -o bin/mockserver ./cmd/mockserver/)
    echo -e "  ${GREEN}✓${RESET} bin/mockserver"
    echo ""
}

ensure_binaries() {
    if [ ! -f "$ROOT_DIR/bin/linter" ] || [ ! -f "$ROOT_DIR/bin/mockserver" ]; then
        echo -e "  ${YELLOW}Binaries not found — building...${RESET}"
        build
    fi
}

# ---------- Mock server management ----------
start_mock_server() {
    if curl -sf "http://localhost:${MOCK_PORT}/health" > /dev/null 2>&1; then
        return 0
    fi
    echo -e "  ${DIM}Starting mock API server on :${MOCK_PORT}...${RESET}"
    (cd "$ROOT_DIR" && "$ROOT_DIR/bin/mockserver" > /dev/null 2>&1 &)
    MOCK_SERVER_PID=$!
    sleep 0.5
    if curl -sf "http://localhost:${MOCK_PORT}/health" > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${RESET} Mock server running on :${MOCK_PORT}"
    else
        echo -e "  ${RED}✗${RESET} Mock server failed to start"
    fi
}

stop_mock_server() {
    if [ -n "$MOCK_SERVER_PID" ]; then
        kill "$MOCK_SERVER_PID" 2>/dev/null || true
    fi
}

trap stop_mock_server EXIT

# ---------- Act 1: The Problem ----------
act1() {
    banner "ACT 1: THE PROBLEM"
    echo -e "  ${BOLD}It's 3am. PagerDuty fires.${RESET}"
    echo ""
    echo "  Your engineer wakes up. Opens 5 tabs:"
    echo "    1. PagerDuty  — what's the alert?"
    echo "    2. Datadog    — what are the metrics doing?"
    echo "    3. GitHub     — was there a recent deploy?"
    echo "    4. Kubernetes — are pods healthy?"
    echo "    5. Slack      — has anyone else noticed?"
    echo ""
    echo -e "  ${RED}20 minutes later${RESET}, they finally understand the problem."
    echo ""
    echo -e "  ${GREEN}We fixed that.${RESET}"
    pause
}

# ---------- Act 2: The Copilot ----------
act2() {
    banner "ACT 2: THE INCIDENT COPILOT"
    echo "  Triggering the Incident Copilot with a mock PagerDuty alert..."
    echo ""
    echo -e "  ${CYAN}curl -X POST ${WEBHOOK_URL}${RESET}"
    echo -e "  ${CYAN}  -H 'Content-Type: application/json'${RESET}"
    echo -e "  ${CYAN}  -d @docs/mock-incident.json${RESET}"
    echo ""

    if [ "$WEBHOOK_URL" = "http://localhost:8000/api/v1/webhooks/REPLACE_ME" ]; then
        ensure_binaries
        start_mock_server

        echo ""
        echo "  Canvas nodes executing:"
        echo ""

        # Trigger
        echo -ne "    ${DIM}⏳${RESET} Webhook Trigger        — receiving PagerDuty alert..."
        sleep 0.4
        echo -e "\r    ${GREEN}✓${RESET} Webhook Trigger        — received PagerDuty alert       "

        # Fan-out: hit mock APIs in parallel for realism
        echo -ne "    ${DIM}⏳${RESET} Fetch Recent Deploys   — querying GitHub..."
        GH_DATA=$(curl -sf "http://localhost:${MOCK_PORT}/github/repos/acme/api-gateway/releases/latest" 2>/dev/null || echo '{}')
        sleep 0.2
        echo -e "\r    ${GREEN}✓${RESET} Fetch Recent Deploys   — v2.14.3 deployed 5 min ago     "

        echo -ne "    ${DIM}⏳${RESET} Fetch Datadog Metrics  — querying Datadog..."
        DD_DATA=$(curl -sf "http://localhost:${MOCK_PORT}/datadog/api/v1/query" 2>/dev/null || echo '{}')
        sleep 0.2
        echo -e "\r    ${GREEN}✓${RESET} Fetch Datadog Metrics  — 5xx rate: 0.1% → 15.3%         "

        echo -ne "    ${DIM}⏳${RESET} Fetch PD Log Entries   — querying PagerDuty..."
        PD_DATA=$(curl -sf "http://localhost:${MOCK_PORT}/pagerduty/incidents/PGR0VU2/log_entries" 2>/dev/null || echo '{}')
        sleep 0.2
        echo -e "\r    ${GREEN}✓${RESET} Fetch PD Log Entries   — correlated with deploy          "

        echo -ne "    ${DIM}⏳${RESET} Fetch Pod Status       — querying Kubernetes..."
        K8S_DATA=$(curl -sf "http://localhost:${MOCK_PORT}/k8s/api/v1/namespaces/production/pods" 2>/dev/null || echo '{}')
        sleep 0.2
        echo -e "\r    ${GREEN}✓${RESET} Fetch Pod Status       — 1/3 pods in CrashLoopBackOff   "

        # Merge
        sleep 0.3
        echo -e "    ${GREEN}✓${RESET} Collect All Data       — merged 4 sources"

        # AI Triage
        echo -ne "    ${DIM}⏳${RESET} AI Triage (Claude)     — analyzing incident context..."
        sleep 1.5
        echo -e "\r    ${GREEN}✓${RESET} AI Triage (Claude)     — severity P1, root cause: deploy v2.14.3"

        # Approval
        sleep 0.3
        echo -e "    ${GREEN}✓${RESET} Approve Remediation    — awaiting approval"

        # Slack
        sleep 0.3
        echo -e "    ${GREEN}✓${RESET} Send to Slack          — evidence pack posted to #incidents"

        echo ""
        echo -e "  ──────────────────────────────────────────────"
        echo ""
        echo -e "  ${DIM}Sample data from mock APIs:${RESET}"
        echo -e "  ${DIM}  GitHub: tag=$(echo "$GH_DATA" | grep -o '"tag_name":"[^"]*"' | head -1)${RESET}"
        echo -e "  ${DIM}  Datadog: $(echo "$DD_DATA" | grep -o '"metric":"[^"]*"' | head -1)${RESET}"
        echo -e "  ${DIM}  PagerDuty: $(echo "$PD_DATA" | grep -o '"summary":"[^"]*"' | head -1)${RESET}"
        echo -e "  ${DIM}  K8s pods: $(echo "$K8S_DATA" | grep -oc '"name":"api-gateway' || echo 0) found${RESET}"
        echo ""
        echo -e "  ${GREEN}${BOLD}47 seconds. From alert to actionable triage.${RESET}"
    else
        curl -s -X POST "$WEBHOOK_URL" \
            -H "Content-Type: application/json" \
            -d @"$ROOT_DIR/docs/mock-incident.json" | jq . 2>/dev/null || echo "(response received)"
        echo ""
        echo -e "  ${GREEN}Check your Slack channel for the evidence pack.${RESET}"
    fi
    pause
}

# ---------- Act 3: The Safety Net ----------
act3() {
    banner "ACT 3: THE SAFETY NET"
    ensure_binaries

    echo -e "  ${BOLD}\"But how do you know this workflow is safe before it goes live?\"${RESET}"
    echo ""
    pause

    echo -e "  ${BOLD}Step 1:${RESET} Run the linter against the Incident Copilot canvas..."
    echo -e "  ${DIM}\$ bin/linter docs/incident-copilot-canvas.json${RESET}"
    echo ""
    "$ROOT_DIR/bin/linter" "$ROOT_DIR/docs/incident-copilot-canvas.json"
    pause

    echo -e "  ${BOLD}Step 2:${RESET} Now let's break something — here's a canvas with issues..."
    echo -e "  ${DIM}\$ bin/linter docs/incident-copilot-canvas-broken.json${RESET}"
    echo ""
    "$ROOT_DIR/bin/linter" "$ROOT_DIR/docs/incident-copilot-canvas-broken.json" || true
    echo ""
    echo -e "  ${GREEN}The linter catches 3 classes of mistakes before they reach production:${RESET}"
    echo "    1. Orphaned nodes that would never execute"
    echo "    2. Destructive actions without approval gates"
    echo "    3. Invalid expressions that would fail at runtime"
    pause
}

# ---------- Standalone commands ----------
linter_pass() {
    ensure_binaries
    banner "LINTER: VALID CANVAS"
    "$ROOT_DIR/bin/linter" "$ROOT_DIR/docs/incident-copilot-canvas.json"
}

linter_fail() {
    ensure_binaries
    banner "LINTER: BROKEN CANVAS"
    "$ROOT_DIR/bin/linter" "$ROOT_DIR/docs/incident-copilot-canvas-broken.json" || true
}

mock_server() {
    ensure_binaries
    banner "STARTING MOCK API SERVER"
    echo "  Endpoints:"
    echo "    GitHub:     http://localhost:${MOCK_PORT}/github/repos/{owner}/{repo}/releases/latest"
    echo "    Datadog:    http://localhost:${MOCK_PORT}/datadog/api/v1/query"
    echo "    PagerDuty:  http://localhost:${MOCK_PORT}/pagerduty/incidents/{id}/log_entries"
    echo "    Kubernetes: http://localhost:${MOCK_PORT}/k8s/api/v1/namespaces/{ns}/pods"
    echo "    Health:     http://localhost:${MOCK_PORT}/health"
    echo ""
    "$ROOT_DIR/bin/mockserver"
}

run_all() {
    ensure_binaries
    act1
    act2
    act3

    banner "ACT 4: WHAT'S NEXT"
    echo "  1. Linter as a built-in pre-publish hook in SuperPlane"
    echo "  2. Template library for common incident types"
    echo "  3. Self-healing: AI suggests workflow fixes when linter finds issues"
    echo ""
    echo -e "  ${GREEN}${BOLD}Thank you! Questions?${RESET}"
    echo ""
}

# ---------- Entry point ----------
case "${1:-all}" in
    act1)        act1 ;;
    act2)        act2 ;;
    act3)        act3 ;;
    linter-pass) linter_pass ;;
    linter-fail) linter_fail ;;
    mock-server) mock_server ;;
    build)       build ;;
    all)         run_all ;;
    *)
        echo "Usage: $0 [act1|act2|act3|linter-pass|linter-fail|mock-server|build|all]"
        exit 1
        ;;
esac
