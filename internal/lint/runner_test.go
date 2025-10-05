package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/config"
)

func writeManifest(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestRunnerSuccessfulLint(t *testing.T) {
	dir := t.TempDir()
	manifest := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
  labels:
    app.kubernetes.io/name: demo
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: workloads
  destination:
    namespace: demo
    server: https://kubernetes.default.svc
  source:
    repoURL: https://example.com/repo.git
    targetRevision: v1.0.0
    path: manifests
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
`
	path := writeManifest(t, dir, "app.yaml", manifest)

	disabled := false
	cfg := config.Config{
		Rules: map[string]config.RuleConfig{
			"AR006": {Enabled: &disabled},
		},
	}

	runner, err := NewRunner(cfg, dir)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	report, err := runner.Run(Options{Target: path, Config: cfg})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(report.Findings))
	}
}

func TestRunnerDetectsDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	manifest := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  project: workloads
  destination:
    namespace: demo
    server: https://kubernetes.default.svc
  source:
    repoURL: https://example.com/repo.git
    targetRevision: v1.0.0
    path: manifests
`
	writeManifest(t, dir, "app1.yaml", manifest)
	writeManifest(t, dir, "app2.yaml", manifest)

	runner, err := NewRunner(config.Config{}, dir)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	report, err := runner.Run(Options{Target: dir, Config: config.Config{}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	found := false
	for _, f := range report.Findings {
		if f.RuleID == "AR011" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected duplicate name finding")
	}
}
