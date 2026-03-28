# Incident Copilot — Canvas Setup Guide

Step-by-step for building the canvas in SuperPlane UI.

## Prerequisites

```bash
make dev.start                    # Start SuperPlane
make hackathon.build              # Build linter + mock server
bin/mockserver &                  # Start mock API server on :9999
```

Create secrets in SuperPlane UI:
- `anthropic_api_key` — your Anthropic API key
- `slack_bot_token` — Slack bot OAuth token (xoxb-...)

## Available Components (verified in codebase)

| Need | Native Component | Fallback |
|------|-----------------|----------|
| PagerDuty trigger | `pagerduty.OnIncident` | `webhook` |
| GitHub deploys | `github.ListReleases` | `http` GET |
| Datadog metrics | *(none — only CreateEvent)* | `http` GET |
| PagerDuty logs | `pagerduty.ListLogEntries` | `http` GET |
| K8s pod status | *(none)* | `http` GET |
| Merge results | `merge` | — |
| AI triage | `http` POST to Anthropic | — |
| Approval gate | `approval` | — |
| Slack output | `slack.SendTextMessage` | `http` POST |

**Recommendation:** Use `webhook` trigger + `http` for all data fetching (simpler, points at mock server). Use native `approval`. Use `slack.SendTextMessage` if Slack integration is connected, otherwise `http`.

## Node-by-Node Setup

### 1. Webhook Trigger — "PagerDuty Incident"
- **Type:** Trigger → `webhook`
- **Auth:** None (for demo)
- **Position:** Far left

### 2. HTTP — "Fetch Recent Deploys"
- **Type:** Component → `http`
- **Method:** GET
- **URL:** `http://host.docker.internal:9999/github/repos/acme/api-gateway/releases/latest`

### 3. HTTP — "Fetch Datadog Metrics"
- **Type:** Component → `http`
- **Method:** GET
- **URL:** `http://host.docker.internal:9999/datadog/api/v1/query`

### 4. HTTP — "Fetch PD Log Entries"
- **Type:** Component → `http`
- **Method:** GET
- **URL:** `http://host.docker.internal:9999/pagerduty/incidents/{{ event.data.id }}/log_entries`

### 5. HTTP — "Fetch Pod Status"
- **Type:** Component → `http`
- **Method:** GET
- **URL:** `http://host.docker.internal:9999/k8s/api/v1/namespaces/production/pods`

### 6. Merge — "Collect All Data"
- **Type:** Component → `merge`
- *(merge waits for all inputs before firing success)*

### 7. HTTP — "AI Triage (Claude)"
- **Type:** Component → `http`
- **Method:** POST
- **URL:** `https://api.anthropic.com/v1/messages`
- **Headers:** `{{ {"x-api-key": secrets.anthropic_api_key, "anthropic-version": "2023-06-01", "content-type": "application/json"} }}`
- **Body:** See `docs/demo/ai-triage-prompt.md`

### 8. Approval — "Approve Remediation"
- **Type:** Component → `approval`
- **Output channels:** `approved`, `rejected`

### 9. HTTP or Slack — "Send to Slack"
- **Type:** Component → `http` (or `slack.SendTextMessage` if integration available)
- **Method:** POST
- **URL:** `https://slack.com/api/chat.postMessage`
- **Headers:** `{{ {"Authorization": "Bearer " + secrets.slack_bot_token, "content-type": "application/json"} }}`
- **Body:** Use `docs/slack-evidence-pack.json` as template

## Edge Wiring

```
webhook-trigger  →(default)→  Fetch Recent Deploys
webhook-trigger  →(default)→  Fetch Datadog Metrics
webhook-trigger  →(default)→  Fetch PD Log Entries
webhook-trigger  →(default)→  Fetch Pod Status
Fetch Recent Deploys  →(success)→  Collect All Data
Fetch Datadog Metrics →(success)→  Collect All Data
Fetch PD Log Entries  →(success)→  Collect All Data
Fetch Pod Status      →(success)→  Collect All Data
Collect All Data      →(success)→  AI Triage (Claude)
AI Triage (Claude)    →(success)→  Approve Remediation
Approve Remediation   →(approved)→ Send to Slack
```

## Test

```bash
# Get webhook URL from the trigger node settings, then:
curl -X POST <WEBHOOK_URL> \
  -H 'Content-Type: application/json' \
  -d @docs/mock-incident.json
```

## Docker Note

If SuperPlane runs in Docker, `localhost:9999` won't reach the mock server on the host. Use:
- macOS: `host.docker.internal:9999`
- Linux: `172.17.0.1:9999` (or the docker bridge IP)
