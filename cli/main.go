// main.go — newbrew: browse recently-merged Homebrew formulae from your terminal.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"

	tea "charm.land/bubbletea/v2"
	flag "github.com/spf13/pflag"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/fetcher"
	"github.com/matt-riley/newbrew/output"
	"github.com/matt-riley/newbrew/tui"
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

func customUsage() {
	fmt.Fprintf(os.Stderr, `newbrew — discover newly-merged Homebrew formulae

Usage:
  newbrew [flags]

Flags:
  -d, --days N       look back this many days for merged Homebrew formulae (default: 5)
  -l, --limit N      maximum number of pull requests to inspect (default: 50)
  -n, --no-cache     disable cache reads and writes
      --plain         output plain text (one formula per line, tab-separated fields)
      --json          output JSON array of formula objects
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

// usageError prints the usage message followed by the error and exits with
// exitUsage (2), matching pflag's own error-handling model for parse errors.
func usageError(msg string) {
	flag.Usage()
	fmt.Fprintf(os.Stderr, "newbrew: %s\n", msg)
	os.Exit(exitUsage)
}

// longFlagNames maps long flag names to their single-character shorthands.
// If a user writes a long name with a single dash (e.g. -version instead of
// --version or -v), pflag would parse it as a shorthand group: -version
// becomes -v -e -r -s -i -o -n.  If every letter happened to be a registered
// shorthand, the token would be silently accepted as a bizarre flag
// combination.  We reject these explicitly before parsing so the error
// message is clear and the behaviour does not depend on which shorthands are
// registered.
var longFlagNames = map[string]string{
	"version":  "v",
	"days":     "d",
	"limit":    "l",
	"no-cache": "n",
	"plain":    "",
	"json":     "",
	"help":     "h",
}

// rejectSingleDashLongFlags scans os.Args for long flag names written with a
// single leading dash and exits with a usage error if any are found.  This
// runs before flag.Parse so pflag never gets a chance to misinterpret them.
func rejectSingleDashLongFlags() {
	for _, arg := range os.Args[1:] {
		// Skip -- (double dash) and non-flag args.
		if strings.HasPrefix(arg, "--") || !strings.HasPrefix(arg, "-") {
			continue
		}
		// arg is a single-dash token like -version or -d.  Extract the flag
		// name (strip leading dash and any =value suffix).
		name := strings.TrimPrefix(arg, "-")
		if i := strings.IndexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		// Skip genuine single-character shorthands.
		if len(name) <= 1 {
			continue
		}
		if short, ok := longFlagNames[name]; ok {
			flag.Usage()
			suggestion := "--" + name
			if short != "" {
				suggestion = "-" + short + " or --" + name
			}
			fmt.Fprintf(os.Stderr, "newbrew: unknown flag: -%s (use %s)\n", name, suggestion)
			os.Exit(exitUsage)
		}
	}
}

func main() {
	// Set the usage function early so pre-parse error paths (e.g. single-dash
	// long flag rejection) also print the help text.  If an init() in
	// usage.go has already overridden flag.Usage, that takes priority.
	flag.Usage = customUsage

	// Reject single-dash long flag names (e.g. -version) before pflag parses,
	// so they fail with a clear error instead of being misinterpreted as
	// shorthand groups.
	rejectSingleDashLongFlags()

	var showVersionFlag bool

	days := flag.IntP("days", "d", 5, "look back this many days for merged Homebrew formulae")
	limit := flag.IntP("limit", "l", 50, "maximum number of pull requests to inspect")
	noCache := flag.BoolP("no-cache", "n", false, "disable cache reads and writes")
	plain := flag.Bool("plain", false, "output plain text (one formula per line, tab-separated fields)")
	jsonOut := flag.Bool("json", false, "output JSON array of formula objects")
	flag.BoolVarP(&showVersionFlag, "version", "v", false, "print version information and exit")

	// pflag does not support multiple shorthands on one flag, so we register
	// -V as a hidden second flag that writes the same variable.  The flag
	// itself is invisible in --help so we document -V inside customUsage.
	flag.BoolVarP(&showVersionFlag, "version-V", "V", false, "print version information and exit")

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
		usageError("--plain and --json are mutually exclusive")
	}

	// Validate flags before constructing the fetcher.
	if *days <= 0 {
		usageError(fmt.Sprintf("--days must be a positive integer (got %d)", *days))
	}
	if *limit <= 0 {
		usageError(fmt.Sprintf("--limit must be a positive integer (got %d)", *limit))
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
		usageError("needs a terminal. Use --plain for scriptable output or --json for structured output.")
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
