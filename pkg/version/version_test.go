package version

import (
	"strings"
	"testing"
)

func TestStringIncludesBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, GitCommit, BuildDate
	Version, GitCommit, BuildDate = "1.2.3", "abc123", "2026-01-01T00:00:00Z"
	t.Cleanup(func() { Version, GitCommit, BuildDate = oldVersion, oldCommit, oldDate })

	got := String()
	for _, want := range []string{"1.2.3", "abc123", "2026-01-01T00:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("String() = %q, missing %q", got, want)
		}
	}
}
