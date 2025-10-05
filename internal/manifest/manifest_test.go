package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileFiltersUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	content := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: skip-me
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	parser := Parser{}
	manifests, err := parser.ParseFile(path)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0].Name != "demo" {
		t.Fatalf("expected name demo, got %s", manifests[0].Name)
	}
	if manifests[0].Kind != "Application" {
		t.Fatalf("expected Application kind")
	}
}
