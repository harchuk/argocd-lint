package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/argocd-lint/argocd-lint/internal/appsetplan"
	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/dryrun"
	"github.com/argocd-lint/argocd-lint/internal/lint"
	"github.com/argocd-lint/argocd-lint/internal/output"
	"github.com/argocd-lint/argocd-lint/internal/render"
	regoplugin "github.com/argocd-lint/argocd-lint/pkg/plugin/rego"
	"github.com/argocd-lint/argocd-lint/pkg/types"
	"github.com/argocd-lint/argocd-lint/pkg/version"
	"github.com/spf13/pflag"
)

// Execute is the entrypoint for the CLI. Returns process exit code.
func Execute(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "plugins":
			return runPluginsCommand(args[1:], stdout, stderr)
		case "applicationset":
			return runApplicationSetCommand(args[1:], stdout, stderr)
		}
	}
	flags := pflag.NewFlagSet("argocd-lint", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	rulesPath := flags.String("rules", "", "Path to rules configuration file")
	format := flags.String("format", "table", "Output format: table|json|sarif")
	includeApps := flags.Bool("apps", true, "Include Application manifests")
	includeAppSets := flags.Bool("appsets", true, "Include ApplicationSet manifests")
	includeProjects := flags.Bool("projects", true, "Include AppProject manifests")
	severityThreshold := flags.String("severity-threshold", "", "Exit with non-zero status at or above this severity (info|warn|error); overrides config")
	argocdVersion := flags.String("argocd-version", "", "Pin schema validation to a specific Argo CD version (e.g. v2.8)")
	renderEnabled := flags.Bool("render", false, "Render Helm/Kustomize sources before linting")
	helmBinary := flags.String("helm-binary", "helm", "Helm binary to use for rendering")
	kustomizeBinary := flags.String("kustomize-binary", "kustomize", "Kustomize binary to use for rendering")
	repoRoot := flags.String("repo-root", "", "Override repository root for resolving source paths when rendering")
	renderCache := flags.Bool("render-cache", false, "Cache render results for identical sources during a run")
	showVersion := flags.Bool("version", false, "Print argocd-lint version and exit")
	dryRunMode := flags.String("dry-run", "", "Perform extended validation: kubeconform|server")
	kubeconfig := flags.String("kubeconfig", "", "Path to kubeconfig for server-side dry-run")
	kubeContext := flags.String("kube-context", "", "Kubernetes context for server-side dry-run")
	kubectlBinary := flags.String("kubectl-binary", "kubectl", "kubectl binary to use for server dry-run")
	kubeconformBinary := flags.String("kubeconform-binary", "kubeconform", "kubeconform binary for schema validation")
	pluginFiles := flags.StringSlice("plugin", nil, "Path to a Rego plugin module (repeatable)")
	pluginDirs := flags.StringSlice("plugin-dir", nil, "Directory of Rego plugin modules (repeatable, recursive)")
	maxParallel := flags.Int("max-parallel", 0, "Maximum number of lint workers to run concurrently (0=CPU count)")
	profiles := flags.StringSlice("profile", nil, "Apply built-in rule profiles (dev, prod, security, hardening)")
	metricsFormat := flags.String("metrics", "", "Emit summary telemetry (table|json)")
	baselinePath := flags.String("baseline", "", "Path to baseline JSON that suppresses known findings")
	writeBaseline := flags.String("write-baseline", "", "Write current findings to baseline JSON")
	baselineAging := flags.Int("baseline-aging", 0, "Report baseline entries older than N days")

	if err := flags.Parse(args); err != nil {
		printError(stderr, "argument", err)
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, version.String())
		return 0
	}

	remaining := flags.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(stderr, "Usage: argocd-lint <path> [flags]")
		return 2
	}
	target := remaining[0]
	absTarget, err := ResolvePath(target)
	if err != nil {
		printError(stderr, "target", err)
		return 2
	}
	info, err := os.Stat(absTarget)
	if err != nil {
		printError(stderr, "target", err)
		return 2
	}

	cfg, err := config.Load(*rulesPath)
	if err != nil {
		printError(stderr, "config", err)
		return 2
	}
	if err := cfg.ApplyProfiles(*profiles...); err != nil {
		printError(stderr, "profile", err)
		return 2
	}
	var baseline *lint.Baseline
	if *baselinePath != "" {
		baseline, err = lint.LoadBaseline(*baselinePath)
		if err != nil {
			printError(stderr, "baseline", err)
			return 2
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		printError(stderr, "workdir", err)
		return 2
	}

	runner, err := lint.NewRunner(cfg, wd, *argocdVersion)
	if err != nil {
		printError(stderr, "runner", err)
		return 2
	}

	if len(*pluginFiles) > 0 || len(*pluginDirs) > 0 {
		var resolved []string
		for _, p := range append(*pluginFiles, *pluginDirs...) {
			path, err := ResolvePath(p)
			if err != nil {
				printError(stderr, "plugin path", err)
				return 2
			}
			if _, err := os.Stat(path); err != nil {
				printError(stderr, "plugin path", err)
				return 2
			}
			resolved = append(resolved, path)
		}
		loader := regoplugin.NewLoader(resolved...)
		plugins, err := loader.Load(context.Background())
		if err != nil {
			printError(stderr, "plugin load", err)
			return 2
		}
		runner.RegisterPlugins(plugins...)
	}

	root := *repoRoot
	if root != "" {
		root, err = ResolvePath(root)
		if err != nil {
			printError(stderr, "repo root", err)
			return 2
		}
	} else {
		if info.IsDir() {
			root = absTarget
		} else {
			root = filepath.Dir(absTarget)
		}
	}

	renderOpts := render.Options{
		Enabled:         *renderEnabled,
		HelmBinary:      *helmBinary,
		KustomizeBinary: *kustomizeBinary,
		RepoRoot:        root,
		CacheEnabled:    *renderCache,
	}

	dryRunOpts := dryrun.Options{
		Enabled:           *dryRunMode != "",
		Mode:              *dryRunMode,
		KubectlBinary:     *kubectlBinary,
		KubeconformBinary: *kubeconformBinary,
		Kubeconfig:        *kubeconfig,
		KubeContext:       *kubeContext,
	}

	threshold := cfg.Threshold
	if *severityThreshold != "" {
		threshold = *severityThreshold
	}

	opts := lint.Options{
		Target:                 target,
		IncludeApplications:    *includeApps,
		IncludeApplicationSets: *includeAppSets,
		IncludeProjects:        *includeProjects,
		Config:                 cfg,
		WorkingDir:             wd,
		Render:                 renderOpts,
		SeverityThreshold:      threshold,
		DryRun:                 dryRunOpts,
		MaxParallel:            *maxParallel,
		Baseline:               baseline,
		BaselineAgingDays:      *baselineAging,
	}

	start := time.Now()
	report, err := runner.Run(opts)
	if err != nil {
		printError(stderr, "lint", err)
		return 2
	}
	duration := time.Since(start)

	if err := output.Write(report, *format, stdout); err != nil {
		printError(stderr, "output", err)
		return 2
	}
	if strings.TrimSpace(*metricsFormat) != "" {
		if err := output.WriteMetrics(report, duration, *metricsFormat, stdout); err != nil {
			printError(stderr, "metrics", err)
			return 2
		}
	}
	if *writeBaseline != "" {
		if err := lint.WriteBaseline(*writeBaseline, report.Suppressed); err != nil {
			printError(stderr, "baseline", err)
			return 2
		}
	}

	thresholdValue := opts.SeverityThreshold
	if thresholdValue == "" {
		thresholdValue = string(types.SeverityError)
	}
	thresholdSeverity, err := config.ParseSeverity(thresholdValue)
	if err != nil {
		printError(stderr, "threshold", err)
		return 2
	}

	highest := output.HighestSeverity(report.Findings)
	if types.SeverityOrder[highest] >= types.SeverityOrder[thresholdSeverity] && len(report.Findings) > 0 {
		return 1
	}

	return 0
}

