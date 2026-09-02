package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPluginsListTable(t *testing.T) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(self), "..", "..")
	coreDir := filepath.Join(root, "bundles", "core")
	var out bytes.Buffer
	var errBuf bytes.Buffer
	code := Execute([]string{"plugins", "list", "--dir", coreDir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	output := out.String()
	if !strings.Contains(output, "core") {
		t.Fatalf("expected bundle name in output")
	}
	if !strings.Contains(output, "RGC001") {
		t.Fatalf("expected rule id RGC001 in output")
	}
}

func TestHelpReturnsSuccess(t *testing.T) {
	var out bytes.Buffer
	var errBuf bytes.Buffer
	code := Execute([]string{"--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected help to return 0, got %d", code)
	}
	if strings.Contains(errBuf.String(), "[ERROR]") {
		t.Fatalf("help should not be reported as an error: %s", errBuf.String())
	}
}

func TestWriteBaselinePersistsCurrentFindings(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "app.yaml")
	manifest := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
  source:
    repoURL: https://example.com/repo.git
    targetRevision: main
    path: manifests
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	baselinePath := filepath.Join(dir, ".lint", "baseline.json")
	var out bytes.Buffer
	var errBuf bytes.Buffer
	code := Execute([]string{manifestPath, "--write-baseline", baselinePath, "--format", "json"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("expected lint findings to return 1, got %d (stderr: %s)", code, errBuf.String())
	}
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !strings.Contains(string(data), `"rule": "AR001"`) {
		t.Fatalf("expected current findings in baseline, got %s", data)
	}
}

func TestPluginsListJSON(t *testing.T) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(self), "..", "..")
	securityDir := filepath.Join(root, "bundles", "security")
	var out bytes.Buffer
	var errBuf bytes.Buffer
	code := Execute([]string{"plugins", "list", "--dir", securityDir, "--format", "json"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	output := out.String()
	if !strings.Contains(output, "security") {
		t.Fatalf("expected security bundle in JSON output")
	}
	if !strings.Contains(output, "helpUrl") {
		t.Fatalf("expected metadata fields in JSON output")
	}
}

func TestApplicationSetPlanTable(t *testing.T) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(self), "..", "..")
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
	appsetPath := filepath.Join(root, "internal", "cli", "test-appset.yaml")
	if err := os.WriteFile(appsetPath, []byte(appset), 0o600); err != nil {
		t.Fatalf("write appset: %v", err)
	}
	defer os.Remove(appsetPath)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	code := Execute([]string{"applicationset", "plan", "--file", appsetPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, errBuf.String())
	}
	output := out.String()
	if !strings.Contains(output, "app-two") {
		t.Fatalf("expected generated application in output: %s", output)
	}
	if !strings.Contains(strings.ToUpper(output), "CREATE") {
		t.Fatalf("expected CREATE action in plan output")
	}
}
