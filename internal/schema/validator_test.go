package schema

import (
	"testing"

	"github.com/argocd-lint/argocd-lint/internal/manifest"
)

func TestSchemaValidatorDetectsInvalidApplication(t *testing.T) {
	validator, err := NewValidator("")
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
	validator, err := NewValidator("")
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
			"metadata": map[string]interface{}{
				"name": "demo",
			},
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
	for _, f := range findings {
		if f.RuleID == "SCHEMA_APPLICATIONSET" {
			t.Fatalf("expected valid ApplicationSet, got schema finding: %s", f.Message)
		}
	}
}

func TestSchemaValidatorSupportsVersionSelection(t *testing.T) {
	versions := []string{"v2.8", "v2.9"}
	for _, version := range versions {
		validator, err := NewValidator(version)
		if err != nil {
			t.Fatalf("new validator for %s: %v", version, err)
		}
		m := &manifest.Manifest{
			FilePath: "good.yaml",
			Kind:     "Application",
			Name:     "demo",
			Object: map[string]interface{}{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata": map[string]interface{}{
					"name": "demo",
				},
				"spec": map[string]interface{}{
					"project": "workloads",
					"destination": map[string]interface{}{
						"server":    "https://kubernetes.default.svc",
						"namespace": "demo",
					},
					"source": map[string]interface{}{
						"repoURL":        "https://example.com/repo.git",
						"targetRevision": "v1.0.0",
						"path":           "manifests",
					},
				},
			},
		}
		findings, err := validator.Validate(m)
		if err != nil {
			t.Fatalf("validate %s: %v", version, err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected no findings for version %s, got %d", version, len(findings))
		}
	}
}

func TestSchemaValidatorRejectsUnknownVersion(t *testing.T) {
	if _, err := NewValidator("v9.9"); err == nil {
		t.Fatalf("expected error for unsupported version")
	}
}
