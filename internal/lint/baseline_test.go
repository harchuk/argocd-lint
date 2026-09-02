package lint

import (
	"path/filepath"
	"testing"

	"github.com/harchuk/argocd-lint/pkg/types"
)

func TestBaselineResourceIdentityAndLegacyCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	finding := types.Finding{RuleID: "AR001", FilePath: "app.yaml", ResourceKind: "Application", ResourceName: "demo"}
	if err := WriteBaseline(path, []types.Finding{finding}); err != nil {
		t.Fatal(err)
	}
	bl, err := LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	remaining, _, _ := bl.Filter([]types.Finding{finding}, 0)
	if len(remaining) != 0 {
		t.Fatalf("resource finding was not suppressed: %#v", remaining)
	}
	legacy := &Baseline{index: map[string]BaselineEntry{baselineKey("app.yaml", "AR001", "", ""): {File: "app.yaml", Rule: "AR001"}}}
	remaining, _, _ = legacy.Filter([]types.Finding{finding}, 0)
	if len(remaining) != 0 {
		t.Fatal("legacy baseline entry was not honored")
	}
}
