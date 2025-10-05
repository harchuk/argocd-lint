package rule

import (
	"fmt"
	"net/url"
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
		ruleRepoURLPolicy(),
		ruleProjectAccess(),
		ruleAppProjectGuardrails(),
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
		finding := builder.NewFinding("targetRevision is empty; pin to a tag or commit", types.SeverityWarn)
		finding.Suggestions = []types.Suggestion{
			{
				Title:       "Pin targetRevision to an immutable reference",
				Description: "Set targetRevision to a specific tag or commit to avoid drifting deployments.",
				Patch:       "targetRevision: <tag-or-commit>",
				Path:        "$.spec.source.targetRevision",
			},
		}
		findings = append(findings, finding)
		return findings
	}
	if rev == "HEAD" {
		finding := builder.NewFinding("targetRevision 'HEAD' is not immutable", types.SeverityError)
		finding.Suggestions = []types.Suggestion{
			{
				Title:       "Replace HEAD with immutable revision",
				Description: "Pin targetRevision to a stable tag or commit instead of HEAD.",
				Patch:       "targetRevision: <tag-or-commit>",
				Path:        "$.spec.source.targetRevision",
			},
		}
		findings = append(findings, finding)
		return findings
	}
	if floatingRevisionPattern.MatchString(rev) {
		finding := builder.NewFinding(fmt.Sprintf("targetRevision '%s' refers to a mutable ref", rev), types.SeverityError)
		finding.Suggestions = []types.Suggestion{
			{
				Title:       "Pin targetRevision to an immutable reference",
				Description: "Use a specific tag or commit instead of a floating branch name.",
				Patch:       "targetRevision: <tag-or-commit>",
				Path:        "$.spec.source.targetRevision",
			},
		}
		findings = append(findings, finding)
	}
	if wildcardPattern.MatchString(rev) || semverWildcard.MatchString(rev) {
		finding := builder.NewFinding(fmt.Sprintf("targetRevision '%s' contains wildcard; prefer exact tag", rev), types.SeverityWarn)
		finding.Suggestions = []types.Suggestion{
			{
				Title:       "Replace wildcard with exact revision",
				Description: "Set targetRevision to a precise tag or commit to ensure deterministic syncs.",
				Patch:       "targetRevision: <tag-or-commit>",
				Path:        "$.spec.source.targetRevision",
			},
		}
		findings = append(findings, finding)
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
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			checkValue := func(project string) []types.Finding {
				project = strings.TrimSpace(project)
				if project == "" {
					return []types.Finding{builder.NewFinding("spec.project is empty; specify a project to scope access", types.SeverityError)}
				}
				if project == "default" {
					return []types.Finding{builder.NewFinding("spec.project should not be 'default'", types.SeverityError)}
				}
				return nil
			}

			switch m.Kind {
			case string(types.ResourceKindApplication):
				return checkValue(getString(m.Object, "spec", "project"))
			case string(types.ResourceKindApplicationSet):
				candidates := []string{
					getString(m.Object, "spec", "project"),
					getString(m.Object, "spec", "template", "spec", "project"),
					getString(m.Object, "spec", "applicationSpec", "project"),
					getString(m.Object, "spec", "applicationCore", "project"),
				}
				for _, candidate := range candidates {
					candidate = strings.TrimSpace(candidate)
					if candidate == "" {
						continue
					}
					return checkValue(candidate)
				}
				return []types.Finding{builder.NewFinding("ApplicationSet template lacks project assignment", types.SeverityError)}
			default:
				return nil
			}
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
				finding := builder.NewFinding("spec.goTemplateOptions missing; include 'missingkey=error'", types.SeverityWarn)
				finding.Suggestions = []types.Suggestion{
					{
						Title:       "Add missingkey=error option",
						Description: "Ensure template rendering fails fast when a variable is absent.",
						Patch:       "spec:\n  goTemplateOptions:\n    - missingkey=error",
						Path:        "$.spec.goTemplateOptions",
					},
				}
				return []types.Finding{finding}
			}
			for _, opt := range options {
				if str, ok := opt.(string); ok && str == "missingkey=error" {
					return nil
				}
			}
			finding := builder.NewFinding("Add 'missingkey=error' to spec.goTemplateOptions", types.SeverityWarn)
			finding.Suggestions = []types.Suggestion{
				{
					Title:       "Append missingkey=error to goTemplateOptions",
					Description: "Include missingkey=error so template issues surface during render.",
					Patch:       "- missingkey=error",
					Path:        "$.spec.goTemplateOptions[]",
				},
			}
			return []types.Finding{finding}
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
				findings = append(findings, validateSource(builder, source, "$.spec.source")...)
			}
			for _, raw := range sources {
				if src, ok := raw.(map[string]interface{}); ok {
					findings = append(findings, validateSource(builder, src, "$.spec.sources[]")...)
				}
			}
			return findings
		},
	}
}

