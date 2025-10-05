package render

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/argocd-lint/argocd-lint/internal/config"
    "github.com/argocd-lint/argocd-lint/internal/manifest"
    "github.com/argocd-lint/argocd-lint/pkg/types"
)

// Options configures rendering behaviour.
type Options struct {
    Enabled         bool
    HelmBinary      string
    KustomizeBinary string
    RepoRoot        string
}

// Renderer executes Helm/Kustomize renders and reports findings when they fail.
type Renderer struct {
    cfg             config.Config
    helmBinary      string
    kustomizeBinary string
    repoRoot        string
}

var (
    helmRuleMeta = types.RuleMetadata{
        ID:              "RENDER_HELM",
        Description:     "Helm template must succeed for referenced charts",
        DefaultSeverity: types.SeverityError,
        AppliesTo: []types.ResourceKind{
            types.ResourceKindApplication,
            types.ResourceKindApplicationSet,
        },
        Category: "render",
        Enabled:  true,
    }

    kustomizeRuleMeta = types.RuleMetadata{
        ID:              "RENDER_KUSTOMIZE",
        Description:     "Kustomize build must succeed for referenced overlays",
        DefaultSeverity: types.SeverityError,
        AppliesTo: []types.ResourceKind{
            types.ResourceKindApplication,
            types.ResourceKindApplicationSet,
        },
        Category: "render",
        Enabled:  true,
    }
)

// NewRenderer constructs a Renderer from configuration.
func NewRenderer(cfg config.Config, opts Options) (*Renderer, error) {
    if !opts.Enabled {
        return &Renderer{cfg: cfg}, nil
    }
    helmBin := strings.TrimSpace(opts.HelmBinary)
    if helmBin == "" {
        helmBin = "helm"
    }
    kustomizeBin := strings.TrimSpace(opts.KustomizeBinary)
    if kustomizeBin == "" {
        kustomizeBin = "kustomize"
    }
    repoRoot := opts.RepoRoot
    if repoRoot == "" {
        wd, err := os.Getwd()
        if err != nil {
            return nil, fmt.Errorf("resolve repo root: %w", err)
        }
        repoRoot = wd
    }
    return &Renderer{
        cfg:             cfg,
        helmBinary:      helmBin,
        kustomizeBinary: kustomizeBin,
        repoRoot:        repoRoot,
    }, nil
}

// Metadata exposes rule metadata for registration with reporting.
func (r *Renderer) Metadata() []types.RuleMetadata {
    return []types.RuleMetadata{helmRuleMeta, kustomizeRuleMeta}
}

// Render attempts to render Helm/Kustomize sources referenced by the manifest.
func (r *Renderer) Render(m *manifest.Manifest) ([]types.Finding, error) {
    if m == nil {
        return nil, errors.New("manifest is nil")
    }
    if r.helmBinary == "" && r.kustomizeBinary == "" {
        return nil, nil
    }

    sources := r.collectSources(m)
    if len(sources) == 0 {
        return nil, nil
    }

    var findings []types.Finding
    for _, src := range sources {
        path := strings.TrimSpace(getString(src, "path"))
        if path == "" {
            // Nothing to resolve locally.
            continue
        }
        absPath := path
        if !filepath.IsAbs(path) {
            absPath = filepath.Join(r.repoRoot, path)
        }
        absPath = filepath.Clean(absPath)
        info, err := os.Stat(absPath)
        if err != nil || !info.IsDir() {
            // Ignore sources we cannot resolve locally.
            continue
        }

        if r.shouldRenderHelm(src, absPath) {
            rendered, err := r.renderHelm(absPath, src, m)
            if err != nil {
                return nil, err
            }
            findings = append(findings, rendered...)
        }
        if r.shouldRenderKustomize(src, absPath) {
            rendered, err := r.renderKustomize(absPath, m)
            if err != nil {
                return nil, err
            }
            findings = append(findings, rendered...)
        }
    }

    return findings, nil
}

