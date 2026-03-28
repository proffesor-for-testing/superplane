package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/superplanehq/superplane/pkg/linter"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <canvas.json> [--json]\n", os.Args[0])
		os.Exit(1)
	}

	path := os.Args[1]
	jsonOutput := len(os.Args) > 2 && os.Args[2] == "--json"

	l := linter.New()
	result, err := l.LintFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		printHuman(result)
	}

	if !result.Pass {
		os.Exit(1)
	}
}

func printHuman(result linter.Result) {
	if result.Pass {
		fmt.Printf("\n%s PASS%s — Canvas passed all lint checks.\n\n", colorGreen, colorReset)
		return
	}

	fmt.Printf("\n%s FAIL%s — %d issue(s) found:\n\n", colorRed, colorReset, len(result.Issues))

	for i, issue := range result.Issues {
		sev := colorRed + "ERROR" + colorReset
		if issue.Severity == linter.SeverityWarning {
			sev = colorYellow + "WARN " + colorReset
		}

		node := issue.NodeID
		if node == "" {
			node = "(global)"
		}

		fmt.Printf("  %d. [%s] %s\n", i+1, sev, issue.Message)
		fmt.Printf("     Rule: %s | Node: %s\n\n", issue.Rule, node)
	}
}
