# AI Triage Prompt — Incident Copilot

## System Prompt (for AI component or HTTP body)

```
You are an SRE incident triage AI. Given incident data from multiple sources,
produce a structured JSON severity assessment. Be concise and actionable.
```

## User Prompt Template

Use this in the HTTP node body that calls `https://api.anthropic.com/v1/messages`:

```
Analyze this production incident using all available context.

## Incident Alert (PagerDuty)
{{ toJson(nodes['webhook-trigger-1'].output) }}

## Recent Deploys (GitHub)
{{ toJson(nodes['http-github-1'].output) }}

## Error Rate & Latency (Datadog)
{{ toJson(nodes['http-datadog-1'].output) }}

## Incident Logs (PagerDuty)
{{ toJson(nodes['http-pd-logs-1'].output) }}

## Pod Status (Kubernetes)
{{ toJson(nodes['http-k8s-1'].output) }}

---

Respond ONLY with this JSON — no markdown, no explanation:

{
  "severity": "P1",
  "title": "one-line summary",
  "customer_impact": "who is affected and how many",
  "root_causes": [
    {"rank": 1, "confidence_pct": 85, "description": "most likely cause"},
    {"rank": 2, "confidence_pct": 10, "description": "second cause"},
    {"rank": 3, "confidence_pct": 5, "description": "third cause"}
  ],
  "affected_systems": ["system1", "system2"],
  "recommended_actions": [
    {"priority": 1, "action": "what to do", "command": "cli command if applicable", "eta_minutes": 3},
    {"priority": 2, "action": "what to do next"},
    {"priority": 3, "action": "follow-up"}
  ],
  "escalation": {
    "current": "Team (Person)",
    "next": "Escalation target if unresolved"
  }
}
```

## Full HTTP Node Body (for Anthropic API call)

```json
{{ {
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 1024,
  "system": "You are an SRE incident triage AI. Given incident data from multiple sources, produce a structured JSON severity assessment. Be concise and actionable.",
  "messages": [{
    "role": "user",
    "content": "Analyze this production incident.\n\nIncident: " + toJson(nodes['webhook-trigger-1'].output) + "\n\nDeploys: " + toJson(nodes['http-github-1'].output) + "\n\nMetrics: " + toJson(nodes['http-datadog-1'].output) + "\n\nLogs: " + toJson(nodes['http-pd-logs-1'].output) + "\n\nPods: " + toJson(nodes['http-k8s-1'].output) + "\n\nRespond ONLY with JSON: {severity, title, customer_impact, root_causes: [{rank, confidence_pct, description}], affected_systems: [], recommended_actions: [{priority, action, command?, eta_minutes?}], escalation: {current, next}}"
  }]
} }}
```

## HTTP Node Config

| Field | Value |
|-------|-------|
| URL | `https://api.anthropic.com/v1/messages` |
| Method | `POST` |
| Headers | `{{ {"x-api-key": secrets.anthropic_api_key, "anthropic-version": "2023-06-01", "content-type": "application/json"} }}` |
| Body | (the template above) |

## Output Schema

| Field | Type | Example |
|-------|------|---------|
| `severity` | string | `"P1"` |
| `title` | string | `"API Gateway 5xx spike from deploy v2.14.3"` |
| `customer_impact` | string | `"~1,247 users unable to complete orders"` |
| `root_causes[].rank` | int | `1` |
| `root_causes[].confidence_pct` | int | `85` |
| `root_causes[].description` | string | `"DB pool exhaustion from v2.14.3"` |
| `affected_systems[]` | string[] | `["api-gateway", "order-service"]` |
| `recommended_actions[].priority` | int | `1` |
| `recommended_actions[].action` | string | `"Rollback deploy v2.14.3"` |
| `recommended_actions[].command` | string? | `"kubectl rollout undo deployment/api-gateway"` |
| `recommended_actions[].eta_minutes` | int? | `3` |
| `escalation.current` | string | `"Platform Engineering (Dragan)"` |
| `escalation.next` | string | `"Database Team (@db-oncall)"` |
