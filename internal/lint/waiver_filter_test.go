package lint

import (
	"testing"
	"time"

	"github.com/argocd-lint/argocd-lint/internal/config"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

func TestApplyWaiversSuppresses(t *testing.T) {
	cfg := config.Config{
		Waivers: []config.Waiver{
			{Rule: "AR001", File: "apps/*.yaml", Reason: "migration", Expires: time.Now().Add(24 * time.Hour).Format("2006-01-02")},
		},
	}
	findings := []types.Finding{{RuleID: "AR001", FilePath: "apps/app.yaml", Severity: types.SeverityError}}
	filtered, extras := applyWaivers(cfg, findings, map[string]types.RuleMetadata{})
	if len(filtered) != 0 {
		t.Fatalf("expected finding to be waived")
	}
	if len(extras) != 0 {
		t.Fatalf("expected no extra findings")
	}
}

func TestApplyWaiversExpired(t *testing.T) {
	cfg := config.Config{
		Waivers: []config.Waiver{
			{Rule: "AR001", File: "apps/*.yaml", Reason: "migration", Expires: time.Now().Add(-24 * time.Hour).Format("2006-01-02")},
		},
	}
	findings := []types.Finding{{RuleID: "AR001", FilePath: "apps/app.yaml", Severity: types.SeverityError}}
	filtered, extras := applyWaivers(cfg, findings, map[string]types.RuleMetadata{})
	if len(filtered) != 1 {
		t.Fatalf("expected original finding to remain when expired")
	}
	if len(extras) != 1 || extras[0].RuleID != waiverExpiredMeta.ID {
		t.Fatalf("expected expired waiver finding")
	}
}

func TestApplyWaiversInvalid(t *testing.T) {
	cfg := config.Config{
		Waivers: []config.Waiver{
			{Rule: "AR001", File: "apps/*.yaml", Reason: "", Expires: time.Now().Add(24 * time.Hour).Format(time.RFC3339)},
		},
	}
	findings := []types.Finding{{RuleID: "AR001", FilePath: "apps/app.yaml", Severity: types.SeverityError}}
	filtered, extras := applyWaivers(cfg, findings, map[string]types.RuleMetadata{})
	if len(filtered) != 1 {
		t.Fatalf("expected finding to remain when waiver invalid")
	}
	if len(extras) != 1 || extras[0].RuleID != waiverInvalidMeta.ID {
		t.Fatalf("expected invalid waiver finding")
	}
}
