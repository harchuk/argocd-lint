package cli

import (
    "fmt"
    "io"
    "os"
    "path/filepath"

    "github.com/argocd-lint/argocd-lint/internal/config"
    "github.com/argocd-lint/argocd-lint/internal/lint"
    "github.com/argocd-lint/argocd-lint/internal/output"
    "github.com/argocd-lint/argocd-lint/internal/render"
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
    severityThreshold := flags.String("severity-threshold", "error", "Exit with non-zero status at or above this severity (info|warn|error)")
    renderEnabled := flags.Bool("render", false, "Render Helm/Kustomize sources before linting")
    helmBinary := flags.String("helm-binary", "helm", "Helm binary to use for rendering")
    kustomizeBinary := flags.String("kustomize-binary", "kustomize", "Kustomize binary to use for rendering")
    repoRoot := flags.String("repo-root", "", "Override repository root for resolving source paths when rendering")
    showVersion := flags.Bool("version", false, "Print argocd-lint version and exit")

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

    opts := lint.Options{
        Target:                 target,
        IncludeApplications:    *includeApps,
        IncludeApplicationSets: *includeAppSets,
        Config:                 cfg,
        WorkingDir:             wd,
        Render:                 renderOpts,
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

    threshold, err := config.ParseSeverity(*severityThreshold)
    if err != nil {
        fmt.Fprintf(stderr, "severity threshold error: %v\n", err)
        return 2
    }

    highest := output.HighestSeverity(report.Findings)
    if types.SeverityOrder[highest] >= types.SeverityOrder[threshold] && len(report.Findings) > 0 {
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
