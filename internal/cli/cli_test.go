package cli

import (
	"bytes"
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
