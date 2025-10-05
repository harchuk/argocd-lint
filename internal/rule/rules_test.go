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

func TestRuleSourceConsistencyConflicts(t *testing.T) {
	rl := ruleSourceConsistency()
	cfg := config.Config{}
	ctx := &Context{Config: cfg}
	manifest := &manifest.Manifest{
		FilePath:     "app.yaml",
		Kind:         string(types.ResourceKindApplication),
		Name:         "app",
		MetadataLine: 1,
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"sources": []interface{}{
					map[string]interface{}{
						"repoURL": "https://example.com/repo.git",
						"path":    "chart",
						"helm":    map[string]interface{}{"valueFiles": []interface{}{"values.yaml"}},
						"kustomize": map[string]interface{}{
							"namePrefix": "demo-",
						},
					},
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
		t.Fatalf("expected conflict findings, got none")
	}
}

func TestRuleAppProjectGuardrails(t *testing.T) {
	rl := ruleAppProjectGuardrails()
	cfg := config.Config{}
	ctx := &Context{Config: cfg}
	manifest := &manifest.Manifest{
		FilePath:     "project.yaml",
		Kind:         string(types.ResourceKindAppProject),
		Name:         "demo-project",
		MetadataLine: 1,
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"sourceNamespaces": []interface{}{"*"},
				"sourceRepos":      []interface{}{"*"},
				"destinations": []interface{}{
					map[string]interface{}{
						"namespace": "*",
					},
				},
			},
		},
	}
	configured, err := cfg.Resolve(rl.Metadata, manifest.FilePath)
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	findings := rl.Check(manifest, ctx, configured)
	if len(findings) < 3 {
		t.Fatalf("expected multiple guardrail findings, got %d", len(findings))
	}
}
