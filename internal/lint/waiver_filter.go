package lint

import (
	"fmt"
	"strings"
	"time"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

var waiverExpiredMeta = types.RuleMetadata{
	ID:              "WAIVER_EXPIRED",
	Description:     "Waiver has expired; finding is no longer suppressed",
	DefaultSeverity: types.SeverityWarn,
	Category:        "waiver",
	Enabled:         true,
}

var waiverInvalidMeta = types.RuleMetadata{
	ID:              "WAIVER_INVALID",
	Description:     "Waiver is invalid (missing rule, reason, or expiry)",
	DefaultSeverity: types.SeverityWarn,
	Category:        "waiver",
	Enabled:         true,
}

func applyWaivers(cfg config.Config, findings []types.Finding, ruleIndex map[string]types.RuleMetadata) ([]types.Finding, []types.Finding) {
	if len(cfg.Waivers) == 0 {
		return findings, nil
	}
	now := time.Now()
	waived := make([]bool, len(findings))
	var extra []types.Finding
	for idx, waiver := range cfg.Waivers {
		expires, err := waiver.ExpiryTime()
		if err != nil {
			msg := fmt.Sprintf("waiver %d invalid: %v", idx, err)
			extra = append(extra, newWaiverFinding(waiverInvalidMeta, waiver.File, msg, types.SeverityWarn))
			continue
		}
		for i, f := range findings {
			if waived[i] {
				continue
			}
			if !waiver.Matches(f.FilePath, f.RuleID) {
				continue
			}
			if expires.Before(now) {
				msg := fmt.Sprintf("waiver for %s on %s expired %s (%s)", f.RuleID, f.FilePath, expires.Format(time.RFC3339), waiver.Reason)
				extra = append(extra, newWaiverFinding(waiverExpiredMeta, f.FilePath, msg, types.SeverityWarn))
				continue
			}
			if strings.TrimSpace(waiver.Reason) == "" {
				msg := fmt.Sprintf("waiver for %s on %s missing reason", f.RuleID, f.FilePath)
				extra = append(extra, newWaiverFinding(waiverInvalidMeta, f.FilePath, msg, types.SeverityWarn))
				continue
			}
			waived[i] = true
		}
	}
	filtered := make([]types.Finding, 0, len(findings))
	for i, f := range findings {
		if waived[i] {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered, extra
}

func newWaiverFinding(meta types.RuleMetadata, file, message string, severity types.Severity) types.Finding {
	return types.Finding{
		RuleID:   meta.ID,
		Message:  message,
		Severity: severity,
		FilePath: file,
		Category: meta.Category,
	}
}