func validateSource(builder types.FindingBuilder, src map[string]interface{}, sourcePath string) []types.Finding {
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
	if directory := getMap(src, "directory"); len(directory) > 0 {
		if helm := getMap(src, "helm"); len(helm) > 0 {
			finding := builder.NewFinding("directory and helm options conflict in Application source", types.SeverityError)
			finding.Suggestions = []types.Suggestion{
				{
					Title:       "Remove mutually exclusive source sections",
					Description: "Use either the directory generator or Helm configuration for a source, not both.",
					Patch:       "# remove either directory: or helm: block",
					Path:        sourcePath,
				},
			}
			findings = append(findings, finding)
		}
		if kustomize := getMap(src, "kustomize"); len(kustomize) > 0 {
			finding := builder.NewFinding("directory and kustomize cannot be combined in one source", types.SeverityError)
			finding.Suggestions = []types.Suggestion{
				{
					Title:       "Split directory and kustomize sources",
					Description: "Define separate sources for raw directories and kustomize overlays.",
					Patch:       "# move kustomize: block to a dedicated source entry",
					Path:        sourcePath,
				},
			}
			findings = append(findings, finding)
		}
	}
	if kustomize := getMap(src, "kustomize"); len(kustomize) > 0 {
		if helm := getMap(src, "helm"); len(helm) > 0 {
			finding := builder.NewFinding("helm and kustomize options conflict; choose one renderer", types.SeverityError)
			finding.Suggestions = []types.Suggestion{
				{
					Title:       "Separate Helm and Kustomize configurations",
					Description: "Use distinct sources when mixing Helm charts and Kustomize overlays.",
					Patch:       "# move helm: block to a dedicated source entry",
					Path:        sourcePath,
				},
			}
			findings = append(findings, finding)
		}
	}
	return findings
}

func ruleRecommendedLabels() Rule {
	meta := types.RuleMetadata{
		ID:              "AR010",
		Description:     "Metadata should include app.kubernetes.io/name label",
		DefaultSeverity: types.SeverityInfo,
		AppliesTo: []types.ResourceKind{
			types.ResourceKindApplication,
			types.ResourceKindApplicationSet,
			types.ResourceKindAppProject,
		},
		Category: "advisory",
		Enabled:  true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return true },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			labels := getMap(m.Object, "metadata", "labels")
			annotations := getMap(m.Object, "metadata", "annotations")
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			var findings []types.Finding
			if _, ok := labels["app.kubernetes.io/name"]; !ok {
				finding := builder.NewFinding("Add app.kubernetes.io/name label to metadata", types.SeverityInfo)
				finding.Suggestions = []types.Suggestion{
					{
						Title:       "Set app.kubernetes.io/name label",
						Description: "Use the canonical application name for consistent ownership.",
						Patch:       "metadata:\n  labels:\n    app.kubernetes.io/name: <name>",
						Path:        "$.metadata.labels",
					},
				}
				findings = append(findings, finding)
			}
			if managedBy, ok := labels["app.kubernetes.io/managed-by"]; !ok || managedBy != "argocd" {
				finding := builder.NewFinding("Set app.kubernetes.io/managed-by=argocd label", types.SeverityInfo)
				finding.Suggestions = []types.Suggestion{
					{
						Title:       "Label resources as managed by Argo CD",
						Description: "Set app.kubernetes.io/managed-by to 'argocd' for tooling consistency.",
						Patch:       "metadata:\n  labels:\n    app.kubernetes.io/managed-by: argocd",
						Path:        "$.metadata.labels",
					},
				}
				findings = append(findings, finding)
			}
			if _, ok := labels["argocd.argoproj.io/owner"]; !ok {
				if _, annOk := annotations["argocd.argoproj.io/owner"]; !annOk {
					finding := builder.NewFinding("Annotate owner via argocd.argoproj.io/owner", types.SeverityInfo)
					finding.Suggestions = []types.Suggestion{
						{
							Title:       "Specify responsible team",
							Description: "Add argocd.argoproj.io/owner label or annotation to document ownership.",
							Patch:       "metadata:\n  annotations:\n    argocd.argoproj.io/owner: <team>",
							Path:        "$.metadata.annotations",
						},
					}
					findings = append(findings, finding)
				}
			}
			return findings
		},
	}
}

