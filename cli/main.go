// main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/tui"
)

var version = "dev"

// Exit codes:
//
//	0 — success
//	1 — operational failure (network, API, cache)
//	2 — usage/environment error (bad flag, no TTY, bad value)
const (
	exitSuccess   = 0
	exitOpFailure = 1
	exitUsage     = 2
)

func main() {
	days := flag.Int("days", 5, "look back this many days for merged Homebrew formulae")
	limit := flag.Int("limit", 50, "maximum number of pull requests to inspect")
	noCache := flag.Bool("no-cache", false, "disable cache reads and writes")
	showVersion := flag.Bool("version", false, "print version and exit")
	plain := flag.Bool("plain", false, "output plain text (one formula per line, tab-separated fields)")
	jsonOut := flag.Bool("json", false, "output JSON array of formula objects")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if *plain && *jsonOut {
		fmt.Fprintln(os.Stderr, "Error: --plain and --json are mutually exclusive")
		os.Exit(exitUsage)
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if !isTTY && !*plain && !*jsonOut {
		fmt.Fprintln(os.Stderr, "newbrew needs a terminal. Use --plain for scriptable output or --json for structured output.")
		os.Exit(exitUsage)
	}

	f := fetcher.New(fetcher.Config{
		Days:  *days,
		Limit: *limit,
	})

	if *plain || *jsonOut {
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
			for _, formula := range result.Formulae {
				fmt.Printf("%s\t%s\t%s\t%s\n",
					formula.PRTitle,
					formula.Desc,
					formula.Homepage,
					formula.MergedAt.Format("2006-01-02"),
				)
			}
		} else {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result.Formulae)
		}

		return
	}

	// TUI mode
	model := tui.NewModel(tui.Config{
		Days:     *days,
		Limit:    *limit,
		UseCache: !*noCache,
		Fetcher:  f,
	})

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(exitOpFailure)
	}
}
