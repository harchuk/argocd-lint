package plugin

import (
	"context"

	"github.com/argocd-lint/argocd-lint/internal/manifest"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

// Matcher decides whether a plugin applies to a manifest.
type Matcher func(*manifest.Manifest) bool

// RulePlugin is the interface custom rules must satisfy.
type RulePlugin interface {
	Metadata() types.RuleMetadata
	Check(ctx context.Context, m *manifest.Manifest) ([]types.Finding, error)
	AppliesTo() Matcher
}

// Registry stores registered rule plugins.
type Registry struct {
	plugins []RulePlugin
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds plugins to the registry.
func (r *Registry) Register(plugins ...RulePlugin) {
	r.plugins = append(r.plugins, plugins...)
}

// Plugins returns all registered plugins.
func (r *Registry) Plugins() []RulePlugin {
	return append([]RulePlugin(nil), r.plugins...)
}