func ruleRepoURLPolicy() Rule {
	meta := types.RuleMetadata{
		ID:              "AR013",
		Description:     "source.repoURL must match approved protocols and domains",
		DefaultSeverity: types.SeverityError,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication, types.ResourceKindApplicationSet},
		Category:        "security",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies: func(m *manifest.Manifest) bool {
			return m.Kind == string(types.ResourceKindApplication) || m.Kind == string(types.ResourceKindApplicationSet)
		},
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			policies := ctx.Config.Policies
			allowedProtocols := normalizeList(policies.AllowedRepoURLProtocols)
			allowedDomains := normalizeList(policies.AllowedRepoURLDomains)
			if len(allowedProtocols) == 0 && len(allowedDomains) == 0 {
				return nil
			}
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			var findings []types.Finding
			for _, repo := range collectRepoURLs(m) {
				repo = strings.TrimSpace(repo)
				if repo == "" {
					continue
				}
				scheme, host := parseRepoURL(repo)
				if len(allowedProtocols) > 0 && scheme != "" && !stringAllowed(scheme, allowedProtocols) {
					msg := fmt.Sprintf("source.repoURL '%s' uses protocol '%s' (allowed: %s)", repo, scheme, strings.Join(allowedProtocols, ","))
					findings = append(findings, builder.NewFinding(msg, cfg.Severity))
					continue
				}
				if len(allowedProtocols) > 0 && scheme == "" && !stringAllowed("", allowedProtocols) {
					msg := fmt.Sprintf("source.repoURL '%s' omits a protocol (allowed: %s)", repo, strings.Join(allowedProtocols, ","))
					findings = append(findings, builder.NewFinding(msg, cfg.Severity))
				}
				if len(allowedDomains) > 0 {
					if host == "" {
						msg := fmt.Sprintf("source.repoURL '%s' has no host; cannot validate against domains (%s)", repo, strings.Join(allowedDomains, ","))
						findings = append(findings, builder.NewFinding(msg, cfg.Severity))
						continue
					}
					if !domainAllowed(host, allowedDomains) {
						msg := fmt.Sprintf("source.repoURL '%s' resolves to '%s' not allowed (%s)", repo, host, strings.Join(allowedDomains, ","))
						findings = append(findings, builder.NewFinding(msg, cfg.Severity))
					}
				}
			}
			return findings
		},
	}
}

