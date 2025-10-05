package rego_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/manifest"
	regoloader "github.com/argocd-lint/argocd-lint/pkg/plugin/rego"
)

func TestLoaderLoadsRegoPlugin(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "require_name.rego")
	module := `package argocd_lint.require_name

metadata := {
  "id": "RG001",
  "description": "Application name must be foo",
  "severity": "error",
  "applies_to": ["Application"],
  "category": "Consistency",
  "help_url": "https://example.com/plugins",
}

deny[f] {
  input.kind == "Application"
  input.name != "foo"
  f := {
    "message": sprintf("name %s must equal foo", [input.name]),
    "line": 17,
    "column": 3,
    "resource_name": input.name,
  }
}
`
	if err := os.WriteFile(modulePath, []byte(module), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	loader := regoloader.NewLoader(modulePath)
	plugins, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	plug := plugins[0]
	if plug.Metadata().ID != "RG001" {
		t.Fatalf("unexpected metadata ID: %s", plug.Metadata().ID)
	}
	if plug.Metadata().DefaultSeverity != "error" {
		t.Fatalf("unexpected severity: %s", plug.Metadata().DefaultSeverity)
	}

	manifest := &manifest.Manifest{
		FilePath: "apps/app.yaml",
		Kind:     "Application",
		Name:     "demo",
		Line:     10,
		Column:   2,
		Object: map[string]interface{}{
			"kind": "Application",
			"metadata": map[string]interface{}{
				"name": "demo",
			},
		},
	}

	matcher := plug.AppliesTo()
	if matcher == nil || !matcher(manifest) {
		t.Fatalf("expected matcher to apply to Application")
	}

	findings, err := plug.Check(context.Background(), manifest)
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	finding := findings[0]
	if finding.Line != 17 {
		t.Fatalf("expected line 17, got %d", finding.Line)
	}
	if finding.ResourceName != "demo" {
		t.Fatalf("expected resource name demo, got %s", finding.ResourceName)
	}
	if finding.RuleID != "RG001" {
		t.Fatalf("expected rule id RG001, got %s", finding.RuleID)
	}
	if finding.Message == "" {
		t.Fatalf("expected message to be populated")
	}
}

func TestLoaderErrorsForMissingPaths(t *testing.T) {
	loader := regoloader.NewLoader("does-not-exist.rego")
	if _, err := loader.Load(context.Background()); err == nil {
		t.Fatalf("expected error for missing plugin path")
	}
}

func TestAppliesRuleSkipsManifest(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "applies.rego")
	module := `package argocd_lint.scope

metadata := {
  "id": "RG002",
  "description": "Only specific names are linted",
  "severity": "warn",
}

applies {
  startswith(input.name, "match-")
}

deny[f] {
  f := {
    "message": "should not see this when applies=false",
  }
}
`
	if err := os.WriteFile(modulePath, []byte(module), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}

	loader := regoloader.NewLoader(modulePath)
	plugins, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	plug := plugins[0]

	manifest := &manifest.Manifest{
		FilePath: "apps/app.yaml",
		Kind:     "Application",
		Name:     "demo",
		Object:   map[string]interface{}{},
	}

	findings, err := plug.Check(context.Background(), manifest)
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings when applies=false, got %d", len(findings))
	}

	manifest.Name = "match-demo"
	findings, err = plug.Check(context.Background(), manifest)
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	if len(findings) == 0 {
		t.Fatalf("expected findings when applies=true")
	}
}

func TestDiscoverMetadata(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "meta.rego")
	module := `package argocd_lint.meta

metadata := {
  "id": "RG010",
  "description": "metadata discovery test",
  "severity": "info",
  "applies_to": ["ApplicationSet"],
  "category": "Advisory",
}

deny[f] {
  f := {"message": "noop"}
}
`
	if err := os.WriteFile(modulePath, []byte(module), 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	records, missing, err := regoloader.DiscoverMetadata(context.Background(), modulePath)
	if err != nil {
		t.Fatalf("discover metadata: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("unexpected missing paths reported: %v", missing)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	record := records[0]
	if record.Metadata.ID != "RG010" {
		t.Fatalf("unexpected rule id: %s", record.Metadata.ID)
	}
	if record.Metadata.Description != "metadata discovery test" {
		t.Fatalf("unexpected description: %s", record.Metadata.Description)
	}
	if len(record.Metadata.AppliesTo) != 1 || record.Metadata.AppliesTo[0] != "ApplicationSet" {
		t.Fatalf("unexpected appliesTo: %+v", record.Metadata.AppliesTo)
	}
	if record.Source != modulePath {
		t.Fatalf("expected source %s, got %s", modulePath, record.Source)
	}
}
