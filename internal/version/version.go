package version

import "fmt"

// Populated via -ldflags at build time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("yafu %s (commit %s, built %s)", Version, Commit, Date)
}