func ruleProjectAccess() Rule {
	meta := types.RuleMetadata{
		ID:              "AR014",
		Description:     "Applications must reference existing AppProjects and stay within declared access scopes",
		DefaultSeverity: types.SeverityError,
		AppliesTo:       []types.ResourceKind{types.ResourceKindApplication, types.ResourceKindApplicationSet},
		Category:        "governance",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies: func(m *manifest.Manifest) bool {
			return m.Kind == string(types.ResourceKindApplication) || m.Kind == string(types.ResourceKindApplicationSet)
		},
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			projects := collectAppProjects(ctx.Manifests)
			if len(projects) == 0 {
				return nil
			}
			projectName, repos, dest := manifestProjectInfo(m)
			if projectName == "" {
				return nil
			}
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			policy, ok := projects[projectName]
			if !ok {
				msg := fmt.Sprintf("AppProject '%s' not found; add manifest or adjust spec.project", projectName)
				return []types.Finding{builder.NewFinding(msg, cfg.Severity)}
			}
			var findings []types.Finding
			for _, repo := range repos {
				repo = strings.TrimSpace(repo)
				if repo == "" {
					continue
				}
				if !repoAllowedByProject(repo, policy.SourceRepos) {
					msg := fmt.Sprintf("source.repoURL '%s' is not permitted by AppProject '%s'", repo, projectName)
					findings = append(findings, builder.NewFinding(msg, cfg.Severity))
				}
			}
			if dest != nil {
				if !destinationAllowedByProject(*dest, policy.Destinations) {
					msg := fmt.Sprintf("destination not permitted by AppProject '%s'", projectName)
					findings = append(findings, builder.NewFinding(msg, cfg.Severity))
				}
			}
			return findings
		},
	}
}

