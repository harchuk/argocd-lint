package appsetplan

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func TestGenerateListPlan(t *testing.T) {
	dir := t.TempDir()
	currentDir := filepath.Join(dir, "current")
	if err := os.Mkdir(currentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	currentApp := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-one
spec:
  project: default
  destination:
    namespace: apps
    server: https://example.com
  source:
    repoURL: https://example.com/repo.git
    path: apps/app-one
`
	writeFile(t, currentDir, "app-one.yaml", currentApp)

	appset := `apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: preview
spec:
  generators:
    - list:
        elements:
          - name: app-one
            namespace: apps
            server: https://example.com
          - name: app-two
            namespace: apps
            server: https://example.com
  template:
    metadata:
      name: '{{ name }}'
    spec:
      project: default
      destination:
        server: '{{ server }}'
        namespace: '{{ namespace }}'
      source:
        repoURL: https://example.com/repo.git
        path: apps/{{ name }}
`
	appsetPath := writeFile(t, dir, "appset.yaml", appset)

	result, err := Generate(Options{AppSetPath: appsetPath, CurrentDir: currentDir})
	if err != nil {
		t.Fatalf("generate plan: %v", err)
	}
	if result.Summary.Total != 2 {
		t.Fatalf("expected 2 rows, got %d", result.Summary.Total)
	}
	var foundCreate, foundUnchanged bool
	for _, row := range result.Rows {
		switch row.Name {
		case "app-one":
			if row.Action != ActionUnchange {
				t.Fatalf("expected app-one unchanged, got %s", row.Action)
			}
			foundUnchanged = true
		case "app-two":
			if row.Action != ActionCreate {
				t.Fatalf("expected app-two create, got %s", row.Action)
			}
			if row.Source.Path != "apps/app-two" {
				t.Fatalf("unexpected source path: %s", row.Source.Path)
			}
			foundCreate = true
		default:
			t.Fatalf("unexpected row: %+v", row)
		}
	}
	if !foundCreate || !foundUnchanged {
		t.Fatalf("expected both create and unchanged actions")
	}
}
