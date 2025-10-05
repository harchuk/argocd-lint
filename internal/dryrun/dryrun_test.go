package dryrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

func TestKubeconformFailureProducesFinding(t *testing.T) {
	workdir := t.TempDir()
	script := filepath.Join(workdir, "kubeconform")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'mock error' 1>&2\nexit 4\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	val := NewValidator(config.Config{}, workdir, Options{Enabled: true, Mode: modeKubeconform, KubeconformBinary: script})
	app := &manifest.Manifest{FilePath: "app.yaml", Kind: string(types.ResourceKindApplication), Name: "demo"}
	findings, err := val.Validate(context.Background(), []*manifest.Manifest{app})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	if findings[0].RuleID != "DRYRUN_KUBECONFORM" {
		t.Fatalf("expected DRYRUN_KUBECONFORM, got %s", findings[0].RuleID)
	}
}

func TestKubeconformSuccess(t *testing.T) {
	workdir := t.TempDir()
	script := filepath.Join(workdir, "kubeconform")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	val := NewValidator(config.Config{}, workdir, Options{Enabled: true, Mode: modeKubeconform, KubeconformBinary: script})
	app := &manifest.Manifest{FilePath: "app.yaml", Kind: string(types.ResourceKindApplication), Name: "demo"}
	findings, err := val.Validate(context.Background(), []*manifest.Manifest{app})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestKubectlFailureProducesFinding(t *testing.T) {
	workdir := t.TempDir()
	script := filepath.Join(workdir, "kubectl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'server rejected manifest' 1>&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	val := NewValidator(config.Config{}, workdir, Options{Enabled: true, Mode: modeServer, KubectlBinary: script})
	app := &manifest.Manifest{FilePath: filepath.Join(workdir, "app.yaml"), Kind: string(types.ResourceKindApplication), Name: "demo"}
	findings, err := val.Validate(context.Background(), []*manifest.Manifest{app})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	if findings[0].RuleID != "DRYRUN_SERVER" {
		t.Fatalf("expected DRYRUN_SERVER finding, got %s", findings[0].RuleID)
	}
}

func TestKubectlSuccess(t *testing.T) {
	workdir := t.TempDir()
	script := filepath.Join(workdir, "kubectl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	val := NewValidator(config.Config{}, workdir, Options{Enabled: true, Mode: modeServer, KubectlBinary: script})
	app := &manifest.Manifest{FilePath: "app.yaml", Kind: string(types.ResourceKindApplication), Name: "demo"}
	findings, err := val.Validate(context.Background(), []*manifest.Manifest{app})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestUnsupportedModeReturnsError(t *testing.T) {
	val := NewValidator(config.Config{}, "", Options{Enabled: true, Mode: "bogus"})
	if _, err := val.Validate(context.Background(), nil); err == nil {
		t.Fatalf("expected error for unsupported mode")
	}
}
