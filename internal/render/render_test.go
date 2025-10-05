package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
)

func fakeManifest(kind string) *manifest.Manifest {
	return &manifest.Manifest{
		FilePath:     "app.yaml",
		Kind:         kind,
		Name:         "demo",
		MetadataLine: 1,
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"project": "workloads",
				"destination": map[string]interface{}{
					"namespace": "demo",
				},
				"source": map[string]interface{}{
					"repoURL":        "https://example.com/repo.git",
					"targetRevision": "v1.0.0",
					"path":           "chart",
				},
			},
		},
	}
}

func shPath() string {
	if p, err := filepath.Abs("/bin/sh"); err == nil {
		return p
	}
	return "sh"
}

func TestRendererHelmFailure(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "chart")
	if err := os.Mkdir(chartDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o600); err != nil {
		t.Fatalf("write chart: %v", err)
	}

	cfg := config.Config{}
	renderer, err := NewRenderer(cfg, Options{
		Enabled:    true,
		HelmBinary: shPath(),
		RepoRoot:   dir,
	})
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	manifest := fakeManifest("Application")
	findings, err := renderer.Render(manifest)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "RENDER_HELM" {
		t.Fatalf("expected RENDER_HELM rule, got %s", findings[0].RuleID)
	}
}

func TestRendererDisabled(t *testing.T) {
	renderer, err := NewRenderer(config.Config{}, Options{Enabled: false})
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	findings, err := renderer.Render(fakeManifest("Application"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings when disabled")
	}
}
