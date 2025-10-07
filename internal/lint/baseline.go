package lint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/argocd-lint/argocd-lint/pkg/types"
)

var baselineAgedMeta = types.RuleMetadata{
	ID:              "BASELINE_AGED",
	Description:     "Baseline entry has been present longer than allowed",
	DefaultSeverity: types.SeverityWarn,
	Category:        "baseline",
	Enabled:         true,
}

// BaselineEntry captures a suppressed finding recorded at a point in time.
type BaselineEntry struct {
	Rule       string `json:"rule"`
	File       string `json:"file"`
	Introduced string `json:"introduced,omitempty"`
}

// Baseline holds parsed entries for lookup.
type Baseline struct {
	Entries []BaselineEntry
	index   map[string]BaselineEntry
}

// LoadBaseline loads a baseline JSON file. Missing files are tolerated.
func LoadBaseline(path string) (*Baseline, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Baseline{index: map[string]BaselineEntry{}}, nil
		}
		return nil, fmt.Errorf("read baseline: %w", err)
	}
	if len(data) == 0 {
		return &Baseline{index: map[string]BaselineEntry{}}, nil
	}
	var entries []BaselineEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse baseline: %w", err)
	}
	bl := &Baseline{Entries: entries, index: make(map[string]BaselineEntry)}
	for _, entry := range entries {
		key := baselineKey(entry.File, entry.Rule)
		bl.index[key] = entry
	}
	return bl, nil
}

// WriteBaseline persists findings to the target path in JSON format.
func WriteBaseline(path string, findings []types.Finding) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("baseline path required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create baseline dir: %w", err)
	}
	now := time.Now().Format("2006-01-02")
	entries := make([]BaselineEntry, 0, len(findings))
	seen := map[string]struct{}{}
	for _, f := range findings {
		key := baselineKey(f.FilePath, f.RuleID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, BaselineEntry{
			Rule:       f.RuleID,
			File:       f.FilePath,
			Introduced: now,
		})
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write baseline: %w", err)
	}
	return nil
}

// Filter applies the baseline, returning remaining findings and aged entries.
func (b *Baseline) Filter(findings []types.Finding, agingDays int) ([]types.Finding, []types.Finding, []types.Finding) {
	if b == nil || len(b.index) == 0 {
		return findings, nil, nil
	}
	threshold := time.Time{}
	if agingDays > 0 {
		threshold = time.Now().Add(-time.Duration(agingDays) * 24 * time.Hour)
	}
	aged := []types.Finding{}
	suppressed := []types.Finding{}
	result := make([]types.Finding, 0, len(findings))
	for _, f := range findings {
		key := baselineKey(f.FilePath, f.RuleID)
		entry, ok := b.index[key]
		if !ok {
			result = append(result, f)
			continue
		}
		suppressed = append(suppressed, f)
		if !threshold.IsZero() {
			if introduced, err := time.Parse("2006-01-02", entry.Introduced); err == nil && introduced.Before(threshold) {
				aged = append(aged, types.Finding{
					RuleID:   baselineAgedMeta.ID,
					Message:  fmt.Sprintf("baseline entry for %s (%s) older than %d days", f.RuleID, f.FilePath, agingDays),
					Severity: baselineAgedMeta.DefaultSeverity,
					FilePath: f.FilePath,
					Category: baselineAgedMeta.Category,
				})
			}
		}
	}
	return result, aged, suppressed
}

func baselineKey(file, rule string) string {
	return strings.ToLower(strings.TrimSpace(file)) + "|" + strings.ToLower(strings.TrimSpace(rule))
}
