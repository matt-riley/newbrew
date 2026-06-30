// main.go — newbrew: browse recently-merged Homebrew formulae from your terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/tui"
)

// Build-time variables injected via -ldflags.
// See .goreleaser.yml for the ldflags block.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const (
	maxLimit = 100
	maxDays  = 365
)

func main() {
	days := flag.Int("days", 5, "look back this many days for merged Homebrew formulae")
	limit := flag.Int("limit", 50, "maximum number of pull requests to inspect")
	noCache := flag.Bool("no-cache", false, "disable cache reads and writes")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("newbrew %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  date:    %s\n", date)
		fmt.Printf("  runtime: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}

	// Validate flags before constructing the fetcher.
	if *days <= 0 {
		fmt.Fprintf(os.Stderr, "newbrew: --days must be a positive integer (got %d)\n", *days)
		os.Exit(2)
	}
	if *limit <= 0 {
		fmt.Fprintf(os.Stderr, "newbrew: --limit must be a positive integer (got %d)\n", *limit)
		os.Exit(2)
	}

	// Cap out-of-range values with a visible warning to stderr.
	if *limit > maxLimit {
		fmt.Fprintf(os.Stderr, "newbrew: --limit %d exceeds maximum, capping to %d\n", *limit, maxLimit)
		*limit = maxLimit
	}
	if *days > maxDays {
		fmt.Fprintf(os.Stderr, "newbrew: --days %d exceeds maximum, capping to %d\n", *days, maxDays)
		*days = maxDays
	}

	model := tui.NewModel(tui.Config{
		Days:     *days,
		Limit:    *limit,
		UseCache: !*noCache,
		Fetcher: fetcher.New(fetcher.Config{
			Days:    *days,
			Limit:   *limit,
			Version: version,
		}),
	})

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "newbrew: %v\n", err)
		os.Exit(1)
	}
}
