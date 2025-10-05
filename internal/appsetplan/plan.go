package appsetplan

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/argocd-lint/argocd-lint/internal/loader"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/types"
	"gopkg.in/yaml.v3"
)

// Action enumerates plan operations.
type Action string

const (
	ActionCreate   Action = "create"
	ActionDelete   Action = "delete"
	ActionUnchange Action = "unchanged"
)

// PlanRow represents a single application in the preview.
type PlanRow struct {
	Name        string
	Action      Action
	Destination DestinationPreview
	Source      SourcePreview
}

// DestinationPreview summarises target cluster/namespace.
type DestinationPreview struct {
	Server    string
	Namespace string
	Name      string
}

// SourcePreview summarises repo path/chart details.
type SourcePreview struct {
	RepoURL string
	Path    string
	Chart   string
}

// Result is the top-level plan payload.
type Result struct {
	ApplicationSet string
	Rows           []PlanRow
	Summary        Summary
}

// Summary aggregates plan counts.
type Summary struct {
	Total     int
	Create    int
	Delete    int
	Unchanged int
}

// Options configures the planner.
type Options struct {
	AppSetPath string
	CurrentDir string
}

// Generate produces the ApplicationSet plan.
func Generate(opts Options) (Result, error) {
	if opts.AppSetPath == "" {
		return Result{}, fmt.Errorf("appset path is required")
	}
	parser := manifest.Parser{}
	docs, err := parser.ParseFile(opts.AppSetPath)
	if err != nil {
		return Result{}, err
	}
	var appset *manifest.Manifest
	for _, doc := range docs {
		if doc != nil && doc.Kind == string(types.ResourceKindApplicationSet) {
			appset = doc
			break
		}
	}
	if appset == nil {
		return Result{}, fmt.Errorf("no ApplicationSet found in %s", opts.AppSetPath)
	}

	desired, err := renderDesiredApplications(appset)
	if err != nil {
		return Result{}, err
	}

	currentNames, err := discoverCurrentApplications(opts.CurrentDir)
	if err != nil {
		return Result{}, err
	}

	rows := make([]PlanRow, 0, len(desired)+len(currentNames))
	summary := Summary{}

	seen := map[string]struct{}{}
	for _, app := range desired {
		row := app
		if _, ok := currentNames[app.Name]; ok {
			row.Action = ActionUnchange
			summary.Unchanged++
		} else {
			row.Action = ActionCreate
			summary.Create++
		}
		rows = append(rows, row)
		seen[app.Name] = struct{}{}
	}
	for name := range currentNames {
		if _, ok := seen[name]; ok {
			continue
		}
		rows = append(rows, PlanRow{Name: name, Action: ActionDelete})
		summary.Delete++
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Action == rows[j].Action {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].Action < rows[j].Action
	})

	summary.Total = len(rows)
	return Result{
		ApplicationSet: appset.Name,
		Rows:           rows,
		Summary:        summary,
	}, nil
}

func renderDesiredApplications(appset *manifest.Manifest) ([]PlanRow, error) {
	spec := mapGet(appset.Object, "spec")
	generators := sliceGet(spec, "generators")
	if len(generators) == 0 {
		return nil, fmt.Errorf("ApplicationSet %s has no generators", appset.Name)
	}

	template := mapGet(spec, "template")
	if len(template) == 0 {
		return nil, fmt.Errorf("ApplicationSet %s missing template", appset.Name)
	}

	var desired []PlanRow
	for _, raw := range generators {
		genMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if list := mapGet(genMap, "list"); len(list) > 0 {
			elements := sliceGet(list, "elements")
			for _, element := range elements {
				ctx, ok := element.(map[string]interface{})
				if !ok {
					continue
				}
				rendered, err := renderTemplate(template, ctx)
				if err != nil {
					return nil, fmt.Errorf("render template: %w", err)
				}
				row := extractPreview(rendered)
				if row.Name == "" {
					row.Name = fmt.Sprintf("<unnamed:%d>", len(desired))
				}
				desired = append(desired, row)
			}
			continue
		}
	}
	if len(desired) == 0 {
		return nil, fmt.Errorf("unsupported generators in ApplicationSet %s", appset.Name)
	}
	return desired, nil
}

func renderTemplate(tpl map[string]interface{}, item map[string]interface{}) (map[string]interface{}, error) {
	raw, err := yaml.Marshal(tpl)
	if err != nil {
		return nil, err
	}
	tmpl, err := templateWithSprig(string(raw), item)
	if err != nil {
		return nil, err
	}
	data := map[string]interface{}{
		"item":   item,
		"values": item,
	}
	for k, v := range item {
		data[k] = v
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	var rendered map[string]interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &rendered); err != nil {
		return nil, err
	}
	return rendered, nil
}

func templateWithSprig(body string, item map[string]interface{}) (*template.Template, error) {
	funcMap := sprig.TxtFuncMap()
	for k, v := range item {
		key := k
		val := v
		funcMap[key] = func() interface{} { return val }
	}
	tmpl := template.New("appset").Funcs(funcMap)
	tmpl.Option("missingkey=zero")
	return tmpl.Parse(body)
}

func extractPreview(rendered map[string]interface{}) PlanRow {
	metadata := mapGet(rendered, "metadata")
	spec := mapGet(rendered, "spec")
	destMap := mapGet(spec, "destination")
	sourceMap := mapGet(spec, "source")
	name := stringGet(metadata, "name")
	row := PlanRow{
		Name: name,
		Destination: DestinationPreview{
			Server:    stringGet(destMap, "server"),
			Namespace: stringGet(destMap, "namespace"),
			Name:      stringGet(destMap, "name"),
		},
		Source: SourcePreview{
			RepoURL: stringGet(sourceMap, "repoURL"),
			Path:    stringGet(sourceMap, "path"),
			Chart:   stringGet(sourceMap, "chart"),
		},
	}
	return row
}

func mapGet(obj map[string]interface{}, path ...string) map[string]interface{} {
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

func sliceGet(obj map[string]interface{}, path ...string) []interface{} {
	if len(path) == 0 {
		return nil
	}
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

func stringGet(obj map[string]interface{}, path ...string) string {
	if len(path) == 0 {
		return ""
	}
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

func discoverCurrentApplications(current string) (map[string]struct{}, error) {
	names := map[string]struct{}{}
	if strings.TrimSpace(current) == "" {
		return names, nil
	}
	info, err := os.Stat(current)
	if err != nil {
		return nil, fmt.Errorf("discover current apps: %w", err)
	}
	var files []string
	if info.IsDir() {
		files, err = loader.DiscoverFiles(current)
		if err != nil {
			return nil, err
		}
	} else {
		files = []string{current}
	}
	parser := manifest.Parser{}
	for _, file := range files {
		docs, err := parser.ParseFile(file)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			if doc != nil && doc.Kind == string(types.ResourceKindApplication) {
				names[doc.Name] = struct{}{}
			}
		}
	}
	return names, nil
}
