package version

import "fmt"

var (
	// Version is the semantic version of the binary.
	Version = "0.1.0"
	// GitCommit is populated via -ldflags at build time.
	GitCommit = "dev"
	// BuildDate is RFC3339 timestamp injected at build time.
	BuildDate = "unknown"
)

// String returns a human-friendly version string.
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildDate)
}