func ruleAppProjectGuardrails() Rule {
	meta := types.RuleMetadata{
		ID:              "AR012",
		Description:     "AppProjects should scope allowed sources and destinations",
		DefaultSeverity: types.SeverityWarn,
		AppliesTo:       []types.ResourceKind{types.ResourceKindAppProject},
		Category:        "governance",
		Enabled:         true,
	}
	return Rule{
		Metadata: meta,
		Applies:  func(m *manifest.Manifest) bool { return m.Kind == string(types.ResourceKindAppProject) },
		Check: func(m *manifest.Manifest, ctx *Context, cfg types.ConfiguredRule) []types.Finding {
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			var findings []types.Finding

			namespaces := getSlice(m.Object, "spec", "sourceNamespaces")
			if len(namespaces) == 0 {
				finding := builder.NewFinding("spec.sourceNamespaces is empty; restrict allowed namespaces", types.SeverityWarn)
				finding.Suggestions = []types.Suggestion{
					{
						Title:       "Define allowed source namespaces",
						Description: "List namespaces that AppProject members may source from.",
						Patch:       "spec:\n  sourceNamespaces:\n    - apps",
						Path:        "$.spec.sourceNamespaces",
					},
				}
				findings = append(findings, finding)
			} else {
				for _, raw := range namespaces {
					if ns, ok := raw.(string); ok && ns == "*" {
						finding := builder.NewFinding("spec.sourceNamespaces uses wildcard '*'; tighten namespace scope", types.SeverityWarn)
						finding.Suggestions = []types.Suggestion{
							{
								Title:       "Replace wildcard namespace",
								Description: "Set explicit namespace names in sourceNamespaces.",
								Patch:       "- <namespace>",
								Path:        "$.spec.sourceNamespaces[]",
							},
						}
						findings = append(findings, finding)
					}
				}
			}

			repos := getSlice(m.Object, "spec", "sourceRepos")
			for _, raw := range repos {
				if repo, ok := raw.(string); ok {
					if strings.ContainsAny(repo, "*") {
						finding := builder.NewFinding("spec.sourceRepos entry allows wildcard; pin repositories explicitly", types.SeverityWarn)
						finding.Suggestions = []types.Suggestion{
							{
								Title:       "List exact repository URL",
								Description: "Replace wildcard entries with explicit repository URLs.",
								Patch:       "- https://git.example.com/org/repo.git",
								Path:        "$.spec.sourceRepos[]",
							},
						}
						findings = append(findings, finding)
					}
				}
			}

			destinations := getSlice(m.Object, "spec", "destinations")
			if len(destinations) == 0 {
				finding := builder.NewFinding("spec.destinations is empty; declare allowed target clusters/namespaces", types.SeverityWarn)
				finding.Suggestions = []types.Suggestion{
					{
						Title:       "Add destination entries",
						Description: "List the clusters and namespaces AppProject may deploy to.",
						Patch:       "spec:\n  destinations:\n    - namespace: apps\n      server: https://kubernetes.default.svc",
						Path:        "$.spec.destinations",
					},
				}
				findings = append(findings, finding)
			}
			for _, raw := range destinations {
				dest, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				namespace := strings.TrimSpace(getStringMap(dest, "namespace"))
				if namespace == "" {
					finding := builder.NewFinding("Destination missing namespace; specify exact namespace or '*'", types.SeverityWarn)
					finding.Suggestions = []types.Suggestion{
						{
							Title:       "Set destination namespace",
							Description: "Declare the namespace this destination permits.",
							Patch:       "namespace: <namespace>",
							Path:        "$.spec.destinations[]",
						},
					}
					findings = append(findings, finding)
				} else if namespace == "*" {
					finding := builder.NewFinding("Destination namespace is wildcard '*'; prefer explicit namespaces", types.SeverityWarn)
					finding.Suggestions = []types.Suggestion{
						{
							Title:       "Replace wildcard namespace",
							Description: "Restrict destinations to known namespaces.",
							Patch:       "namespace: <namespace>",
							Path:        "$.spec.destinations[]",
						},
					}
					findings = append(findings, finding)
				}
				server := strings.TrimSpace(getStringMap(dest, "server"))
				name := strings.TrimSpace(getStringMap(dest, "name"))
				if server == "" && name == "" {
					finding := builder.NewFinding("Destination missing cluster selector; set server or name", types.SeverityWarn)
					finding.Suggestions = []types.Suggestion{
						{
							Title:       "Provide cluster identifier",
							Description: "Specify destination.server URL or destination.name for cluster selection.",
							Patch:       "server: https://kubernetes.default.svc",
							Path:        "$.spec.destinations[]",
						},
					}
					findings = append(findings, finding)
				} else if server == "*" {
					finding := builder.NewFinding("Destination server wildcard '*'; scope clusters explicitly", types.SeverityWarn)
					finding.Suggestions = []types.Suggestion{
						{
							Title:       "Replace wildcard server",
							Description: "Use explicit destination.name or destination.server entries.",
							Patch:       "server: https://kubernetes.default.svc",
							Path:        "$.spec.destinations[]",
						},
					}
					findings = append(findings, finding)
				}
			}

			return findings
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

func normalizeList(values []string) []string {
	var out []string
	for _, v := range values {
		trimmed := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(v, ":"), "://")))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func collectRepoURLs(m *manifest.Manifest) []string {
	set := map[string]struct{}{}
	add := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		set[url] = struct{}{}
	}
	switch m.Kind {
	case string(types.ResourceKindApplication):
		source := getMap(m.Object, "spec", "source")
		add(getStringMap(source, "repoURL"))
		for _, raw := range getSlice(m.Object, "spec", "sources") {
			if src, ok := raw.(map[string]interface{}); ok {
				add(getStringMap(src, "repoURL"))
			}
		}
	case string(types.ResourceKindApplicationSet):
		templateSpec := getMap(m.Object, "spec", "template", "spec")
		src := getMap(templateSpec, "source")
		add(getStringMap(src, "repoURL"))
		for _, raw := range getSlice(templateSpec, "sources") {
			if srcMap, ok := raw.(map[string]interface{}); ok {
				add(getStringMap(srcMap, "repoURL"))
			}
		}
	}
	urls := make([]string, 0, len(set))
	for url := range set {
		urls = append(urls, url)
	}
	return urls
}

func parseRepoURL(raw string) (scheme string, host string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Host != "" {
		return strings.ToLower(parsed.Scheme), strings.ToLower(parsed.Hostname())
	}
	withoutUser := trimmed
	if at := strings.LastIndex(trimmed, "@"); at != -1 {
		withoutUser = trimmed[at+1:]
	}
	if idx := strings.Index(withoutUser, ":"); idx != -1 {
		return "", strings.ToLower(withoutUser[:idx])
	}
	if strings.HasPrefix(withoutUser, "//") {
		return "", strings.ToLower(strings.TrimPrefix(withoutUser, "//"))
	}
	return "", strings.ToLower(withoutUser)
}

func stringAllowed(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if item == "*" {
			return true
		}
		if value == item {
			return true
		}
	}
	return false
}

func domainAllowed(domain string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if globMatch(pattern, domain) {
			return true
		}
	}
	return false
}

