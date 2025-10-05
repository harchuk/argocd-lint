package rego

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	opaast "github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"

	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/plugin"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

// Loader discovers and instantiates Rego-backed plugins.
type Loader struct {
	files   []string
	missing []string
}

// NewLoader creates a Loader for the provided file paths.
func NewLoader(paths ...string) *Loader {
	unique := make(map[string]struct{}, len(paths))
	var normalized []string
	var missing []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		info, statErr := os.Stat(abs)
		if statErr == nil {
			if info.IsDir() {
				_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if d.IsDir() {
						return nil
					}
					if strings.HasSuffix(d.Name(), ".rego") {
						if _, seen := unique[path]; !seen {
							unique[path] = struct{}{}
							normalized = append(normalized, path)
						}
					}
					return nil
				})
				continue
			}
			if strings.HasSuffix(abs, ".rego") {
				if _, seen := unique[abs]; !seen {
					unique[abs] = struct{}{}
					normalized = append(normalized, abs)
				}
			}
			continue
		}
		missing = append(missing, abs)
	}
	sort.Strings(normalized)
	sort.Strings(missing)
	return &Loader{files: normalized, missing: missing}
}

// MetadataRecord describes a discovered plugin rule.
type MetadataRecord struct {
	Source   string
	Metadata types.RuleMetadata
}

// DiscoverMetadata loads metadata for the provided plugin paths without
// retaining the instantiated plugins. Missing paths are returned for caller
// awareness.
func DiscoverMetadata(ctx context.Context, paths ...string) ([]MetadataRecord, []string, error) {
	loader := NewLoader(paths...)
	records := make([]MetadataRecord, 0, len(loader.files))
	for _, file := range loader.files {
		plug, err := loadFile(ctx, file)
		if err != nil {
			return nil, loader.missing, err
		}
		if rp, ok := plug.(*regoPlugin); ok {
			records = append(records, MetadataRecord{Source: rp.source, Metadata: rp.meta})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Metadata.ID == records[j].Metadata.ID {
			return records[i].Source < records[j].Source
		}
		return records[i].Metadata.ID < records[j].Metadata.ID
	})
	return records, loader.missing, nil
}

// Load instantiates RulePlugin implementations from the loader's files.
func (l *Loader) Load(ctx context.Context) ([]plugin.RulePlugin, error) {
	if len(l.missing) > 0 {
		return nil, fmt.Errorf("missing plugin paths: %s", strings.Join(l.missing, ", "))
	}
	plugins := make([]plugin.RulePlugin, 0, len(l.files))
	for _, file := range l.files {
		p, err := loadFile(ctx, file)
		if err != nil {
			return nil, fmt.Errorf("load rego plugin %s: %w", file, err)
		}
		plugins = append(plugins, p)
	}
	return plugins, nil
}

type regoPlugin struct {
	source       string
	meta         types.RuleMetadata
	denyQuery    rego.PreparedEvalQuery
	appliesQuery *rego.PreparedEvalQuery
}

func (p *regoPlugin) Metadata() types.RuleMetadata {
	return p.meta
}

func (p *regoPlugin) Check(ctx context.Context, m *manifest.Manifest) ([]types.Finding, error) {
	input := manifestToInput(m)

	if p.appliesQuery != nil {
		rs, err := p.appliesQuery.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			return nil, fmt.Errorf("evaluate applies: %w", err)
		}
		if len(rs) == 0 {
			return nil, nil
		}
		matched := false
		for _, result := range rs {
			for _, exp := range result.Expressions {
				if b, ok := exp.Value.(bool); ok {
					if b {
						matched = true
					}
				}
			}
		}
		if !matched {
			return nil, nil
		}
	}

	rs, err := p.denyQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, fmt.Errorf("evaluate deny: %w", err)
	}
	var findings []types.Finding
	for _, result := range rs {
		for _, exp := range result.Expressions {
			maps, err := extractFindingMaps(exp.Value)
			if err != nil {
				return nil, err
			}
			for _, entry := range maps {
				findings = append(findings, mapToFinding(entry, p.meta, m))
			}
		}
	}
	return findings, nil
}

func (p *regoPlugin) AppliesTo() plugin.Matcher {
	if len(p.meta.AppliesTo) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(p.meta.AppliesTo))
	for _, kind := range p.meta.AppliesTo {
		allowed[string(kind)] = struct{}{}
	}
	return func(m *manifest.Manifest) bool {
		_, ok := allowed[m.Kind]
		return ok
	}
}

func loadFile(ctx context.Context, path string) (plugin.RulePlugin, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read module: %w", err)
	}

	module, err := opaast.ParseModule(path, string(source))
	if err != nil {
		return nil, fmt.Errorf("parse module: %w", err)
	}

	compiler, err := opaast.CompileModules(map[string]string{path: string(source)})
	if err != nil {
		return nil, fmt.Errorf("compile module: %w", err)
	}

	pkgRef := module.Package.Path.String()

	metadataQuery, err := rego.New(
		rego.Compiler(compiler),
		rego.Query(fmt.Sprintf("%s.metadata", pkgRef)),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("prepare metadata query: %w", err)
	}

	denyQuery, err := rego.New(
		rego.Compiler(compiler),
		rego.Query(fmt.Sprintf("%s.deny", pkgRef)),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("prepare deny query: %w", err)
	}

	var appliesQuery *rego.PreparedEvalQuery
	if hasRule(module, "applies") {
		prepared, err := rego.New(
			rego.Compiler(compiler),
			rego.Query(fmt.Sprintf("%s.applies", pkgRef)),
		).PrepareForEval(ctx)
		if err != nil {
			return nil, fmt.Errorf("prepare applies query: %w", err)
		}
		appliesQuery = &prepared
	}

	meta, err := evaluateMetadata(ctx, metadataQuery)
	if err != nil {
		return nil, err
	}

	return &regoPlugin{source: path, meta: meta, denyQuery: denyQuery, appliesQuery: appliesQuery}, nil
}