// ResolvePath ensures the target is absolute relative to working dir.
func ResolvePath(target string) (string, error) {
	if filepath.IsAbs(target) {
		return target, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, target), nil
}

type pluginRow struct {
	Bundle      string   `json:"bundle"`
	Rule        string   `json:"rule"`
	Severity    string   `json:"severity"`
	AppliesTo   []string `json:"appliesTo"`
	Category    string   `json:"category,omitempty"`
	Description string   `json:"description"`
	HelpURL     string   `json:"helpUrl,omitempty"`
	Source      string   `json:"source"`
}

func runPluginsCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "list" {
		return runPluginsList(args, stdout, stderr)
	}
	fmt.Fprintln(stderr, "Usage: argocd-lint plugins list [flags]")
	return 2
}

func runPluginsList(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "list" {
		args = args[1:]
	}
	flags := pflag.NewFlagSet("plugins list", pflag.ContinueOnError)
	flags.SetOutput(stderr)
	dirs := flags.StringSlice("dir", nil, "Bundle directories to inspect (default: ./bundles)")
	format := flags.String("format", "table", "Output format: table|json")
	if err := flags.Parse(args); err != nil {
		printError(stderr, "argument", err)
		return 2
	}
	roots := *dirs
	if len(roots) == 0 {
		roots = []string{"bundles"}
	}
	wd, err := os.Getwd()
	if err != nil {
		printError(stderr, "workdir", err)
		return 2
	}
	var rows []pluginRow
	ctx := context.Background()
	for _, root := range roots {
		resolved, err := ResolvePath(root)
		if err != nil {
			printError(stderr, "plugin dir", err)
			return 2
		}
		info, statErr := os.Stat(resolved)
		if statErr != nil {
			printError(stderr, "plugin dir", statErr)
			return 2
		}
		records, missing, err := regoplugin.DiscoverMetadata(ctx, resolved)
		if err != nil {
			printError(stderr, "plugin load", err)
			return 2
		}
		if len(missing) > 0 {
			printError(stderr, "plugin path", fmt.Errorf("missing: %s", strings.Join(missing, ", ")))
			return 2
		}
		bundleName := info.Name()
		if !info.IsDir() {
			bundleName = filepath.Base(filepath.Dir(resolved))
		}
		for _, rec := range records {
			source := rec.Source
			if rel, relErr := filepath.Rel(wd, source); relErr == nil {
				source = rel
			}
			applies := make([]string, 0, len(rec.Metadata.AppliesTo))
			for _, kind := range rec.Metadata.AppliesTo {
				applies = append(applies, string(kind))
			}
			rows = append(rows, pluginRow{
				Bundle:      bundleName,
				Rule:        rec.Metadata.ID,
				Severity:    string(rec.Metadata.DefaultSeverity),
				AppliesTo:   applies,
				Category:    rec.Metadata.Category,
				Description: rec.Metadata.Description,
				HelpURL:     rec.Metadata.HelpURL,
				Source:      source,
			})
		}
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "No plugins found.")
		return 0
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Bundle == rows[j].Bundle {
			return rows[i].Rule < rows[j].Rule
		}
		return rows[i].Bundle < rows[j].Bundle
	})
	switch strings.ToLower(*format) {
	case "", "table":
		if err := renderPluginTable(rows, stdout); err != nil {
			printError(stderr, "output", err)
			return 2
		}
		return 0
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			printError(stderr, "output", err)
			return 2
		}
		return 0
	default:
		printError(stderr, "format", fmt.Errorf("unsupported format %q", *format))
		return 2
	}
}

