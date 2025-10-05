package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/argocd-lint/argocd-lint/internal/lint"
	"github.com/argocd-lint/argocd-lint/pkg/types"
)

// Format enumerates supported output formats.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatSARIF = "sarif"
)

// Write renders the report to the writer using the requested format.
func Write(report lint.Report, format string, w io.Writer) error {
	switch strings.ToLower(format) {
	case "", FormatTable:
		return writeTable(report, w)
	case FormatJSON:
		return writeJSON(report, w)
	case FormatSARIF:
		return writeSARIF(report, w)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeTable(report lint.Report, w io.Writer) error {
	if len(report.Findings) == 0 {
		if _, err := fmt.Fprintln(w, "No findings."); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, "\nSummary: %s\n", SummaryString(report.Findings))
		return err
	}
	headers := []string{"Severity", "Rule", "Resource", "Location", "Message"}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	rows := make([][]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		severity := strings.ToUpper(string(f.Severity))
		if severity == "" {
			severity = "INFO"
		}
		resource := fmt.Sprintf("%s/%s", f.ResourceKind, f.ResourceName)
		location := f.FilePath
		if f.Line > 0 {
			location = fmt.Sprintf("%s:%d", f.FilePath, f.Line)
		}
		row := []string{severity, f.RuleID, resource, location, f.Message}
		rows = append(rows, row)
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	separator := buildTableSeparator(widths)
	if _, err := fmt.Fprintln(w, separator); err != nil {
		return err
	}
	if err := writeTableRow(w, headers, widths); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, separator); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeTableRow(w, row, widths); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, separator); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "\nSummary: %s\n", SummaryString(report.Findings))
	return err
}

func buildTableSeparator(widths []int) string {
	parts := make([]string, len(widths))
	for i, width := range widths {
		parts[i] = strings.Repeat("-", width+2)
	}
	return "+" + strings.Join(parts, "+") + "+"
}

func writeTableRow(w io.Writer, values []string, widths []int) error {
	var b strings.Builder
	b.WriteString("|")
	for i, width := range widths {
		fmt.Fprintf(&b, " %-*s ", width, values[i])
		b.WriteString("|")
	}
	b.WriteString("\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func writeJSON(report lint.Report, w io.Writer) error {
	payload := struct {
		Findings []types.Finding               `json:"findings"`
		Rules    map[string]types.RuleMetadata `json:"rules"`
	}{
		Findings: report.Findings,
		Rules:    report.RuleIndex,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func writeSARIF(report lint.Report, w io.Writer) error {
	type sarifResult struct {
		RuleID  string `json:"ruleId"`
		Level   string `json:"level"`
		Message struct {
			Text string `json:"text"`
		} `json:"message"`
		Locations []struct {
			PhysicalLocation struct {
				ArtifactLocation struct {
					URI string `json:"uri"`
				} `json:"artifactLocation"`
				Region struct {
					StartLine int `json:"startLine,omitempty"`
				} `json:"region"`
			} `json:"physicalLocation"`
		} `json:"locations"`
		Properties map[string]interface{} `json:"properties,omitempty"`
	}
	type sarifSuggestion struct {
		Title       string `json:"title"`
		Description string `json:"description,omitempty"`
		Patch       string `json:"patch,omitempty"`
		Path        string `json:"path,omitempty"`
	}
	type sarifRule struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		ShortDesc struct {
			Text string `json:"text"`
		} `json:"shortDescription"`
		FullDesc struct {
			Text string `json:"text"`
		} `json:"fullDescription"`
		HelpURI string `json:"helpUri,omitempty"`
	}
	type sarifTool struct {
		Driver struct {
			Name           string      `json:"name"`
			InformationURI string      `json:"informationUri"`
			Rules          []sarifRule `json:"rules"`
		} `json:"driver"`
	}
	type sarifRun struct {
		Tool    sarifTool     `json:"tool"`
		Results []sarifResult `json:"results"`
	}
	type sarif struct {
		Schema  string     `json:"$schema"`
		Version string     `json:"version"`
		Runs    []sarifRun `json:"runs"`
	}

	ruleIDs := make([]string, 0, len(report.RuleIndex))
	for id := range report.RuleIndex {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	driver := sarifTool{}
	driver.Driver.Name = "argocd-lint"
	driver.Driver.InformationURI = "https://github.com/argocd-lint/argocd-lint"
	driver.Driver.Rules = make([]sarifRule, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		meta := report.RuleIndex[id]
		ruleEntry := sarifRule{ID: meta.ID, Name: meta.Category}
		ruleEntry.ShortDesc.Text = meta.Description
		ruleEntry.FullDesc.Text = meta.Description
		ruleEntry.HelpURI = meta.HelpURL
		driver.Driver.Rules = append(driver.Driver.Rules, ruleEntry)
	}

	results := make([]sarifResult, 0, len(report.Findings))
	for _, finding := range report.Findings {
		res := sarifResult{RuleID: finding.RuleID, Level: string(finding.Severity)}
		res.Message.Text = finding.Message
		location := struct {
			PhysicalLocation struct {
				ArtifactLocation struct {
					URI string `json:"uri"`
				} `json:"artifactLocation"`
				Region struct {
					StartLine int `json:"startLine,omitempty"`
				} `json:"region"`
			} `json:"physicalLocation"`
		}{}
		location.PhysicalLocation.ArtifactLocation.URI = finding.FilePath
		location.PhysicalLocation.Region.StartLine = finding.Line
		res.Locations = []struct {
			PhysicalLocation struct {
				ArtifactLocation struct {
					URI string `json:"uri"`
				} `json:"artifactLocation"`
				Region struct {
					StartLine int `json:"startLine,omitempty"`
				} `json:"region"`
			} `json:"physicalLocation"`
		}{location}
		if len(finding.Suggestions) > 0 {
			suggestions := make([]sarifSuggestion, 0, len(finding.Suggestions))
			for _, suggestion := range finding.Suggestions {
				suggestions = append(suggestions, sarifSuggestion{
					Title:       suggestion.Title,
					Description: suggestion.Description,
					Patch:       suggestion.Patch,
					Path:        suggestion.Path,
				})
			}
			res.Properties = map[string]interface{}{
				"suggestions": suggestions,
			}
		}
		results = append(results, res)
	}

	payload := sarif{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{
			{
				Tool:    driver,
				Results: results,
			},
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// HighestSeverity returns the highest severity in findings.
func HighestSeverity(findings []types.Finding) types.Severity {
	highest := types.SeverityInfo
	for _, f := range findings {
		highest = types.HigherSeverity(highest, f.Severity)
	}
	return highest
}

// SummaryString generates a short textual summary.
func SummaryString(findings []types.Finding) string {
	if len(findings) == 0 {
		return "0 findings"
	}
	counts := map[types.Severity]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	keys := []types.Severity{types.SeverityError, types.SeverityWarn, types.SeverityInfo}
	var parts []string
	for _, key := range keys {
		if counts[key] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[key], key))
		}
	}
	return fmt.Sprintf("%d findings (%s)", len(findings), strings.Join(parts, ", "))
}

// MetadataStamp returns RFC3339 timestamp for use in reports.
func MetadataStamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