func manifestToInput(m *manifest.Manifest) map[string]interface{} {
	return map[string]interface{}{
		"file":           m.FilePath,
		"document_index": m.DocumentIndex,
		"kind":           m.Kind,
		"api_version":    m.APIVersion,
		"name":           m.Name,
		"namespace":      m.Namespace,
		"line":           m.Line,
		"column":         m.Column,
		"metadata_line":  m.MetadataLine,
		"object":         m.Object,
	}
}

func hasRule(module *opaast.Module, name string) bool {
	for _, rule := range module.Rules {
		if string(rule.Head.Name) == name {
			return true
		}
	}
	return false
}

func evaluateMetadata(ctx context.Context, query rego.PreparedEvalQuery) (types.RuleMetadata, error) {
	rs, err := query.Eval(ctx)
	if err != nil {
		return types.RuleMetadata{}, fmt.Errorf("evaluate metadata: %w", err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return types.RuleMetadata{}, fmt.Errorf("metadata query returned no results")
	}
	obj, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		return types.RuleMetadata{}, fmt.Errorf("metadata must be an object")
	}
	meta := types.RuleMetadata{Enabled: true}
	id, _ := obj["id"].(string)
	if id == "" {
		return types.RuleMetadata{}, fmt.Errorf("metadata.id is required")
	}
	meta.ID = id
	if desc, ok := obj["description"].(string); ok {
		meta.Description = desc
	}
	if category, ok := obj["category"].(string); ok {
		meta.Category = category
	}
	if help, ok := obj["help_url"].(string); ok {
		meta.HelpURL = help
	}
	if enabled, ok := obj["enabled"].(bool); ok {
		meta.Enabled = enabled
	}
	if severity, ok := obj["severity"].(string); ok {
		meta.DefaultSeverity = types.Severity(strings.ToLower(severity))
	} else {
		meta.DefaultSeverity = types.SeverityWarn
	}
	if applies, ok := obj["applies_to"].([]interface{}); ok {
		for _, item := range applies {
			if s, ok := item.(string); ok {
				meta.AppliesTo = append(meta.AppliesTo, types.ResourceKind(s))
			}
		}
	}
	return meta, nil
}

func extractFindingMaps(value interface{}) ([]map[string]interface{}, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			m, err := toStringMap(item)
			if err != nil {
				return nil, err
			}
			out = append(out, m)
		}
		return out, nil
	case map[string]interface{}:
		return []map[string]interface{}{v}, nil
	default:
		return nil, fmt.Errorf("deny query must return objects or array of objects, got %T", value)
	}
}

func toStringMap(val interface{}) (map[string]interface{}, error) {
	switch typed := val.(type) {
	case map[string]interface{}:
		return typed, nil
	default:
		return nil, fmt.Errorf("finding must be object, got %T", val)
	}
}

func mapToFinding(raw map[string]interface{}, meta types.RuleMetadata, man *manifest.Manifest) types.Finding {
	finding := types.Finding{
		RuleID:       meta.ID,
		Severity:     meta.DefaultSeverity,
		Message:      "violation",
		FilePath:     man.FilePath,
		Line:         man.Line,
		Column:       man.Column,
		ResourceName: man.Name,
		ResourceKind: man.Kind,
		Category:     meta.Category,
		HelpURL:      meta.HelpURL,
	}

	if msg, ok := raw["message"].(string); ok && msg != "" {
		finding.Message = msg
	}
	if ruleID, ok := raw["rule_id"].(string); ok && ruleID != "" {
		finding.RuleID = ruleID
	}
	if severity, ok := raw["severity"].(string); ok && severity != "" {
		finding.Severity = types.Severity(strings.ToLower(severity))
	}
	if file, ok := raw["file"].(string); ok && file != "" {
		finding.FilePath = file
	}
	if line, ok := numberToInt(raw["line"]); ok {
		finding.Line = line
	}
	if column, ok := numberToInt(raw["column"]); ok {
		finding.Column = column
	}
	if resourceName, ok := raw["resource_name"].(string); ok && resourceName != "" {
		finding.ResourceName = resourceName
	}
	if resourceKind, ok := raw["resource_kind"].(string); ok && resourceKind != "" {
		finding.ResourceKind = resourceKind
	}
	if cat, ok := raw["category"].(string); ok && cat != "" {
		finding.Category = cat
	}
	if help, ok := raw["help_url"].(string); ok && help != "" {
		finding.HelpURL = help
	}
	return finding
}

func numberToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case int32:
		return int(v), true
	case int16:
		return int(v), true
	case int8:
		return int(v), true
	case uint:
		return int(v), true
	case uint64:
		return int(v), true
	case uint32:
		return int(v), true
	case uint16:
		return int(v), true
	case uint8:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case json.Number:
		if i, err := strconv.Atoi(v.String()); err == nil {
			return i, true
		}
		if f, err := v.Float64(); err == nil {
			return int(f), true
		}
		return 0, false
	default:
		return 0, false
	}
}
