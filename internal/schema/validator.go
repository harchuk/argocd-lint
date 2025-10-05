package schema

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/types"
	"github.com/xeipuuv/gojsonschema"
)

var (
	//go:embed data/*/*.json
	schemaFiles embed.FS

	supportedVersions = map[string]string{
		"":     "v2.9",
		"v2.9": "v2.9",
		"v2.8": "v2.8",
	}
)

// Validator performs JSON schema validation using embedded CRD specs.
type Validator struct {
	version         string
	appLoader       gojsonschema.JSONLoader
	appSetLoader    gojsonschema.JSONLoader
	ruleApplication types.ConfiguredRule
	ruleAppSet      types.ConfiguredRule
}

// NewValidator constructs a schema validator for the selected Argo CD version.
func NewValidator(version string) (*Validator, error) {
	resolved, err := resolveVersion(version)
	if err != nil {
		return nil, err
	}
	appSchema, err := schemaFiles.ReadFile(filepath.Join("data", resolved, "application.json"))
	if err != nil {
		return nil, fmt.Errorf("load application schema for %s: %w", resolved, err)
	}
	appSetSchema, err := schemaFiles.ReadFile(filepath.Join("data", resolved, "applicationset.json"))
	if err != nil {
		return nil, fmt.Errorf("load applicationset schema for %s: %w", resolved, err)
	}
	appLoader := gojsonschema.NewStringLoader(string(appSchema))
	appSetLoader := gojsonschema.NewStringLoader(string(appSetSchema))
	versionSuffix := formatDescriptionSuffix(resolved)
	return &Validator{
		version:      resolved,
		appLoader:    appLoader,
		appSetLoader: appSetLoader,
		ruleApplication: types.ConfiguredRule{
			Metadata: types.RuleMetadata{
				ID:              "SCHEMA_APPLICATION",
				Description:     "Application manifest must satisfy the Argo CD Application CRD schema" + versionSuffix,
				DefaultSeverity: types.SeverityError,
				AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
				Category:        "schema",
				Enabled:         true,
			},
			Severity: types.SeverityError,
			Enabled:  true,
		},
		ruleAppSet: types.ConfiguredRule{
			Metadata: types.RuleMetadata{
				ID:              "SCHEMA_APPLICATIONSET",
				Description:     "ApplicationSet manifest must satisfy the Argo CD ApplicationSet CRD schema" + versionSuffix,
				DefaultSeverity: types.SeverityError,
				AppliesTo:       []types.ResourceKind{types.ResourceKindApplicationSet},
				Category:        "schema",
				Enabled:         true,
			},
			Severity: types.SeverityError,
			Enabled:  true,
		},
	}, nil
}

func resolveVersion(version string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(version))
	trimmed = strings.TrimPrefix(trimmed, "argocd-")
	if trimmed == "" {
		return supportedVersions[""], nil
	}
	trimmed = strings.TrimPrefix(trimmed, "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) >= 2 {
		trimmed = fmt.Sprintf("v%s.%s", parts[0], parts[1])
	} else if len(parts) == 1 {
		trimmed = fmt.Sprintf("v%s", parts[0])
	}
	if resolved, ok := supportedVersions[trimmed]; ok {
		return resolved, nil
	}
	return "", fmt.Errorf("unsupported argocd version %q", version)
}

func formatDescriptionSuffix(version string) string {
	if version == "" {
		return ""
	}
	return fmt.Sprintf(" (%s)", version)
}

// Metadata returns schema rule metadata entries.
func (v *Validator) Metadata() []types.RuleMetadata {
	return []types.RuleMetadata{v.ruleApplication.Metadata, v.ruleAppSet.Metadata}
}

// Validate checks the manifest against the matching schema.
func (v *Validator) Validate(m *manifest.Manifest) ([]types.Finding, error) {
	if m == nil {
		return nil, fmt.Errorf("manifest is nil")
	}
	var loader gojsonschema.JSONLoader
	var rule types.ConfiguredRule
	switch m.Kind {
	case string(types.ResourceKindApplication):
		loader = v.appLoader
		rule = v.ruleApplication
	case string(types.ResourceKindApplicationSet):
		loader = v.appSetLoader
		rule = v.ruleAppSet
	default:
		return nil, nil
	}
	docLoader := gojsonschema.NewGoLoader(m.Object)
	result, err := gojsonschema.Validate(loader, docLoader)
	if err != nil {
		return nil, fmt.Errorf("schema validation error: %w", err)
	}
	if result.Valid() {
		return nil, nil
	}
	builder := types.FindingBuilder{
		Rule:         rule,
		FilePath:     m.FilePath,
		Line:         m.Line,
		ResourceName: m.Name,
		ResourceKind: m.Kind,
	}
	findings := make([]types.Finding, 0, len(result.Errors()))
	for _, err := range result.Errors() {
		findings = append(findings, builder.NewFinding(err.String(), types.SeverityError))
	}
	return findings, nil
}
