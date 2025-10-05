package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	flags := pflag.NewFlagSet("argocd-lint", pflag.ContinueOnError)
	flags.SetOutput(stderr)

	rulesPath := flags.String("rules", "", "Path to rules configuration file")
	format := flags.String("format", "table", "Output format: table|json|sarif")
	includeApps := flags.Bool("apps", true, "Include Application manifests")
	includeAppSets := flags.Bool("appsets", true, "Include ApplicationSet manifests")
	severityThreshold := flags.String("severity-threshold", "", "Exit with non-zero status at or above this severity (info|warn|error); overrides config")
	renderEnabled := flags.Bool("render", false, "Render Helm/Kustomize sources before linting")
	helmBinary := flags.String("helm-binary", "helm", "Helm binary to use for rendering")
	kustomizeBinary := flags.String("kustomize-binary", "kustomize", "Kustomize binary to use for rendering")
	repoRoot := flags.String("repo-root", "", "Override repository root for resolving source paths when rendering")
	showVersion := flags.Bool("version", false, "Print argocd-lint version and exit")
	dryRunMode := flags.String("dry-run", "", "Perform extended validation: kubeconform|server")
	kubeconfig := flags.String("kubeconfig", "", "Path to kubeconfig for server-side dry-run")
	kubeContext := flags.String("kube-context", "", "Kubernetes context for server-side dry-run")
	kubectlBinary := flags.String("kubectl-binary", "kubectl", "kubectl binary to use for server dry-run")
	kubeconformBinary := flags.String("kubeconform-binary", "kubeconform", "kubeconform binary for schema validation")
	pluginFiles := flags.StringSlice("plugin", nil, "Path to a Rego plugin module (repeatable)")
	pluginDirs := flags.StringSlice("plugin-dir", nil, "Directory of Rego plugin modules (repeatable, recursive)")

	if err := flags.Parse(args); err != nil {
		fmt.Fprintf(stderr, "argument error: %v\n", err)
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, version.String())
		return 0
	}

	remaining := flags.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(stderr, "usage: argocd-lint <path> [flags]")
		return 2
	}
	target := remaining[0]
	absTarget, err := ResolvePath(target)
	if err != nil {
		fmt.Fprintf(stderr, "target error: %v\n", err)
		return 2
	}
	info, err := os.Stat(absTarget)
	if err != nil {
		fmt.Fprintf(stderr, "target error: %v\n", err)
		return 2
	}

	cfg, err := config.Load(*rulesPath)
	if err != nil {
		fmt.Fprintf(stderr, "config error: %v\n", err)
		return 2
	}

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "working directory error: %v\n", err)
		return 2
	}

	runner, err := lint.NewRunner(cfg, wd)
	if err != nil {
		fmt.Fprintf(stderr, "runner error: %v\n", err)
		return 2
	}

	if len(*pluginFiles) > 0 || len(*pluginDirs) > 0 {
		var resolved []string
		for _, p := range append(*pluginFiles, *pluginDirs...) {
			path, err := ResolvePath(p)
			if err != nil {
				fmt.Fprintf(stderr, "plugin path error: %v\n", err)
				return 2
			}
			if _, err := os.Stat(path); err != nil {
				fmt.Fprintf(stderr, "plugin path error: %v\n", err)
				return 2
			}
			resolved = append(resolved, path)
		}
		loader := regoplugin.NewLoader(resolved...)
		plugins, err := loader.Load(context.Background())
		if err != nil {
			fmt.Fprintf(stderr, "plugin load error: %v\n", err)
			return 2
		}
		runner.RegisterPlugins(plugins...)
	}

	root := *repoRoot
	if root != "" {
		root, err = ResolvePath(root)
		if err != nil {
			fmt.Fprintf(stderr, "repo root error: %v\n", err)
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
		Config:                 cfg,
		WorkingDir:             wd,
		Render:                 renderOpts,
		SeverityThreshold:      threshold,
		DryRun:                 dryRunOpts,
	}

	report, err := runner.Run(opts)
	if err != nil {
		fmt.Fprintf(stderr, "lint error: %v\n", err)
		return 2
	}

	if err := output.Write(report, *format, stdout); err != nil {
		fmt.Fprintf(stderr, "output error: %v\n", err)
		return 2
	}

	thresholdValue := opts.SeverityThreshold
	if thresholdValue == "" {
		thresholdValue = string(types.SeverityError)
	}
	thresholdSeverity, err := config.ParseSeverity(thresholdValue)
	if err != nil {
		fmt.Fprintf(stderr, "severity threshold error: %v\n", err)
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
