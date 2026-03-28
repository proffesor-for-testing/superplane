package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Mock API server that simulates GitHub, Datadog, PagerDuty, and Kubernetes
// responses for the Incident Copilot demo. Run it alongside the canvas so
// HTTP nodes can point at localhost:9999 instead of real APIs.

const defaultPort = "9999"

func main() {
	port := defaultPort
	if p := os.Getenv("MOCK_PORT"); p != "" {
		port = p
	}

	mux := http.NewServeMux()

	// GitHub — latest release
	mux.HandleFunc("/github/repos/", handleGitHub)

	// Datadog — metrics query
	mux.HandleFunc("/datadog/api/v1/query", handleDatadog)

	// PagerDuty — incident log entries
	mux.HandleFunc("/pagerduty/incidents/", handlePagerDuty)

	// Kubernetes — pod list
	mux.HandleFunc("/k8s/api/v1/namespaces/", handleKubernetes)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("Mock API server starting on :%s", port)
	log.Printf("  GitHub:     http://localhost:%s/github/repos/{owner}/{repo}/releases/latest", port)
	log.Printf("  Datadog:    http://localhost:%s/datadog/api/v1/query", port)
	log.Printf("  PagerDuty:  http://localhost:%s/pagerduty/incidents/{id}/log_entries", port)
	log.Printf("  Kubernetes: http://localhost:%s/k8s/api/v1/namespaces/{ns}/pods", port)

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleGitHub(w http.ResponseWriter, r *http.Request) {
	log.Printf("[GitHub] %s %s", r.Method, r.URL.Path)
	respond(w, loadOrDefault("docs/mock-github-release.json", defaultGitHubRelease))
}

func handleDatadog(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Datadog] %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
	respond(w, loadOrDefault("docs/mock-datadog-metrics.json", defaultDatadogMetrics))
}

func handlePagerDuty(w http.ResponseWriter, r *http.Request) {
	log.Printf("[PagerDuty] %s %s", r.Method, r.URL.Path)
	if strings.HasSuffix(r.URL.Path, "/log_entries") {
		respond(w, loadOrDefault("docs/mock-pd-logs.json", defaultPDLogs))
	} else {
		respond(w, loadOrDefault("docs/mock-incident.json", defaultIncident))
	}
}

func handleKubernetes(w http.ResponseWriter, r *http.Request) {
	log.Printf("[K8s] %s %s", r.Method, r.URL.Path)
	respond(w, k8sPodList())
}

func respond(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func loadOrDefault(path string, fallback func() []byte) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return fallback()
	}
	return data
}

// ----------- Fallback data generators (used if JSON files don't exist) -----------

func defaultGitHubRelease() []byte {
	d, _ := json.Marshal(map[string]any{
		"tag_name":     "v2.14.3",
		"name":         "Release v2.14.3 - Order Service Refactor",
		"published_at": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		"body":         "## Changes\n- Refactored order validation logic\n- DB connection pool 20->50",
		"author":       map[string]any{"login": "braca"},
	})
	return d
}

func defaultDatadogMetrics() []byte {
	d, _ := json.Marshal(map[string]any{
		"status": "ok",
		"series": []map[string]any{
			{
				"metric": "api.gateway.error_rate_5xx",
				"points": [][]float64{{1711633200, 0.1}, {1711633260, 0.3}, {1711633320, 2.1}, {1711633380, 8.7}, {1711633440, 15.3}},
				"tags":   []string{"service:api-gateway", "env:production"},
			},
		},
	})
	return d
}

func defaultPDLogs() []byte {
	d, _ := json.Marshal(map[string]any{
		"log_entries": []map[string]any{
			{"type": "trigger_log_entry", "created_at": time.Now().Format(time.RFC3339), "summary": "Triggered by Datadog monitor: API 5xx Error Rate > 5%"},
			{"type": "notify_log_entry", "created_at": time.Now().Format(time.RFC3339), "summary": "Notified Dragan Petrovic via push notification"},
		},
	})
	return d
}

func defaultIncident() []byte {
	d, _ := json.Marshal(map[string]any{
		"event": map[string]any{
			"event_type": "incident.triggered",
			"data": map[string]any{
				"title":   "API Gateway: 5xx error rate spike to 15%",
				"urgency": "high",
				"status":  "triggered",
			},
		},
	})
	return d
}

func k8sPodList() []byte {
	now := time.Now().Format(time.RFC3339)
	d, _ := json.Marshal(map[string]any{
		"kind":       "PodList",
		"apiVersion": "v1",
		"items": []map[string]any{
			{
				"metadata": map[string]any{"name": "api-gateway-7b8d9f4c5-x2k4m", "namespace": "production", "createdAt": now},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []map[string]any{
						{"name": "api-gateway", "ready": true, "restartCount": 3, "state": map[string]any{"running": map[string]any{"startedAt": now}}},
					},
				},
			},
			{
				"metadata": map[string]any{"name": "api-gateway-7b8d9f4c5-j9n2p", "namespace": "production", "createdAt": now},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []map[string]any{
						{"name": "api-gateway", "ready": false, "restartCount": 7, "state": map[string]any{
							"waiting": map[string]any{"reason": "CrashLoopBackOff", "message": "back-off 5m0s restarting failed container"},
						}},
					},
				},
			},
			{
				"metadata": map[string]any{"name": "api-gateway-7b8d9f4c5-q3r8w", "namespace": "production", "createdAt": now},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []map[string]any{
						{"name": "api-gateway", "ready": true, "restartCount": 0, "state": map[string]any{"running": map[string]any{"startedAt": now}}},
					},
				},
			},
		},
	})
	return d
}

func init() {
	// Ensure fmt import is used
	_ = fmt.Sprint
}
