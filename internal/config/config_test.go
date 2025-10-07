package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argocd-lint/argocd-lint/pkg/types"
)

func TestLoadEmptyConfig(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Rules) != 0 {
		t.Fatalf("expected empty rules map")
	}
}

func TestResolveWithOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	content := []byte("rules:\n  AR001:\n    severity: warn\n    enabled: true\noverrides:\n  - pattern: 'apps/*.yaml'\n    rules:\n      AR001:\n        enabled: false\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	meta := types.RuleMetadata{
		ID:              "AR001",
		Description:     "",
		DefaultSeverity: types.SeverityError,
		Enabled:         true,
	}
	rule, err := cfg.Resolve(meta, "manifests/app.yaml")
	if err != nil {
		t.Fatalf("resolve default: %v", err)
	}
	if rule.Severity != types.SeverityWarn {
		t.Fatalf("expected severity warn, got %s", rule.Severity)
	}
	if !rule.Enabled {
		t.Fatalf("expected rule enabled")
	}

	rule, err = cfg.Resolve(meta, "apps/test.yaml")
	if err != nil {
		t.Fatalf("resolve override: %v", err)
	}
	if rule.Enabled {
		t.Fatalf("expected rule disabled by override")
	}
}

func TestConfigThreshold(t *testing.T) {
	cfg := Config{Threshold: "warn"}
	if cfg.Threshold != "warn" {
		t.Fatalf("expected threshold warn, got %s", cfg.Threshold)
	}
}

func TestApplyProfiles(t *testing.T) {
	cfg := Config{}
	if err := cfg.ApplyProfiles("prod"); err != nil {
		t.Fatalf("apply profile: %v", err)
	}
	if cfg.Threshold != "error" {
		t.Fatalf("expected threshold error, got %s", cfg.Threshold)
	}
	rule, ok := cfg.Rules["AR013"]
	if !ok {
		t.Fatalf("expected rule override for AR013")
	}
	if rule.Severity != "error" {
		t.Fatalf("expected severity error, got %s", rule.Severity)
	}
	if err := cfg.ApplyProfiles("security"); err != nil {
		t.Fatalf("apply additional profile: %v", err)
	}
	if cfg.Threshold != "error" {
		t.Fatalf("expected threshold to remain error, got %s", cfg.Threshold)
	}
}

func TestLoadAppliesProfilesFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("profiles:\n  - dev\n  - security\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Threshold != "warn" {
		t.Fatalf("expected dev threshold warn, got %s", cfg.Threshold)
	}
	if rule, ok := cfg.Rules["AR013"]; !ok || rule.Severity != "error" {
		t.Fatalf("expected security profile to keep AR013 severity error")
	}
}

func TestApplyProfilesUnknown(t *testing.T) {
	cfg := Config{}
	if err := cfg.ApplyProfiles("unknown"); err == nil {
		t.Fatalf("expected error for unknown profile")
	}
}

func TestWaiverValidation(t *testing.T) {
	good := Waiver{Rule: "AR001", File: "apps/*.yaml", Reason: "migration", Expires: "2099-01-01"}
	if err := good.Validate(); err != nil {
		t.Fatalf("expected valid waiver, got %v", err)
	}
	bad := Waiver{Rule: "", File: "*.yaml", Reason: "", Expires: "yesterday"}
	if err := bad.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestParseSeverityErrors(t *testing.T) {
	if sev, err := ParseSeverity("critical"); err == nil {
		t.Fatalf("expected error on unknown severity")
	} else if sev != "" {
		t.Fatalf("expected empty severity on error, got %q", sev)
	}
	if sev, err := ParseSeverity(""); err == nil {
		t.Fatalf("expected error on empty severity")
	} else if sev != "" {
		t.Fatalf("expected empty severity on error, got %q", sev)
	}
}
