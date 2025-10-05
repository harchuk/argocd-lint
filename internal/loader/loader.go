package loader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverFiles returns manifest file paths within the provided target.
func DiscoverFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("stat target: %w", err)
	}
	if !info.IsDir() {
		if isManifestFile(target) {
			return []string{target}, nil
		}
		return nil, fmt.Errorf("file %s is not a YAML/JSON manifest", target)
	}
	var files []string
	walkErr := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != target && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if isManifestFile(path) {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}

func isManifestFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".json")
}
