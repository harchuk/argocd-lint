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

func TestParseSeverityErrors(t *testing.T) {
    if _, err := ParseSeverity("critical"); err == nil {
        t.Fatalf("expected error on unknown severity")
    }
    if _, err := ParseSeverity(""); err == nil {
        t.Fatalf("expected error on empty severity")
    }
}
