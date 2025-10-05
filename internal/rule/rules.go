package rule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

// Context provides additional data for rule evaluation.
type Context struct {
	Config    config.Config
	Manifests []*manifest.Manifest
}

// Rule is a lint rule definition.
type Rule struct {
	Metadata types.RuleMetadata
	Applies  func(*manifest.Manifest) bool
	Check    func(*manifest.Manifest, *Context, types.ConfiguredRule) []types.Finding
}

// DefaultRules returns all built-in rules.
func DefaultRules() []Rule {
	return []Rule{
		ruleTargetRevisionPinned(),
		ruleProjectNotDefault(),
		ruleDestinationNamespace(),
		ruleSyncPolicyExplicit(),
		ruleSyncPolicyAutomatedSafety(),
		ruleFinalizerAware(),
		ruleIgnoreDifferencesScoped(),
		ruleApplicationSetGoTemplateOptions(),
		ruleSourceConsistency(),
		ruleRecommendedLabels(),
	}
}

var (
	floatingRevisionPattern = regexp.MustCompile(`(?i)^(head|latest|tip|main|master|trunk)$`)
	wildcardPattern         = regexp.MustCompile(`[\*]`)
	semverWildcard          = regexp.MustCompile(`(?i)^v?\d+\.[^\n]*\*`)
)

func ruleTargetRevisionPinned() Rule {
	meta := types.RuleMetadata{
		ID:              "AR001",
		Description:     "targetRevision must be pinned to an immutable value",
		DefaultSeverity: types.SeverityWarn,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication, types.ResourceKindApplicationSet},
		HelpURL:         "https://argo-cd.readthedocs.io/en/stable/user-guide/application_sources/",
		Category:        "best-practice",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies: func(m *manifest.Manifest) bool {
			return m.Kind == string(types.ResourceKindApplication) || m.Kind == string(types.ResourceKindApplicationSet)
		},
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			builder := types.FindingBuilder{
				Rule:         cfg,
				FilePath:     m.FilePath,
				Line:         m.MetadataLine,
				ResourceName: m.Name,
				ResourceKind: m.Kind,
			}
			var findings []types.Finding
			switch m.Kind {
			case string(types.ResourceKindApplication):
				src := getMap(m.Object, "spec", "source")
				findings = append(findings, checkRevision(builder, src)...)
				sources := getSlice(m.Object, "spec", "sources")
				for _, item := range sources {
					if sub, ok := item.(map[string]interface{}); ok {
						findings = append(findings, checkRevision(builder, sub)...)
					}
				}
			case string(types.ResourceKindApplicationSet):
				template := getMap(m.Object, "spec", "template", "spec", "source")
				findings = append(findings, checkRevision(builder, template)...)
			}
			return findings
		},
	}
}

func checkRevision(builder types.FindingBuilder, src map[string]interface{}) []types.Finding {
	var findings []types.Finding
	rev := getString(src, "targetRevision")
	if rev == "" {
		findings = append(findings, builder.NewFinding("targetRevision is empty; pin to a tag or commit", types.SeverityWarn))
		return findings
	}
	if rev == "HEAD" {
		findings = append(findings, builder.NewFinding("targetRevision 'HEAD' is not immutable", types.SeverityError))
		return findings
	}
	if floatingRevisionPattern.MatchString(rev) {
		findings = append(findings, builder.NewFinding(fmt.Sprintf("targetRevision '%s' refers to a mutable ref", rev), types.SeverityError))
	}
	if wildcardPattern.MatchString(rev) || semverWildcard.MatchString(rev) {
		findings = append(findings, builder.NewFinding(fmt.Sprintf("targetRevision '%s' contains wildcard; prefer exact tag", rev), types.SeverityWarn))
	}
	return findings
}

func ruleProjectNotDefault() Rule {
	meta := types.RuleMetadata{
		ID:              "AR002",
		Description:     "Applications must target a non-default project",
		DefaultSeverity: types.SeverityError,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication, types.ResourceKindApplicationSet},
		Category:        "security",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return true },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			project := getString(m.Object, "spec", "project")
			if strings.TrimSpace(project) == "" {
				builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
				return []types.Finding{builder.NewFinding("spec.project is empty; specify a project to scope access", types.SeverityError)}
			}
			if project == "default" {
				builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
				return []types.Finding{builder.NewFinding("spec.project should not be 'default'", types.SeverityError)}
			}
			return nil
		},
	}
}

