// main.go
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/tui"
)

var version = "dev"

const (
	maxDays  = 365
	maxLimit = 100
)

func main() {
	days := flag.Int("days", 5, "look back this many days for merged Homebrew formulae")
	limit := flag.Int("limit", 50, "maximum number of pull requests to inspect")
	noCache := flag.Bool("no-cache", false, "disable cache reads and writes")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	// Validate --days: zero or negative is a fatal error.
	if *days <= 0 {
		fmt.Fprintf(os.Stderr, "newbrew: --days must be a positive integer (got %d)\n", *days)
		os.Exit(2)
	}

	// Validate --limit: zero or negative is a fatal error.
	if *limit <= 0 {
		fmt.Fprintf(os.Stderr, "newbrew: --limit must be a positive integer (got %d)\n", *limit)
		os.Exit(2)
	}

	var clampWarnings []string

	if *days > maxDays {
		clampWarnings = append(clampWarnings,
			fmt.Sprintf("--days capped from %d to %d", *days, maxDays))
		*days = maxDays
	}

	if *limit > maxLimit {
		clampWarnings = append(clampWarnings,
			fmt.Sprintf("--limit capped from %d to %d", *limit, maxLimit))
		*limit = maxLimit
	}

	model := tui.NewModel(tui.Config{
		Days:          *days,
		Limit:         *limit,
		UseCache:      !*noCache,
		ClampWarnings: clampWarnings,
		Fetcher: fetcher.New(fetcher.Config{
			Days:  *days,
			Limit: *limit,
		}),
	})

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "newbrew: %v\n", err)
		os.Exit(1)
	}
}
