package schema

import (
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/manifest"
)

func TestSchemaValidatorDetectsInvalidApplication(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	m := &manifest.Manifest{
		FilePath: "bad.yaml",
		Kind:     "Application",
		Name:     "bad",
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"spec": map[string]interface{}{
				"project": "",
				"destination": map[string]interface{}{
					"server": "https://kubernetes.default.svc",
				},
				"source": map[string]interface{}{
					"repoURL":        "https://example.com/repo.git",
					"targetRevision": "HEAD",
					"path":           "app",
				},
			},
		},
	}
	findings, err := validator.Validate(m)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(findings) == 0 {
		t.Fatalf("expected schema findings for incomplete manifest")
	}
}

func TestSchemaValidatorAcceptsValidApplicationSet(t *testing.T) {
	validator, err := NewValidator()
	if err != nil {
		t.Fatalf("new validator: %v", err)
	}
	m := &manifest.Manifest{
		FilePath: "good.yaml",
		Kind:     "ApplicationSet",
		Name:     "demo",
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "ApplicationSet",
			"spec": map[string]interface{}{
				"generators": []interface{}{
					map[string]interface{}{
						"list": map[string]interface{}{
							"elements": []interface{}{
								map[string]interface{}{"cluster": "prod", "url": "https://example.com"},
							},
						},
					},
				},
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"project": "workloads",
						"destination": map[string]interface{}{
							"server":    "https://example.com",
							"namespace": "demo",
						},
						"source": map[string]interface{}{
							"repoURL":        "https://example.com/repo.git",
							"targetRevision": "v1.0.0",
							"path":           "app",
						},
					},
				},
			},
		},
	}
	findings, err := validator.Validate(m)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}
