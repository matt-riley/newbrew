// Package output provides format-specific writers for newbrew formula data.
package output

import (
	"fmt"
	"io"

	"github.com/matt-riley/newbrew/models"
)

// WritePlain writes formula metadata to w as tab-separated values, one formula
// per line. Fields: PRTitle, Desc, Homepage, MergedAt (RFC 3339 date).
// An empty list produces no output (zero lines written, no error).
func WritePlain(w io.Writer, formulas []models.FormulaInfo) error {
	for _, f := range formulas {
		line := fmt.Sprintf("%s\t%s\t%s\t%s\n",
			f.PRTitle,
			f.Desc,
			f.Homepage,
			f.MergedAt.Format("2006-01-02"),
		)
		if _, err := io.WriteString(w, line); err != nil {
			return fmt.Errorf("write plain output: %w", err)
		}
	}
	return nil
}
