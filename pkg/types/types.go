package types

// Severity enumerates lint finding levels.
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// SeverityOrder helps compare severities.
var SeverityOrder = map[Severity]int{
	SeverityInfo:  0,
	SeverityWarn:  1,
	SeverityError: 2,
}

// ResourceKind identifies supported Argo CD resource types.
type ResourceKind string

const (
	ResourceKindApplication    ResourceKind = "Application"
	ResourceKindApplicationSet ResourceKind = "ApplicationSet"
	ResourceKindAppProject     ResourceKind = "AppProject"
)

// Finding represents a lint rule result.
type Finding struct {
	RuleID       string       `json:"ruleId"`
	Message      string       `json:"message"`
	Severity     Severity     `json:"severity"`
	FilePath     string       `json:"file"`
	Line         int          `json:"line,omitempty"`
	Column       int          `json:"column,omitempty"`
	ResourceName string       `json:"resourceName"`
	ResourceKind string       `json:"resourceKind"`
	Category     string       `json:"category,omitempty"`
	HelpURL      string       `json:"helpUrl,omitempty"`
	Suggestions  []Suggestion `json:"suggestions,omitempty"`
}

// Suggestion proposes an optional remediation for a finding.
type Suggestion struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Patch       string `json:"patch,omitempty"`
	Path        string `json:"path,omitempty"`
}

// RuleMetadata keeps description for reporting.
type RuleMetadata struct {
	ID              string
	Description     string
	DefaultSeverity Severity
	AppliesTo       []ResourceKind
	HelpURL         string
	Category        string
	Enabled         bool
}

// ConfiguredRule holds runtime configuration.
type ConfiguredRule struct {
	Metadata RuleMetadata
	Severity Severity
	Enabled  bool
}

// FindingBuilder is used inside rule checks to construct findings.
type FindingBuilder struct {
	Rule         ConfiguredRule
	FilePath     string
	Line         int
	Column       int
	ResourceName string
	ResourceKind string
}

// NewFinding creates a finding for the provided message.
func (b FindingBuilder) NewFinding(message string, severity Severity) Finding {
	sev := severity
	if sev == "" {
		sev = b.Rule.Severity
	}
	return Finding{
		RuleID:       b.Rule.Metadata.ID,
		Message:      message,
		Severity:     sev,
		FilePath:     b.FilePath,
		Line:         b.Line,
		Column:       b.Column,
		ResourceName: b.ResourceName,
		ResourceKind: b.ResourceKind,
		Category:     b.Rule.Metadata.Category,
		HelpURL:      b.Rule.Metadata.HelpURL,
	}
}

// HigherSeverity returns the higher of two severities.
func HigherSeverity(a, b Severity) Severity {
	if SeverityOrder[a] >= SeverityOrder[b] {
		return a
	}
	return b
}