type projectPolicy struct {
	SourceRepos  []string
	Destinations []projectDestination
}

type projectDestination struct {
	Server    string
	Name      string
	Namespace string
}

func collectAppProjects(manifests []*manifest.Manifest) map[string]projectPolicy {
	projects := make(map[string]projectPolicy)
	for _, m := range manifests {
		if m == nil || m.Kind != string(types.ResourceKindAppProject) {
			continue
		}
		spec := getMap(m.Object, "spec")
		repos := sliceToStrings(getSlice(spec, "sourceRepos"))
		if len(repos) == 0 {
			repos = []string{"*"}
		}
		dests := make([]projectDestination, 0)
		for _, raw := range getSlice(spec, "destinations") {
			if dest, ok := raw.(map[string]interface{}); ok {
				dests = append(dests, projectDestination{
					Server:    strings.TrimSpace(getStringMap(dest, "server")),
					Name:      strings.TrimSpace(getStringMap(dest, "name")),
					Namespace: strings.TrimSpace(getStringMap(dest, "namespace")),
				})
			}
		}
		if len(dests) == 0 {
			dests = append(dests, projectDestination{Server: "*", Namespace: "*", Name: "*"})
		}
		projects[m.Name] = projectPolicy{SourceRepos: repos, Destinations: dests}
	}
	return projects
}

func sliceToStrings(items []interface{}) []string {
	var out []string
	for _, item := range items {
		if str, ok := item.(string); ok {
			trimmed := strings.TrimSpace(str)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func manifestProjectInfo(m *manifest.Manifest) (string, []string, *projectDestination) {
	switch m.Kind {
	case string(types.ResourceKindApplication):
		project := strings.TrimSpace(getString(m.Object, "spec", "project"))
		repos := collectRepoURLs(m)
		destMap := getMap(m.Object, "spec", "destination")
		dest := destinationFromMap(destMap)
		return project, repos, dest
	case string(types.ResourceKindApplicationSet):
		project := appSetProjectName(m)
		templateSpec := getMap(m.Object, "spec", "template", "spec")
		repos := collectRepoURLs(m)
		dest := destinationFromMap(getMap(templateSpec, "destination"))
		return project, repos, dest
	default:
		return "", nil, nil
	}
}

func appSetProjectName(m *manifest.Manifest) string {
	candidates := []string{
		getString(m.Object, "spec", "project"),
		getString(m.Object, "spec", "template", "spec", "project"),
		getString(m.Object, "spec", "applicationSpec", "project"),
		getString(m.Object, "spec", "applicationCore", "project"),
	}
	for _, c := range candidates {
		trimmed := strings.TrimSpace(c)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func destinationFromMap(dest map[string]interface{}) *projectDestination {
	if len(dest) == 0 {
		return nil
	}
	return &projectDestination{
		Server:    strings.TrimSpace(getStringMap(dest, "server")),
		Name:      strings.TrimSpace(getStringMap(dest, "name")),
		Namespace: strings.TrimSpace(getStringMap(dest, "namespace")),
	}
}

func repoAllowedByProject(repo string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	repoLower := strings.ToLower(repo)
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if globMatch(pattern, repoLower) {
			return true
		}
	}
	return false
}

func destinationAllowedByProject(dest projectDestination, allowed []projectDestination) bool {
	for _, candidate := range allowed {
		if matchDestinationField(dest.Namespace, candidate.Namespace) &&
			matchDestinationField(dest.Server, candidate.Server) &&
			matchDestinationField(dest.Name, candidate.Name) {
			return true
		}
	}
	return false
}

func matchDestinationField(value, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return true
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return globMatch(strings.ToLower(pattern), strings.ToLower(value))
}

func globMatch(pattern, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	var builder strings.Builder
	for _, r := range pattern {
		switch r {
		case '*':
			builder.WriteString(".*")
		case '?':
			builder.WriteString(".")
		default:
			builder.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	regex := "^" + builder.String() + "$"
	matched, err := regexp.MatchString(regex, value)
	if err != nil {
		return false
	}
	return matched
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