func renderPluginTable(rows []pluginRow, w io.Writer) error {
	headers := []string{"Bundle", "Rule", "Severity", "Applies", "Category", "Description", "Source"}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	data := make([][]string, 0, len(rows))
	for _, row := range rows {
		severity := strings.ToUpper(row.Severity)
		if severity == "" {
			severity = "INFO"
		}
		applies := "-"
		if len(row.AppliesTo) > 0 {
			applies = strings.Join(row.AppliesTo, ",")
		}
		entry := []string{
			row.Bundle,
			row.Rule,
			severity,
			applies,
			row.Category,
			row.Description,
			row.Source,
		}
		data = append(data, entry)
		for i, cell := range entry {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	separator := make([]string, len(widths))
	for i, width := range widths {
		separator[i] = strings.Repeat("-", width+2)
	}
	lineFmt := func(values []string) string {
		var b strings.Builder
		b.WriteString("|")
		for i, width := range widths {
			fmt.Fprintf(&b, " %-*s ", width, values[i])
			b.WriteString("|")
		}
		b.WriteString("\n")
		return b.String()
	}
	if _, err := fmt.Fprintln(w, "+"+strings.Join(separator, "+")+"+"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, lineFmt(headers)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "+"+strings.Join(separator, "+")+"+"); err != nil {
		return err
	}
	for _, row := range data {
		if _, err := io.WriteString(w, lineFmt(row)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "+"+strings.Join(separator, "+")+"+"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "\nTotal: %d rules\n", len(rows))
	return err
}

func runApplicationSetCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "plan" {
		return runApplicationSetPlan(args, stdout, stderr)
	}
	fmt.Fprintln(stderr, "Usage: argocd-lint applicationset plan --file <path> [flags]")
	return 2
}

func runApplicationSetPlan(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "plan" {
		args = args[1:]
	}
	flags := pflag.NewFlagSet("applicationset plan", pflag.ContinueOnError)
	flags.SetOutput(stderr)
	file := flags.String("file", "", "Path to ApplicationSet manifest")
	current := flags.String("current", "", "Directory or file with existing Application manifests")
	format := flags.String("format", "table", "Output format: table|json")
	if err := flags.Parse(args); err != nil {
		printError(stderr, "argument", err)
		return 2
	}
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(stderr, "--file is required")
		return 2
	}
	plan, err := appsetplan.Generate(appsetplan.Options{AppSetPath: *file, CurrentDir: *current})
	if err != nil {
		printError(stderr, "plan", err)
		return 2
	}
	switch strings.ToLower(*format) {
	case "", "table":
		if err := renderPlanTable(plan, stdout); err != nil {
			printError(stderr, "output", err)
			return 2
		}
		return 0
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(plan); err != nil {
			printError(stderr, "output", err)
			return 2
		}
		return 0
	default:
		printError(stderr, "format", fmt.Errorf("unsupported format %q", *format))
		return 2
	}
}

func renderPlanTable(plan appsetplan.Result, w io.Writer) error {
	headers := []string{"Action", "Name", "Destination", "Source"}
	widths := make([]int, len(headers))
	for i, head := range headers {
		widths[i] = len(head)
	}
	rows := make([][]string, 0, len(plan.Rows))
	for _, row := range plan.Rows {
		entry := []string{
			strings.ToUpper(string(row.Action)),
			row.Name,
			formatDestination(row.Destination),
			formatSource(row.Source),
		}
		rows = append(rows, entry)
		for i, cell := range entry {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	separator := make([]string, len(widths))
	for i, width := range widths {
		separator[i] = strings.Repeat("-", width+2)
	}
	line := func(values []string) string {
		var b strings.Builder
		b.WriteString("|")
		for i, width := range widths {
			fmt.Fprintf(&b, " %-*s ", width, values[i])
			b.WriteString("|")
		}
		b.WriteString("\n")
		return b.String()
	}
	if _, err := fmt.Fprintln(w, "+"+strings.Join(separator, "+")+"+"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, line(headers)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "+"+strings.Join(separator, "+")+"+"); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := io.WriteString(w, line(row)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "+"+strings.Join(separator, "+")+"+"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "\nTotal: %d  create=%d  delete=%d  unchanged=%d\n", plan.Summary.Total, plan.Summary.Create, plan.Summary.Delete, plan.Summary.Unchanged)
	return err
}

func formatDestination(dest appsetplan.DestinationPreview) string {
	parts := make([]string, 0, 3)
	if dest.Namespace != "" {
		parts = append(parts, dest.Namespace)
	}
	if dest.Name != "" {
		parts = append(parts, dest.Name)
	}
	if dest.Server != "" {
		parts = append(parts, dest.Server)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " | ")
}

func formatSource(src appsetplan.SourcePreview) string {
	parts := make([]string, 0, 3)
	if src.RepoURL != "" {
		parts = append(parts, src.RepoURL)
	}
	if src.Path != "" {
		parts = append(parts, src.Path)
	}
	if src.Chart != "" {
		parts = append(parts, fmt.Sprintf("chart=%s", src.Chart))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " | ")
}

func printError(w io.Writer, stage string, err error) {
	fmt.Fprintf(w, "[ERROR] %-12s %v\n", strings.ToUpper(stage), err)
}
