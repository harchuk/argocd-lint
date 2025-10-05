package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
)

func TestRenderHelmFailureProducesFinding(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "chart")
	if err := os.Mkdir(chartDir, 0o755); err != nil {
		t.Fatalf("make chart dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: demo\nversion: 0.1.0\n"), 0o600); err != nil {
		t.Fatalf("write chart: %v", err)
	}

	cfg := config.Config{}
	renderer, err := NewRenderer(cfg, Options{
		Enabled:    true,
		HelmBinary: "false",
		RepoRoot:   dir,
	})
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}

	manifest := &manifest.Manifest{
		FilePath:     "app.yaml",
		Kind:         "Application",
		Name:         "demo",
		MetadataLine: 1,
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"path": "chart",
				},
			},
		},
	}

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

func TestRenderSkipsWhenDisabled(t *testing.T) {
	cfg := config.Config{}
	renderer, err := NewRenderer(cfg, Options{})
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	manifest := &manifest.Manifest{
		Kind: "Application",
	}
	findings, err := renderer.Render(manifest)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings when renderer disabled")
	}
}
