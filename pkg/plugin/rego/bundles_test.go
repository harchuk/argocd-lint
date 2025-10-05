package rego_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	regoloader "github.com/argocd-lint/argocd-lint/pkg/plugin/rego"
)

func TestCuratedBundlesCompile(t *testing.T) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path")
	}
	root := filepath.Join(filepath.Dir(self), "..", "..", "..")
	bundlesDir := filepath.Join(root, "bundles")
	entries, err := os.ReadDir(bundlesDir)
	if err != nil {
		t.Fatalf("read bundles directory: %v", err)
	}
	ctx := context.Background()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		bundlePath := filepath.Join(bundlesDir, entry.Name())
		hasRego := false
		_ = filepath.WalkDir(bundlePath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(d.Name()) == ".rego" {
				hasRego = true
			}
			return nil
		})
		if !hasRego {
			continue
		}
		loader := regoloader.NewLoader(bundlePath)
		plugins, err := loader.Load(ctx)
		if err != nil {
			t.Fatalf("bundle %s failed to load: %v", entry.Name(), err)
		}
		if len(plugins) == 0 {
			t.Fatalf("bundle %s produced no plugins", entry.Name())
		}
		for _, plug := range plugins {
			meta := plug.Metadata()
			if meta.ID == "" {
				t.Fatalf("bundle %s contains plugin with empty id", entry.Name())
			}
		}
	}
}
