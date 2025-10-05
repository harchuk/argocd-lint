package dryrun

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

// Options controls dry-run validation behaviour.
type Options struct {
	Mode              string
	KubectlBinary     string
	KubeconformBinary string
	Kubeconfig        string
	KubeContext       string
	Enabled           bool
}

// Validator executes optional dry-run validation using kubectl or kubeconform.
type Validator struct {
	cfg             config.Config
	workdir         string
	options         Options
	ruleServer      types.RuleMetadata
	ruleKubeconform types.RuleMetadata
}

const (
	modeServer      = "server"
	modeKubeconform = "kubeconform"
)

// NewValidator creates a dry-run validator.
func NewValidator(cfg config.Config, workdir string, opts Options) *Validator {
	return &Validator{
		cfg:     cfg,
		workdir: workdir,
		options: opts,
		ruleServer: types.RuleMetadata{
			ID:              "DRYRUN_SERVER",
			Description:     "kubectl --dry-run=server must succeed",
			DefaultSeverity: types.SeverityError,
			AppliesTo:       []types.ResourceKind{types.ResourceKindApplication, types.ResourceKindApplicationSet},
			Category:        "validation",
			Enabled:         true,
		},
		ruleKubeconform: types.RuleMetadata{
			ID:              "DRYRUN_KUBECONFORM",
			Description:     "kubeconform validation must succeed",
			DefaultSeverity: types.SeverityError,
			AppliesTo:       []types.ResourceKind{types.ResourceKindApplication, types.ResourceKindApplicationSet},
			Category:        "validation",
			Enabled:         true,
		},
	}
}

// Metadata exposes rule metadata for registration.
func (v *Validator) Metadata() []types.RuleMetadata {
	return []types.RuleMetadata{v.ruleServer, v.ruleKubeconform}
}

// Validate executes the configured dry-run mode against the provided manifests.
func (v *Validator) Validate(ctx context.Context, manifests []*manifest.Manifest) ([]types.Finding, error) {
	if !v.options.Enabled || v.options.Mode == "" {
		return nil, nil
	}
	mode := strings.ToLower(v.options.Mode)
	files := groupByFile(manifests)
	switch mode {
	case modeServer:
		return v.validateKubectl(ctx, files)
	case modeKubeconform:
		return v.validateKubeconform(ctx, files)
	default:
		return nil, fmt.Errorf("unsupported dry-run mode %q", v.options.Mode)
	}
}

func (v *Validator) validateKubectl(ctx context.Context, files map[string][]*manifest.Manifest) ([]types.Finding, error) {
	var findings []types.Finding
	for file, manifests := range files {
		cfg, err := v.cfg.Resolve(v.ruleServer, file)
		if err != nil {
			return nil, err
		}
		if !cfg.Enabled {
			continue
		}
		args := []string{"apply", "--dry-run=server", "--filename", file, "--validate=true"}
		if v.options.Kubeconfig != "" {
			args = append(args, "--kubeconfig", v.options.Kubeconfig)
		}
		if v.options.KubeContext != "" {
			args = append(args, "--context", v.options.KubeContext)
		}
		binary := v.options.KubectlBinary
		if strings.TrimSpace(binary) == "" {
			binary = "kubectl"
		}
		msg, err := runCommand(ctx, v.workdir, binary, args...)
		if err == nil {
			continue
		}
		for _, m := range manifests {
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			findings = append(findings, builder.NewFinding(msg, cfg.Severity))
		}
	}
	return findings, nil
}

func (v *Validator) validateKubeconform(ctx context.Context, files map[string][]*manifest.Manifest) ([]types.Finding, error) {
	var findings []types.Finding
	for file, manifests := range files {
		cfg, err := v.cfg.Resolve(v.ruleKubeconform, file)
		if err != nil {
			return nil, err
		}
		if !cfg.Enabled {
			continue
		}
		binary := v.options.KubeconformBinary
		if strings.TrimSpace(binary) == "" {
			binary = "kubeconform"
		}
		args := []string{"--summary", file}
		msg, err := runCommand(ctx, v.workdir, binary, args...)
		if err == nil {
			continue
		}
		for _, m := range manifests {
			builder := types.FindingBuilder{Rule: cfg, FilePath: m.FilePath, Line: m.MetadataLine, ResourceName: m.Name, ResourceKind: m.Kind}
			findings = append(findings, builder.NewFinding(msg, cfg.Severity))
		}
	}
	return findings, nil
}

func groupByFile(manifests []*manifest.Manifest) map[string][]*manifest.Manifest {
	files := make(map[string][]*manifest.Manifest)
	for _, m := range manifests {
		files[m.FilePath] = append(files[m.FilePath], m)
	}
	return files
}

func runCommand(ctx context.Context, dir, binary string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := strings.TrimSpace(strings.Join([]string{stdout.String(), stderr.String()}, "\n"))
	if output == "" {
		if err != nil {
			output = err.Error()
		}
	}
	return output, err
}
