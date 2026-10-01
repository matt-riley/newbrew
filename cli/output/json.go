package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/matt-riley/newbrew/models"
)

// formulaJSON is the JSON-serialisable representation of a single formula,
// mapping FormulaInfo fields to the expected JSON keys.
type formulaJSON struct {
	PRTitle     string `json:"PRTitle"`
	Description string `json:"Description"`
	Homepage    string `json:"Homepage"`
	MergedAt    string `json:"MergedAt"`
}

// WriteJSON writes a JSON array of formula metadata to w. Each object contains
// PRTitle, Description, Homepage, and MergedAt (RFC 3339) keys. An empty or
// nil slice is written as an empty JSON array ([]). The output is a single
// compact line suitable for piping through jq.
func WriteJSON(w io.Writer, formulas []models.FormulaInfo) error {
	if formulas == nil {
		formulas = []models.FormulaInfo{}
	}

	out := make([]formulaJSON, len(formulas))
	for i, f := range formulas {
		out[i] = formulaJSON{
			PRTitle:     f.PRTitle,
			Description: f.Desc,
			Homepage:    f.Homepage,
			MergedAt:    formatMergedAt(f.MergedAt),
		}
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("write JSON output: %w", err)
	}
	return nil
}

// formatMergedAt returns the RFC 3339 representation of t, or an empty string
// for the zero time so that an empty MergedAt field appears as "" rather than
// "0001-01-01T00:00:00Z".
func formatMergedAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
