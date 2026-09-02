package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFilesSkipsHiddenDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.yaml"), []byte("kind: Application\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "hidden.yaml"), []byte("kind: Application\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := DiscoverFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "app.yaml" {
		t.Fatalf("unexpected files: %v", files)
	}
}

func TestDiscoverFilesRejectsNonManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DiscoverFiles(path); err == nil {
		t.Fatal("expected non-manifest error")
	}
}
