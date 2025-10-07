package lint

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/dryrun"
	"github.com/argocd-lint/argocd-lint/internal/loader"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/internal/render"
	"github.com/argocd-lint/argocd-lint/internal/rule"
	"github.com/argocd-lint/argocd-lint/internal/schema"
	"github.com/argocd-lint/argocd-lint/pkg/plugin"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

// Options controls lint execution.
type Options struct {
	Target                 string
	IncludeApplications    bool
	IncludeApplicationSets bool
	IncludeProjects        bool
	Config                 config.Config
	WorkingDir             string
	Render                 render.Options
	SeverityThreshold      string
	DryRun                 dryrun.Options
	MaxParallel            int
	Baseline               *Baseline
	BaselineAgingDays      int
}

// Report is the lint result collection.
type Report struct {
	Findings   []types.Finding
	RuleIndex  map[string]types.RuleMetadata
	Suppressed []types.Finding
}

// Runner orchestrates parsing, validation, and rule checks.
type Runner struct {
	parser        manifest.Parser
	rules         []rule.Rule
	schema        *schema.Validator
	cfg           config.Config
	workdir       string
	plugins       *plugin.Registry
	schemaVersion string
}

// NewRunner creates a Runner with the provided configuration.
func NewRunner(cfg config.Config, workdir, schemaVersion string) (*Runner, error) {
	validator, err := schema.NewValidator(schemaVersion)
	if err != nil {
		return nil, err
	}
	return &Runner{
		parser:        manifest.Parser{},
		rules:         rule.DefaultRules(),
		schema:        validator,
		cfg:           cfg,
		workdir:       workdir,
		plugins:       plugin.NewRegistry(),
		schemaVersion: schemaVersion,
	}, nil
}

// RegisterPlugins registers additional rule plugins.
func (r *Runner) RegisterPlugins(plugins ...plugin.RulePlugin) {
	if r.plugins == nil {
		r.plugins = plugin.NewRegistry()
	}
	r.plugins.Register(plugins...)
}

// Run executes the linting workflow.
func (r *Runner) Run(opts Options) (Report, error) {
	if opts.Target == "" {
		return Report{}, fmt.Errorf("no target specified")
	}
	if !opts.IncludeApplications && !opts.IncludeApplicationSets && !opts.IncludeProjects {
		opts.IncludeApplications = true
		opts.IncludeApplicationSets = true
		opts.IncludeProjects = true
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
		if includeManifest(m, opts.IncludeApplications, opts.IncludeApplicationSets, opts.IncludeProjects) {
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
	ruleIndex[waiverExpiredMeta.ID] = waiverExpiredMeta
	ruleIndex[waiverInvalidMeta.ID] = waiverInvalidMeta
	ruleIndex[baselineAgedMeta.ID] = baselineAgedMeta
	if r.plugins != nil {
		for _, plug := range r.plugins.Plugins() {
			meta := plug.Metadata()
			ruleIndex[meta.ID] = meta
		}
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

	var dryRunValidator *dryrun.Validator
	if opts.DryRun.Enabled {
		dryRunValidator = dryrun.NewValidator(r.cfg, r.workdir, opts.DryRun)
		for _, meta := range dryRunValidator.Metadata() {
			ruleIndex[meta.ID] = meta
		}
	}

	maxParallel := opts.MaxParallel
	if maxParallel <= 0 {
		maxParallel = runtime.NumCPU()
		if maxParallel < 1 {
			maxParallel = 1
		}
	}
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	var findingsMu sync.Mutex
	var firstErr error
	var errOnce sync.Once
	var errFlag atomic.Bool
	setErr := func(err error) {
		if err == nil {
			return
		}
		errOnce.Do(func() {
			firstErr = err
			errFlag.Store(true)
		})
	}
	for _, manifest := range included {
		m := manifest
		wg.Add(1)
		go func() {
			defer wg.Done()
			if errFlag.Load() {
				return
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			if errFlag.Load() {
				return
			}
			localFindings := make([]types.Finding, 0, 4)
			schemaFindings, err := r.schema.Validate(m)
			if err != nil {
				setErr(err)
				return
			}
			localFindings = append(localFindings, schemaFindings...)
			if renderer != nil {
				renderFindings, err := renderer.Render(m)
				if err != nil {
					setErr(err)
					return
				}
				localFindings = append(localFindings, renderFindings...)
			}
			findingsMu.Lock()
			findings = append(findings, localFindings...)
			findingsMu.Unlock()
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return Report{}, firstErr
	}

	if dryRunValidator != nil {
		dryRunFindings, err := dryRunValidator.Validate(context.Background(), included)
		if err != nil {
			return Report{}, err
		}
		findings = append(findings, dryRunFindings...)
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
		if r.plugins != nil {
			ctxWithRule := context.Background()
			for _, plug := range r.plugins.Plugins() {
				if applies := plug.AppliesTo(); applies != nil && !applies(m) {
					continue
				}
				cfg, err := r.cfg.Resolve(plug.Metadata(), m.FilePath)
				if err != nil {
					return Report{}, err
				}
				if !cfg.Enabled {
					continue
				}
				results, err := plug.Check(ctxWithRule, m)
				if err != nil {
					return Report{}, err
				}
				for _, f := range results {
					if f.RuleID == "" {
						f.RuleID = cfg.Metadata.ID
					}
					if f.Severity == "" {
						f.Severity = cfg.Severity
					}
					if f.FilePath == "" {
						f.FilePath = m.FilePath
					}
					if f.ResourceName == "" {
						f.ResourceName = m.Name
					}
					if f.ResourceKind == "" {
						f.ResourceKind = m.Kind
					}
					if f.Category == "" {
						f.Category = cfg.Metadata.Category
					}
					if f.HelpURL == "" {
						f.HelpURL = cfg.Metadata.HelpURL
					}
					findings = append(findings, f)
				}
			}
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

	filtered, waiverFindings := applyWaivers(r.cfg, findings, ruleIndex)
	filtered = append(filtered, waiverFindings...)
	var agedBaseline, suppressed []types.Finding
	if opts.Baseline != nil {
		baselineFiltered, aged, suppressedEntries := opts.Baseline.Filter(filtered, opts.BaselineAgingDays)
		filtered = baselineFiltered
		agedBaseline = aged
		suppressed = suppressedEntries
	}
	filtered = append(filtered, agedBaseline...)
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].FilePath == filtered[j].FilePath {
			if filtered[i].Line == filtered[j].Line {
				if filtered[i].RuleID == filtered[j].RuleID {
					return filtered[i].Message < filtered[j].Message
				}
				return filtered[i].RuleID < filtered[j].RuleID
			}
			return filtered[i].Line < filtered[j].Line
		}
		return filtered[i].FilePath < filtered[j].FilePath
	})

	return Report{Findings: filtered, RuleIndex: ruleIndex, Suppressed: suppressed}, nil
}

func includeManifest(m *manifest.Manifest, apps, appsets, projects bool) bool {
	switch m.Kind {
	case string(types.ResourceKindApplication):
		return apps
	case string(types.ResourceKindApplicationSet):
		return appsets
	case string(types.ResourceKindAppProject):
		return projects
	default:
		return false
	}
}
