package lint

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/loader"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/internal/render"
	"github.com/argocd-lint/argocd-lint/internal/rule"
	"github.com/argocd-lint/argocd-lint/internal/schema"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

// Options controls lint execution.
type Options struct {
	Target                 string
	IncludeApplications    bool
	IncludeApplicationSets bool
	Config                 config.Config
	WorkingDir             string
	Render                 render.Options
}

// Report is the lint result collection.
type Report struct {
	Findings  []types.Finding
	RuleIndex map[string]types.RuleMetadata
}

// Runner orchestrates parsing, validation, and rule checks.
type Runner struct {
	parser  manifest.Parser
	rules   []rule.Rule
	schema  *schema.Validator
	cfg     config.Config
	workdir string
}

// NewRunner creates a Runner with the provided configuration.
func NewRunner(cfg config.Config, workdir string) (*Runner, error) {
	validator, err := schema.NewValidator()
	if err != nil {
		return nil, err
	}
	return &Runner{
		parser:  manifest.Parser{},
		rules:   rule.DefaultRules(),
		schema:  validator,
		cfg:     cfg,
		workdir: workdir,
	}, nil
}

// Run executes the linting workflow.
func (r *Runner) Run(opts Options) (Report, error) {
	if opts.Target == "" {
		return Report{}, fmt.Errorf("no target specified")
	}
	if !opts.IncludeApplications && !opts.IncludeApplicationSets {
		opts.IncludeApplications = true
		opts.IncludeApplicationSets = true
	}
	files, err := loader.DiscoverFiles(opts.Target)
	if err != nil {
		return Report{}, err
	}
	var manifests []*manifest.Manifest
	for _, file := range files {
		docs, err := r.parser.ParseFile(file)
		if err != nil {
			return Report{}, err
		}
		manifests = append(manifests, docs...)
	}
	included := make([]*manifest.Manifest, 0, len(manifests))
	for _, m := range manifests {
		if m == nil {
			continue
		}
		if includeManifest(m, opts.IncludeApplications, opts.IncludeApplicationSets) {
			if r.workdir != "" {
				if rel, err := filepath.Rel(r.workdir, m.FilePath); err == nil {
					m.FilePath = rel
				}
			}
			included = append(included, m)
		}
	}
	ctx := &rule.Context{Config: r.cfg, Manifests: included}
	findings := make([]types.Finding, 0, len(included))
	ruleIndex := map[string]types.RuleMetadata{}
	for _, meta := range r.schema.Metadata() {
		ruleIndex[meta.ID] = meta
	}
	for _, rl := range r.rules {
		ruleIndex[rl.Metadata.ID] = rl.Metadata
	}

	var renderer *render.Renderer
	if opts.Render.Enabled {
		var err error
		renderer, err = render.NewRenderer(r.cfg, opts.Render)
		if err != nil {
			return Report{}, err
		}
		for _, meta := range renderer.Metadata() {
			ruleIndex[meta.ID] = meta
		}
	}

	for _, m := range included {
		schemaFindings, err := r.schema.Validate(m)
		if err != nil {
			return Report{}, err
		}
		findings = append(findings, schemaFindings...)

		if renderer != nil {
			renderFindings, err := renderer.Render(m)
			if err != nil {
				return Report{}, err
			}
			findings = append(findings, renderFindings...)
		}
	}

	for _, m := range included {
		for _, rl := range r.rules {
			if rl.Applies != nil && !rl.Applies(m) {
				continue
			}
			cfg, err := r.cfg.Resolve(rl.Metadata, m.FilePath)
			if err != nil {
				return Report{}, err
			}
			if !cfg.Enabled {
				continue
			}
			findings = append(findings, rl.Check(m, ctx, cfg)...)
		}
	}

	findings = append(findings, rule.UniqueNameFindings(ctx)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].FilePath == findings[j].FilePath {
			if findings[i].Line == findings[j].Line {
				if findings[i].RuleID == findings[j].RuleID {
					return findings[i].Message < findings[j].Message
				}
				return findings[i].RuleID < findings[j].RuleID
			}
			return findings[i].Line < findings[j].Line
		}
		return findings[i].FilePath < findings[j].FilePath
	})

	return Report{Findings: findings, RuleIndex: ruleIndex}, nil
}

func includeManifest(m *manifest.Manifest, apps, appsets bool) bool {
	switch m.Kind {
	case string(types.ResourceKindApplication):
		return apps
	case string(types.ResourceKindApplicationSet):
		return appsets
	default:
		return false
	}
}
