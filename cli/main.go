// main.go — newbrew: browse recently-merged Homebrew formulae from your terminal.
package main

import (
	"fmt"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	flag "github.com/spf13/pflag"

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

func customUsage() {
	fmt.Fprintf(os.Stderr, `newbrew — discover newly-merged Homebrew formulae

Usage:
  newbrew [flags]

Flags:
  -d, --days N       look back this many days for merged Homebrew formulae (default: 5)
  -l, --limit N      maximum number of pull requests to inspect (default: 50)
  -n, --no-cache     disable cache reads and writes
  -v, --version      print version and exit
  -V                  same as --version
  -h, --help         show this help message and exit

Environment:
  GITHUB_TOKEN       GitHub personal access token (public_repo scope).
                     If unset, newbrew uses unauthenticated API access with lower rate limits.
  XDG_CACHE_HOME     Cache directory. Defaults to ~/.cache. newbrew caches API
                     responses in XDG_CACHE_HOME/newbrew/ for 24 hours.

Examples:
  newbrew                          # show last 5 days, up to 50 formulae
  newbrew -d 7 -l 100              # show last 7 days, up to 100 formulae
  newbrew -d 14 -l 50 -n           # skip cache, show last 14 days
  GITHUB_TOKEN=ghp_... newbrew     # authenticated access for higher rate limits
`)
}

func main() {
	var showVersionFlag bool

	days := flag.IntP("days", "d", 5, "look back this many days for merged Homebrew formulae")
	limit := flag.IntP("limit", "l", 50, "maximum number of pull requests to inspect")
	noCache := flag.BoolP("no-cache", "n", false, "disable cache reads and writes")
	flag.BoolVarP(&showVersionFlag, "version", "v", false, "print version information and exit")

	// pflag does not support multiple shorthands on one flag, so we register
	// -V as a hidden second flag that writes the same variable.  The flag
	// itself is invisible in --help so we document -V inside customUsage.
	flag.BoolVarP(&showVersionFlag, "version-V", "V", false, "print version information and exit")

	flag.Usage = customUsage
	flag.Parse()

	if showVersionFlag {
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