func ruleDestinationNamespace() Rule {
	meta := types.RuleMetadata{
		ID:              "AR003",
		Description:     "Destination namespace must be declared for namespace-scoped applications",
		DefaultSeverity: types.SeverityError,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
		Category:        "safety",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return m.Kind == string(types.ResourceKindApplication) },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			dest := getMap(m.Object, "spec", "destination")
			ns := strings.TrimSpace(getStringMap(dest, "namespace"))
			server := strings.TrimSpace(getStringMap(dest, "server"))
			name := strings.TrimSpace(getStringMap(dest, "name"))
			if ns == "" && name == "" && server != "https://kubernetes.default.svc" && server != "" {
				// cluster destination explicit, allow
				return nil
			}
			if ns == "" {
				builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
				return []types.Finding{builder.NewFinding("spec.destination.namespace is required", types.SeverityError)}
			}
			return nil
		},
	}
}

func ruleSyncPolicyExplicit() Rule {
	meta := types.RuleMetadata{
		ID:              "AR004",
		Description:     "Applications should declare syncPolicy automated or manual",
		DefaultSeverity: types.SeverityWarn,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
		Category:        "operations",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return m.Kind == string(types.ResourceKindApplication) },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			policy := getMap(m.Object, "spec", "syncPolicy")
			if len(policy) == 0 {
				builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
				return []types.Finding{builder.NewFinding("spec.syncPolicy not set; decide on automated or manual sync", types.SeverityWarn)}
			}
			return nil
		},
	}
}

func ruleSyncPolicyAutomatedSafety() Rule {
	meta := types.RuleMetadata{
		ID:              "AR005",
		Description:     "Automated sync should enable prune and selfHeal when required",
		DefaultSeverity: types.SeverityWarn,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
		Category:        "operations",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return true },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			auto := getMap(m.Object, "spec", "syncPolicy", "automated")
			if len(auto) == 0 {
				return nil
			}
			findings := []types.Finding{}
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			prune, ok := auto["prune"].(bool)
			if !ok || !prune {
				findings = append(findings, builder.NewFinding("Automated sync without prune may leave orphaned resources", types.SeverityWarn))
			}
			selfHeal, ok := auto["selfHeal"].(bool)
			if !ok || !selfHeal {
				findings = append(findings, builder.NewFinding("Automated sync without selfHeal may drift", types.SeverityWarn))
			}
			return findings
		},
	}
}

func ruleFinalizerAware() Rule {
	meta := types.RuleMetadata{
		ID:              "AR006",
		Description:     "Applications should explicitly opt-in/out of finalizers",
		DefaultSeverity: types.SeverityInfo,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
		Category:        "safety",
		Enabled:         true,
	}
	finalizerValue := "resources-finalizer.argocd.argoproj.io"
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return true },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			list := getSlice(m.Object, "metadata", "finalizers")
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			for _, item := range list {
				if str, ok := item.(string); ok && str == finalizerValue {
					return []types.Finding{builder.NewFinding("Finalizer resources-finalizer.argocd.argoproj.io enabled", types.SeverityInfo)}
				}
			}
			return []types.Finding{builder.NewFinding("Application deletes cascaded resources immediately; add resources-finalizer.argocd.argoproj.io if needed", types.SeverityWarn)}
		},
	}
}

func ruleIgnoreDifferencesScoped() Rule {
	meta := types.RuleMetadata{
		ID:              "AR007",
		Description:     "ignoreDifferences entries must be tightly scoped",
		DefaultSeverity: types.SeverityWarn,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
		Category:        "drift",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return true },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			items := getSlice(m.Object, "spec", "ignoreDifferences")
			if len(items) == 0 {
				return nil
			}
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			var findings []types.Finding
			for _, raw := range items {
				entry, ok := raw.(map[string]interface{})
				if !ok {
					findings = append(findings, builder.NewFinding("ignoreDifferences entry is not an object", types.SeverityWarn))
					continue
				}
				kind := getStringMap(entry, "kind")
				if kind == "*" {
					findings = append(findings, builder.NewFinding("ignoreDifferences with kind '*' disables drift detection for all kinds", types.SeverityError))
				}
				jsonPointers := getSlice(entry, "jsonPointers")
				jqPaths := getSlice(entry, "jqPathExpressions")
				if len(jsonPointers) == 0 && len(jqPaths) == 0 {
					findings = append(findings, builder.NewFinding("ignoreDifferences entry lacks jsonPointers or jqPathExpressions", types.SeverityWarn))
				}
			}
			return findings
		},
	}
}

func ruleApplicationSetGoTemplateOptions() Rule {
	meta := types.RuleMetadata{
		ID:              "AR008",
		Description:     "ApplicationSets should enable missingkey=error to surface template issues",
		DefaultSeverity: types.SeverityWarn,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplicationSet},
		Category:        "best-practice",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return m.Kind == string(types.ResourceKindApplicationSet) },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			options := getSlice(m.Object, "spec", "goTemplateOptions")
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			if len(options) == 0 {
				return []types.Finding{builder.NewFinding("spec.goTemplateOptions missing; include 'missingkey=error'", types.SeverityWarn)}
			}
			for _, opt := range options {
				if str, ok := opt.(string); ok && str == "missingkey=error" {
					return nil
				}
			}
			return []types.Finding{builder.NewFinding("Add 'missingkey=error' to spec.goTemplateOptions", types.SeverityWarn)}
		},
	}
}

