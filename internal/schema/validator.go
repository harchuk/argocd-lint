package schema

import (
    _ "embed"
    "fmt"

    "github.com/argocd-lint/argocd-lint/internal/manifest"
    "github.com/argocd-lint/argocd-lint/pkg/types"
    "github.com/xeipuuv/gojsonschema"
)

var (
    //go:embed data/application.json
    applicationSchema string
    //go:embed data/applicationset.json
    applicationSetSchema string
)

// Validator performs JSON schema validation using embedded CRD specs.
type Validator struct {
    appLoader       gojsonschema.JSONLoader
    appSetLoader    gojsonschema.JSONLoader
    ruleApplication types.ConfiguredRule
    ruleAppSet      types.ConfiguredRule
}

// NewValidator constructs a schema validator.
func NewValidator() (*Validator, error) {
    appLoader := gojsonschema.NewStringLoader(applicationSchema)
    appSetLoader := gojsonschema.NewStringLoader(applicationSetSchema)
    return &Validator{
        appLoader:    appLoader,
        appSetLoader: appSetLoader,
        ruleApplication: types.ConfiguredRule{
            Metadata: types.RuleMetadata{
                ID:              "SCHEMA_APPLICATION",
                Description:     "Application manifest must satisfy the Argo CD Application CRD schema",
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
                Description:     "ApplicationSet manifest must satisfy the Argo CD ApplicationSet CRD schema",
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
