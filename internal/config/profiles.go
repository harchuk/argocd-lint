package config

import (
	"fmt"
	"sort"
	"strings"
)

type profile struct {
	rules     map[string]RuleConfig
	threshold string
}

var builtinProfiles = map[string]profile{
	"dev": {
		rules: map[string]RuleConfig{
			"AR001": {Severity: "warn"},
			"AR005": {Severity: "info"},
			"AR013": {Severity: "warn"},
			"AR014": {Severity: "warn"},
		},
		threshold: "warn",
	},
	"prod": {
		rules: map[string]RuleConfig{
			"AR001": {Severity: "error"},
			"AR007": {Severity: "error"},
			"AR013": {Severity: "error"},
			"AR014": {Severity: "error"},
		},
		threshold: "error",
	},
	"security": {
		rules: map[string]RuleConfig{
			"AR013": {Severity: "error"},
			"AR014": {Severity: "error"},
			"AR010": {Severity: "warn"},
		},
	},
	"hardening": {
		rules: map[string]RuleConfig{
			"AR001": {Severity: "error"},
			"AR010": {Severity: "warn"},
			"AR013": {Severity: "error"},
			"AR014": {Severity: "error"},
		},
		threshold: "error",
	},
}

// ApplyProfiles merges the provided built-in profiles into the configuration.
func (cfg *Config) ApplyProfiles(names ...string) error {
	if len(names) == 0 {
		return nil
	}
	if cfg.Rules == nil {
		cfg.Rules = make(map[string]RuleConfig)
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		profile, ok := builtinProfiles[strings.ToLower(name)]
		if !ok {
			return fmt.Errorf("unknown profile %q", name)
		}
		if profile.threshold != "" {
			cfg.Threshold = profile.threshold
		}
		for ruleID, override := range profile.rules {
			existing := cfg.Rules[ruleID]
			if override.Enabled != nil {
				existing.Enabled = override.Enabled
			}
			if override.Severity != "" {
				existing.Severity = override.Severity
			}
			cfg.Rules[ruleID] = existing
		}
	}
	return nil
}

// AvailableProfiles returns a sorted list of built-in profile names.
func AvailableProfiles() []string {
	names := make([]string, 0, len(builtinProfiles))
	for name := range builtinProfiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
