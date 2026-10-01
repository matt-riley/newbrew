package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/matt-riley/newbrew/models"
)

func TestWriteJSON_SingleFormula(t *testing.T) {
	formulas := []models.FormulaInfo{
		{
			PRTitle:  "foo 1.0.0 (new formula)",
			Desc:     "A useful tool",
			Homepage: "https://example.com",
			MergedAt: fixedDate(2026, 6, 15),
		},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, formulas); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify valid JSON and round-trip parse.
	var parsed []formulaJSON
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 element, got %d", len(parsed))
	}

	got := parsed[0]
	if got.PRTitle != "foo 1.0.0 (new formula)" {
		t.Errorf("PRTitle = %q, want %q", got.PRTitle, "foo 1.0.0 (new formula)")
	}
	if got.Description != "A useful tool" {
		t.Errorf("Description = %q, want %q", got.Description, "A useful tool")
	}
	if got.Homepage != "https://example.com" {
		t.Errorf("Homepage = %q, want %q", got.Homepage, "https://example.com")
	}
	if got.MergedAt != "2026-06-15T00:00:00Z" {
		t.Errorf("MergedAt = %q, want %q", got.MergedAt, "2026-06-15T00:00:00Z")
	}
}

func TestWriteJSON_MultipleFormulas(t *testing.T) {
	formulas := []models.FormulaInfo{
		{
			PRTitle:  "alpha 1.0.0 (new formula)",
			Desc:     "First tool",
			Homepage: "https://alpha.example",
			MergedAt: fixedDate(2026, 1, 10),
		},
		{
			PRTitle:  "beta 2.0.0 (new formula)",
			Desc:     "Second tool",
			Homepage: "https://beta.example",
			MergedAt: fixedDate(2026, 2, 20),
		},
		{
			PRTitle:  "gamma 3.0.0 (new formula)",
			Desc:     "Third tool",
			Homepage: "https://gamma.example",
			MergedAt: fixedDate(2026, 3, 30),
		},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, formulas); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed []formulaJSON
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if len(parsed) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(parsed))
	}

	want := []struct{ title, desc, homepage, merged string }{
		{"alpha 1.0.0 (new formula)", "First tool", "https://alpha.example", "2026-01-10T00:00:00Z"},
		{"beta 2.0.0 (new formula)", "Second tool", "https://beta.example", "2026-02-20T00:00:00Z"},
		{"gamma 3.0.0 (new formula)", "Third tool", "https://gamma.example", "2026-03-30T00:00:00Z"},
	}
	for i, w := range want {
		if parsed[i].PRTitle != w.title {
			t.Errorf("[%d] PRTitle = %q, want %q", i, parsed[i].PRTitle, w.title)
		}
		if parsed[i].Description != w.desc {
			t.Errorf("[%d] Description = %q, want %q", i, parsed[i].Description, w.desc)
		}
		if parsed[i].Homepage != w.homepage {
			t.Errorf("[%d] Homepage = %q, want %q", i, parsed[i].Homepage, w.homepage)
		}
		if parsed[i].MergedAt != w.merged {
			t.Errorf("[%d] MergedAt = %q, want %q", i, parsed[i].MergedAt, w.merged)
		}
	}
}

func TestWriteJSON_EmptyList(t *testing.T) {
	// Nil slice → "[]"
	var buf bytes.Buffer
	if err := WriteJSON(&buf, nil); err != nil {
		t.Fatalf("unexpected error for nil slice: %v", err)
	}
	if got := buf.String(); got != "[]\n" {
		t.Errorf("nil slice: got %q, want %q", got, "[]\n")
	}

	buf.Reset()
	// Empty slice → "[]"
	if err := WriteJSON(&buf, []models.FormulaInfo{}); err != nil {
		t.Fatalf("unexpected error for empty slice: %v", err)
	}
	if got := buf.String(); got != "[]\n" {
		t.Errorf("empty slice: got %q, want %q", got, "[]\n")
	}
}

func TestWriteJSON_MissingFields(t *testing.T) {
	formulas := []models.FormulaInfo{
		{
			PRTitle:  "delta (new formula)",
			Desc:     "",
			Homepage: "(not found)",
			MergedAt: time.Time{},
		},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, formulas); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed []formulaJSON
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}

	got := parsed[0]
	if got.PRTitle != "delta (new formula)" {
		t.Errorf("PRTitle = %q, want %q", got.PRTitle, "delta (new formula)")
	}
	if got.Description != "" {
		t.Errorf("Description = %q, want empty string", got.Description)
	}
	if got.Homepage != "(not found)" {
		t.Errorf("Homepage = %q, want %q", got.Homepage, "(not found)")
	}
	if got.MergedAt != "" {
		t.Errorf("MergedAt = %q, want empty string for zero time", got.MergedAt)
	}
}

func TestWriteJSON_PipeableOutput(t *testing.T) {
	// Output must be a single line that jq can parse.
	formulas := []models.FormulaInfo{
		{
			PRTitle:  "foo 1.0.0 (new formula)",
			Desc:     "A useful tool",
			Homepage: "https://example.com",
			MergedAt: fixedDate(2026, 6, 15),
		},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, formulas); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := buf.String()

	// Single line only (no embedded newlines before trailing encoder newline).
	if bytes.Count(buf.Bytes(), []byte("\n")) > 1 {
		t.Errorf("output contains embedded newlines, won't work with line-oriented pipes: %q", raw)
	}

	// Must be valid JSON.
	if !json.Valid(buf.Bytes()) {
		t.Errorf("output is not valid JSON: %q", raw)
	}
}
