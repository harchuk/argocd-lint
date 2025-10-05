package rule

import (
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

func TestRuleTargetRevisionPinned(t *testing.T) {
	rl := ruleTargetRevisionPinned()
	cfg := config.Config{}
	ctx := &Context{Config: cfg}
	manifest := &manifest.Manifest{
		FilePath:     "test.yaml",
		Kind:         string(types.ResourceKindApplication),
		Name:         "demo",
		MetadataLine: 1,
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"repoURL":        "https://example.com/repo.git",
					"targetRevision": "HEAD",
				},
			},
		},
	}
	configured, err := cfg.Resolve(rl.Metadata, manifest.FilePath)
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	findings := rl.Check(manifest, ctx, configured)
	if len(findings) == 0 {
		t.Fatalf("expected findings, got none")
	}
	if findings[0].Severity != types.SeverityError {
		t.Fatalf("expected error severity, got %s", findings[0].Severity)
	}
}
