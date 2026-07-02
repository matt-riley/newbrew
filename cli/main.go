// main.go — newbrew: browse recently-merged Homebrew formulae from your terminal.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/output"
	"github.com/matt-riley/newbrew/tui"

	flag "github.com/spf13/pflag"
)

// Build-time variables injected via -ldflags.
// See .goreleaser.yml for the ldflags block.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Exit codes:
//
//	0 — success
//	1 — operational failure (network, API, cache)
//	2 — usage/environment error (bad flag, no TTY, bad value)
const (
	exitSuccess   = 0
	exitOpFailure = 1
	exitUsage     = 2

	maxLimit = 100
	maxDays  = 365
)

func main() {
	days := flag.IntP("days", "d", 5, "look back this many days for merged Homebrew formulae")
	limit := flag.IntP("limit", "l", 50, "maximum number of pull requests to inspect")
	noCache := flag.BoolP("no-cache", "n", false, "disable cache reads and writes")
	plain := flag.Bool("plain", false, "output plain text (one formula per line, tab-separated fields)")
	jsonOut := flag.Bool("json", false, "output JSON array of formula objects")

	var showVersionFlag bool
	flag.BoolVarP(&showVersionFlag, "version", "v", false, "print version information and exit")
	// pflag allows only one shorthand per flag, so we register -V as a hidden
	// alias that writes the same variable.
	flag.BoolVarP(&showVersionFlag, "version-V", "V", false, "print version information and exit")
	_ = flag.CommandLine.MarkHidden("version-V")

	flag.Parse()

	if showVersionFlag {
		fmt.Printf("newbrew %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  date:    %s\n", date)
		fmt.Printf("  runtime: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}

	// Mutually exclusive output modes.
	if *plain && *jsonOut {
		fmt.Fprintln(os.Stderr, "newbrew: --plain and --json are mutually exclusive")
		os.Exit(exitUsage)
	}

	// Validate flags before constructing the fetcher.
	if *days <= 0 {
		fmt.Fprintf(os.Stderr, "newbrew: --days must be a positive integer (got %d)\n", *days)
		os.Exit(exitUsage)
	}
	if *limit <= 0 {
		fmt.Fprintf(os.Stderr, "newbrew: --limit must be a positive integer (got %d)\n", *limit)
		os.Exit(exitUsage)
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

	// Non-TTY detection: require --plain or --json when stdout is not a terminal.
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if !isTTY && !*plain && !*jsonOut {
		fmt.Fprintln(os.Stderr, "newbrew needs a terminal. Use --plain for scriptable output or --json for structured output.")
		os.Exit(exitUsage)
	}

	if *plain || *jsonOut {
		f := fetcher.New(fetcher.Config{
			Days:  *days,
			Limit: *limit,
		})

		var c fetcher.CacheInterface
		if !*noCache {
			cacheStore, err := cache.NewCache()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Warning:", err)
			} else {
				c = cacheStore
			}
		}

		result, err := f.FetchAndCache(c)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(exitOpFailure)
		}

		if len(result.Warnings) > 0 {
			fmt.Fprintln(os.Stderr, "Warnings:", strings.Join(result.Warnings, " | "))
		}

		if *plain {
			if err := output.WritePlain(os.Stdout, result.Formulae); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(exitOpFailure)
			}
		} else {
			if err := output.WriteJSON(os.Stdout, result.Formulae); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(exitOpFailure)
			}
		}

		return
	}

	// TUI mode
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
		os.Exit(exitOpFailure)
	}
}