func (r *Renderer) renderHelm(path string, src map[string]interface{}, m *manifest.Manifest) ([]types.Finding, error) {
    cfg, err := r.cfg.Resolve(helmRuleMeta, m.FilePath)
    if err != nil {
        return nil, err
    }
    if !cfg.Enabled || r.helmBinary == "" {
        return nil, nil
    }
    args := []string{"template", "argocd-lint-render", "."}
    helmCfg := getMap(src, "helm")
    valueFiles := getSlice(helmCfg, "valueFiles")
    for _, item := range valueFiles {
        if str, ok := item.(string); ok && str != "" {
            args = append(args, "--values", filepath.Join(path, str))
        }
    }
    parameters := getSlice(helmCfg, "parameters")
    for _, item := range parameters {
        param, ok := item.(map[string]interface{})
        if !ok {
            continue
        }
        name := strings.TrimSpace(getString(param, "name"))
        value := getString(param, "value")
        if name == "" {
            continue
        }
        args = append(args, "--set", fmt.Sprintf("%s=%s", name, value))
    }
    releaseName := strings.TrimSpace(getString(helmCfg, "releaseName"))
    if releaseName != "" {
        args = append(args, "--release-name")
        args = append(args, releaseName)
    }

    cmd := exec.Command(r.helmBinary, args...)
    cmd.Dir = path
    output, err := cmd.CombinedOutput()
    if err == nil {
        return nil, nil
    }
    builder := types.FindingBuilder{
        Rule:         cfg,
        FilePath:     m.FilePath,
        Line:         m.MetadataLine,
        ResourceName: m.Name,
        ResourceKind: m.Kind,
    }
    msg := fmt.Sprintf("helm template failed in %s: %v", path, err)
    trimmed := trimOutput(output)
    if trimmed != "" {
        msg = fmt.Sprintf("%s: %s", msg, trimmed)
    }
    return []types.Finding{builder.NewFinding(msg, cfg.Severity)}, nil
}

func (r *Renderer) renderKustomize(path string, m *manifest.Manifest) ([]types.Finding, error) {
    cfg, err := r.cfg.Resolve(kustomizeRuleMeta, m.FilePath)
    if err != nil {
        return nil, err
    }
    if !cfg.Enabled || r.kustomizeBinary == "" {
        return nil, nil
    }
    cmd := exec.Command(r.kustomizeBinary, "build", path)
    cmd.Dir = path
    output, err := cmd.CombinedOutput()
    if err == nil {
        return nil, nil
    }
    builder := types.FindingBuilder{
        Rule:         cfg,
        FilePath:     m.FilePath,
        Line:         m.MetadataLine,
        ResourceName: m.Name,
        ResourceKind: m.Kind,
    }
    msg := fmt.Sprintf("kustomize build failed in %s: %v", path, err)
    trimmed := trimOutput(output)
    if trimmed != "" {
        msg = fmt.Sprintf("%s: %s", msg, trimmed)
    }
    return []types.Finding{builder.NewFinding(msg, cfg.Severity)}, nil
}

func (r *Renderer) shouldRenderHelm(src map[string]interface{}, path string) bool {
    if r.helmBinary == "" {
        return false
    }
    if strings.TrimSpace(getString(src, "chart")) != "" {
        if exists(filepath.Join(path, "Chart.yaml")) {
            return true
        }
        return false
    }
    if exists(filepath.Join(path, "Chart.yaml")) {
        return true
    }
    helmCfg := getMap(src, "helm")
    return len(helmCfg) > 0 && exists(filepath.Join(path, "Chart.yaml"))
}

func (r *Renderer) shouldRenderKustomize(src map[string]interface{}, path string) bool {
    if r.kustomizeBinary == "" {
        return false
    }
    if exists(filepath.Join(path, "kustomization.yaml")) || exists(filepath.Join(path, "kustomization.yml")) || exists(filepath.Join(path, "Kustomization")) {
        return true
    }
    kus := getMap(src, "kustomize")
    return len(kus) > 0 && exists(filepath.Join(path, "kustomization.yaml"))
}

func (r *Renderer) collectSources(m *manifest.Manifest) []map[string]interface{} {
    var results []map[string]interface{}
    switch m.Kind {
    case string(types.ResourceKindApplication):
        if src := getMap(m.Object, "spec", "source"); len(src) > 0 {
            results = append(results, src)
        }
        for _, item := range getSlice(m.Object, "spec", "sources") {
            if src, ok := item.(map[string]interface{}); ok {
                results = append(results, src)
            }
        }
    case string(types.ResourceKindApplicationSet):
        templateSpec := getMap(m.Object, "spec", "template", "spec")
        if len(templateSpec) == 0 {
            return results
        }
        if src := getMap(templateSpec, "source"); len(src) > 0 {
            results = append(results, src)
        }
        for _, item := range getSlice(templateSpec, "sources") {
            if src, ok := item.(map[string]interface{}); ok {
                results = append(results, src)
            }
        }
    }
    return results
}

func trimOutput(output []byte) string {
    trimmed := strings.TrimSpace(string(output))
    if trimmed == "" {
        return ""
    }
    if len(trimmed) > 280 {
        return trimmed[:280] + "..."
    }
    return trimmed
}

func exists(path string) bool {
    _, err := os.Stat(path)
    return err == nil
}

// Helpers replicate minimal YAML traversal without pulling rule internals.
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
