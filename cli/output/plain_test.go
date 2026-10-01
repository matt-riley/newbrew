package output

import (
	"bytes"
	"testing"
	"time"

	"github.com/matt-riley/newbrew/models"
)

func fixedDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestWritePlain_SingleFormula(t *testing.T) {
	formulas := []models.FormulaInfo{
		{
			PRTitle:  "foo 1.0.0 (new formula)",
			Desc:     "A useful tool",
			Homepage: "https://example.com",
			MergedAt: fixedDate(2026, 6, 15),
		},
	}

	var buf bytes.Buffer
	if err := WritePlain(&buf, formulas); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "foo 1.0.0 (new formula)\tA useful tool\thttps://example.com\t2026-06-15\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWritePlain_MultipleFormulas(t *testing.T) {
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
	if err := WritePlain(&buf, formulas); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "" +
		"alpha 1.0.0 (new formula)\tFirst tool\thttps://alpha.example\t2026-01-10\n" +
		"beta 2.0.0 (new formula)\tSecond tool\thttps://beta.example\t2026-02-20\n" +
		"gamma 3.0.0 (new formula)\tThird tool\thttps://gamma.example\t2026-03-30\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWritePlain_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePlain(&buf, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected empty output for nil slice, got %q", got)
	}

	buf.Reset()
	if err := WritePlain(&buf, []models.FormulaInfo{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected empty output for empty slice, got %q", got)
	}
}

func TestWritePlain_MissingFields(t *testing.T) {
	// Fields may be empty strings or zero time — output should still be stable.
	formulas := []models.FormulaInfo{
		{
			PRTitle:  "delta (new formula)",
			Desc:     "",
			Homepage: "(not found)",
			MergedAt: time.Time{},
		},
	}

	var buf bytes.Buffer
	if err := WritePlain(&buf, formulas); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "delta (new formula)\t\t(not found)\t0001-01-01\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
