package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/lint"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

func sampleReport() lint.Report {
	meta := types.RuleMetadata{ID: "AR001", Description: "demo", Category: "test"}
	finding := types.Finding{
		RuleID:       "AR001",
		Message:      "example",
		Severity:     types.SeverityWarn,
		FilePath:     "demo.yaml",
		ResourceName: "demo",
		ResourceKind: "Application",
		Suggestions: []types.Suggestion{
			{
				Title: "Demo suggestion",
				Patch: "demo: patch",
			},
		},
	}
	return lint.Report{
		Findings: []types.Finding{finding},
		RuleIndex: map[string]types.RuleMetadata{
			meta.ID: meta,
		},
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(sampleReport(), FormatJSON, &buf); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	findings, ok := payload["findings"].([]interface{})
	if !ok || len(findings) != 1 {
		t.Fatalf("expected 1 finding in json output")
	}
	firstFinding, ok := findings[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected finding to be an object")
	}
	suggestions, ok := firstFinding["suggestions"].([]interface{})
	if !ok || len(suggestions) != 1 {
		t.Fatalf("expected suggestion payload in json output")
	}
}

func TestWriteTableNoFindings(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(lint.Report{}, FormatTable, &buf); err != nil {
		t.Fatalf("write table: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No findings.") {
		t.Fatalf("expected 'No findings.' message")
	}
	if !strings.Contains(output, "Summary: 0 findings") {
		t.Fatalf("expected summary with zero findings")
	}
}

func TestWriteSARIF(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(sampleReport(), FormatSARIF, &buf); err != nil {
		t.Fatalf("write sarif: %v", err)
	}
	if !strings.Contains(buf.String(), "\"version\": \"2.1.0\"") {
		t.Fatalf("expected SARIF version header")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}
	runs, ok := payload["runs"].([]interface{})
	if !ok || len(runs) == 0 {
		t.Fatalf("expected sarif runs array")
	}
	firstRun, ok := runs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected run to be an object")
	}
	results, ok := firstRun["results"].([]interface{})
	if !ok || len(results) == 0 {
		t.Fatalf("expected results array in sarif output")
	}
	firstResult, ok := results[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result to be an object")
	}
	props, ok := firstResult["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties block with suggestions")
	}
	sarifSuggestions, ok := props["suggestions"].([]interface{})
	if !ok || len(sarifSuggestions) != 1 {
		t.Fatalf("expected sarif suggestions entry")
	}
}

func TestHighestSeverity(t *testing.T) {
	findings := []types.Finding{
		{Severity: types.SeverityInfo},
		{Severity: types.SeverityWarn},
		{Severity: types.SeverityError},
	}
	if got := HighestSeverity(findings); got != types.SeverityError {
		t.Fatalf("expected highest severity error, got %s", got)
	}
}

func TestSummaryString(t *testing.T) {
	findings := []types.Finding{{Severity: types.SeverityWarn}, {Severity: types.SeverityWarn}}
	summary := SummaryString(findings)
	if !strings.Contains(summary, "2 findings") {
		t.Fatalf("expected count in summary")
	}
	if !strings.Contains(summary, "2 warn") {
		t.Fatalf("expected warn count in summary")
	}
}
