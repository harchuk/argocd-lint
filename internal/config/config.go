package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argocd-lint/argocd-lint/pkg/types"
	"gopkg.in/yaml.v3"
)

// RuleConfig describes rule overrides.
type RuleConfig struct {
	Enabled  *bool  `yaml:"enabled"`
	Severity string `yaml:"severity"`
}

// Override applies overrides based on file path pattern.
type Override struct {
	Pattern string                `yaml:"pattern"`
	Rules   map[string]RuleConfig `yaml:"rules"`
}

// Config is the runtime rule configuration.
type Config struct {
	Rules     map[string]RuleConfig `yaml:"rules"`
	Overrides []Override            `yaml:"overrides"`
	Threshold string                `yaml:"severityThreshold"`
	Policies  PolicyConfig          `yaml:"policies"`
	Profiles  []string              `yaml:"profiles"`
}

// PolicyConfig captures additional governance settings.
type PolicyConfig struct {
	AllowedRepoURLProtocols []string `yaml:"allowedRepoURLProtocols"`
	AllowedRepoURLDomains   []string `yaml:"allowedRepoURLDomains"`
}

// Load reads configuration from file. Empty path returns defaults.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if len(data) == 0 {
		return Config{}, nil
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.ApplyProfiles(cfg.Profiles...); err != nil {
		return Config{}, err
	}
	cfg.Profiles = append([]string(nil), cfg.Profiles...)
	return cfg, nil
}

// Resolve merges default rule metadata with configuration overrides.
func (c Config) Resolve(rule types.RuleMetadata, filePath string) (types.ConfiguredRule, error) {
	result := types.ConfiguredRule{
		Metadata: rule,
		Severity: rule.DefaultSeverity,
		Enabled:  rule.Enabled,
	}
	apply := func(rc RuleConfig) error {
		if rc.Enabled != nil {
			result.Enabled = *rc.Enabled
		}
		if rc.Severity != "" {
			sev, err := ParseSeverity(rc.Severity)
			if err != nil {
				return err
			}
			result.Severity = sev
		}
		return nil
	}

	if ruleConfig, ok := c.Rules[rule.ID]; ok {
		if err := apply(ruleConfig); err != nil {
			return result, err
		}
	}
	for _, override := range c.Overrides {
		if override.Pattern == "" {
			continue
		}
		match, err := filepath.Match(override.Pattern, filePath)
		if err != nil {
			return result, fmt.Errorf("invalid override pattern %q: %w", override.Pattern, err)
		}
		if match {
			if rc, ok := override.Rules[rule.ID]; ok {
				if err := apply(rc); err != nil {
					return result, err
				}
			}
		}
	}
	return result, nil
}

// ParseSeverity converts string to Severity type.
func ParseSeverity(value string) (types.Severity, error) {
	norm := strings.ToLower(strings.TrimSpace(value))
	switch norm {
	case string(types.SeverityInfo):
		return types.SeverityInfo, nil
	case string(types.SeverityWarn):
		return types.SeverityWarn, nil
	case string(types.SeverityError):
		return types.SeverityError, nil
	case "":
		return "", fmt.Errorf("empty severity")
	default:
		return "", errors.New("unknown severity: " + value)
	}
}
