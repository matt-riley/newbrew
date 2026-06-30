// Package models provides shared data types for the newbrew Homebrew formula browser.
package models

import "time"

// FormulaInfo holds metadata about a single Homebrew formula pull request.
type FormulaInfo struct {
	PRTitle  string    // Title of the pull request (e.g. "foo 1.0.0 (new formula)")
	Desc     string    // Description string extracted from the formula's Ruby source
	Homepage string    // Homepage URL extracted from the formula's Ruby source
	MergedAt time.Time // When the pull request was merged
}