func ruleSourceConsistency() Rule {
	meta := types.RuleMetadata{
		ID:              "AR009",
		Description:     "Application sources must be defined consistently",
		DefaultSeverity: types.SeverityError,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
		Category:        "configuration",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return m.Kind == string(types.ResourceKindApplication) },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			var findings []types.Finding
			source := getMap(m.Object, "spec", "source")
			sources := getSlice(m.Object, "spec", "sources")
			if len(source) != 0 && len(sources) != 0 {
				findings = append(findings, builder.NewFinding("Use either spec.source or spec.sources, not both", types.SeverityError))
			}
			if len(source) != 0 {
				findings = append(findings, validateSource(builder, source)...)
			}
			for _, raw := range sources {
				if src, ok := raw.(map[string]interface{}); ok {
					findings = append(findings, validateSource(builder, src)...)
				}
			}
			return findings
		},
	}
}

func validateSource(builder types.FindingBuilder, src map[string]interface{}) []types.Finding {
	var findings []types.Finding
	repo := strings.TrimSpace(getStringMap(src, "repoURL"))
	if repo == "" {
		findings = append(findings, builder.NewFinding("source.repoURL is required", types.SeverityError))
	}
	pathVal := strings.TrimSpace(getStringMap(src, "path"))
	chartVal := strings.TrimSpace(getStringMap(src, "chart"))
	if pathVal != "" && chartVal != "" {
		findings = append(findings, builder.NewFinding("source.path and source.chart cannot both be set", types.SeverityError))
	}
	if pathVal == "" && chartVal == "" {
		findings = append(findings, builder.NewFinding("provide source.path for Git or source.chart for Helm", types.SeverityWarn))
	}
	return findings
}

func ruleRecommendedLabels() Rule {
	meta := types.RuleMetadata{
		ID:              "AR010",
		Description:     "Metadata should include app.kubernetes.io/name label",
		DefaultSeverity: types.SeverityInfo,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication, types.ResourceKindApplicationSet},
		Category:        "advisory",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return true },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			labels := getMap(m.Object, "metadata", "labels")
			if _, ok := labels["app.kubernetes.io/name"]; ok {
				return nil
			}
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			return []types.Finding{builder.NewFinding("Add app.kubernetes.io/name label to metadata", types.SeverityInfo)}
		},
	}
}

// Helpers
func getMap(obj map[string]interface{}, path ...string) map[string]interface{} {
	current := obj
	for _, key := range path {
		if current == nil {
			return map[string]interface{}{}
		}
		next, _ := current[key].(map[string]interface{})
		current = next
	}
	if current == nil {
		return map[string]interface{}{}
	}
	return current
}

func getSlice(obj map[string]interface{}, path ...string) []interface{} {
	current := obj
	for i, key := range path {
		if current == nil {
			return nil
		}
		if i == len(path)-1 {
			if slice, ok := current[key].([]interface{}); ok {
				return slice
			}
			return nil
		}
		next, _ := current[key].(map[string]interface{})
		current = next
	}
	return nil
}

func getStringMap(obj map[string]interface{}, key string) string {
	if obj == nil {
		return ""
	}
	if v, ok := obj[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

func getString(obj map[string]interface{}, path ...string) string {
	current := obj
	for i, key := range path {
		if current == nil {
			return ""
		}
		if i == len(path)-1 {
			if v, ok := current[key]; ok {
				if str, ok := v.(string); ok {
					return str
				}
			}
			return ""
		}
		next, _ := current[key].(map[string]interface{})
		current = next
	}
	return ""
}

// UniqueNameFindings flags duplicate Application names across manifests.
func UniqueNameFindings(ctx *Context) []types.Finding {
	meta := types.RuleMetadata{
		ID:              "AR011",
		Description:     "Application names must be unique across manifests",
		DefaultSeverity: types.SeverityError,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication},
		Category:        "consistency",
		Enabled:         true,
	}
	var findings []types.Finding
	seen := map[string][]*manifest.Manifest{}
	for _, m := range ctx.Manifests {
		if m.Kind != string(types.ResourceKindApplication) {
			continue
		}
		seen[m.Name] = append(seen[m.Name], m)
	}
	for name, manifests := range seen {
		if len(manifests) <= 1 {
			continue
		}
		for _, m := range manifests {
			cfg, err := ctx.Config.Resolve(meta, m.FilePath)
			if err != nil {
				cfg = types.ConfiguredRule{Metadata: meta, Severity: meta.DefaultSeverity, Enabled: meta.Enabled}
			}
			if !cfg.Enabled {
				continue
			}
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			findings = append(findings, builder.NewFinding(fmt.Sprintf("Application name '%s' is declared in multiple manifests", name), types.SeverityError))
		}
	}
	return findings
}
