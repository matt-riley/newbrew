// usage.go — rich help text for newbrew, overriding pflag's default Usage.
package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `newbrew — browse recently-merged Homebrew formulae from your terminal.

Usage:
  newbrew [flags]

Flags:
  -d, --days int        look back this many days for merged Homebrew formulae (default 5)
  -l, --limit int       maximum number of pull requests to inspect (default 50)
  -n, --no-cache        disable cache reads and writes
      --plain           output plain text (one formula per line, tab-separated)
      --json            output JSON array of formula objects
  -v, --version         print version information and exit

Environment variables:
  GITHUB_TOKEN     GitHub personal access token. When set, requests are
                   authenticated, raising the API rate limit from 60 to
                   5000 requests/hour. Optional but recommended.
  XDG_CACHE_HOME   Overrides the base cache directory (defaults to ~/.cache).
                   newbrew stores its cache at $XDG_CACHE_HOME/newbrew/formulae.json
                   and considers it fresh for 10 minutes.

Examples:
  newbrew -d 7 -n        Browse formulae merged in the last 7 days without cache
  newbrew --limit 50     Inspect up to 50 pull requests (default window)
  newbrew --plain        Script-friendly plain-text output for piping
  newbrew --json | jq .  Structured JSON output for programmatic consumption
`)
	}
}
