package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Waiver suppresses findings for a rule/file combination until expiry.
type Waiver struct {
	Rule    string `yaml:"rule"`
	File    string `yaml:"file"`
	Reason  string `yaml:"reason"`
	Expires string `yaml:"expires"`
}

// Validate performs static validation at load time.
func (w Waiver) Validate() error {
	if strings.TrimSpace(w.Rule) == "" {
		return fmt.Errorf("rule is required")
	}
	if strings.TrimSpace(w.File) == "" {
		return fmt.Errorf("file pattern is required")
	}
	if strings.TrimSpace(w.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if _, err := w.ExpiryTime(); err != nil {
		return err
	}
	return nil
}

// ExpiryTime parses the expiry timestamp.
func (w Waiver) ExpiryTime() (time.Time, error) {
	value := strings.TrimSpace(w.Expires)
	if value == "" {
		return time.Time{}, fmt.Errorf("expires is required")
	}
	// Accept RFC3339 or date-only.
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02", value); err == nil {
		return ts, nil
	}
	return time.Time{}, fmt.Errorf("invalid expires format %q (expected RFC3339 or YYYY-MM-DD)", value)
}

// Matches determines whether the waiver covers the provided finding.
func (w Waiver) Matches(finding string, ruleID string) bool {
	if strings.ToLower(strings.TrimSpace(ruleID)) != strings.ToLower(strings.TrimSpace(w.Rule)) {
		return false
	}
	pattern := strings.TrimSpace(w.File)
	if pattern == "" {
		return false
	}
	ok, _ := filepath.Match(pattern, finding)
	return ok
}
