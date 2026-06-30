package fetcher

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/matt-riley/newbrew/cache"
	"github.com/matt-riley/newbrew/models"
)

// TestIntegrationFullPipeline exercises the complete fetch→parse→cache→read
// pipeline using an httptest.Server and a real on-disk cache.
func TestIntegrationFullPipeline(t *testing.T) {
	// Point the OS cache directory at a temp dir so we can inspect it.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	var baseURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			if got := r.URL.Query().Get("per_page"); got != "7" {
				t.Errorf("expected per_page 7, got %q", got)
			}
			_, _ = io.WriteString(w, `{"items":[
				{"number":201,"title":"foo 2.0 (new formula)","closed_at":"2026-06-10T12:00:00Z"},
				{"number":202,"title":"bar 1.5 (new formula)","closed_at":"2026-06-11T08:00:00Z"}
			]}`)
		case "/repos/Homebrew/homebrew-core/pulls/201/files":
			_, _ = io.WriteString(w, `[{"filename":"Formula/foo.rb","status":"added","raw_url":"`+baseURL+`/raw/foo.rb"}]`)
		case "/repos/Homebrew/homebrew-core/pulls/202/files":
			_, _ = io.WriteString(w, `[{"filename":"Formula/bar.rb","status":"added","raw_url":"`+baseURL+`/raw/bar.rb"}]`)
		case "/raw/foo.rb":
			_, _ = io.WriteString(w, "class Foo < Formula\n  desc \"The foo tool\"\n  homepage \"https://foo.example.com\"\nend\n")
		case "/raw/bar.rb":
			_, _ = io.WriteString(w, "class Bar < Formula\n  desc \"Bar utility\"\n  homepage \"https://bar.example.com\"\nend\n")
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	baseURL = server.URL

	// Build a fetcher with a pinned clock that hits our test server.
	f := New(Config{
		HTTPClient: server.Client(),
		Now: func() time.Time {
			return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
		},
		Days:  5,
		Limit: 7,
	})
	f.apiBaseURL = server.URL

	// --- Phase 1: Fetch + cache ---
	c, err := cache.NewCache()
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	result, err := f.FetchAndCache(c)
	if err != nil {
		t.Fatalf("FetchAndCache: %v", err)
	}

	if len(result.Formulae) != 2 {
		t.Fatalf("expected 2 formulae, got %d", len(result.Formulae))
	}

	// --- Phase 2: Read back from disk ---
	loaded, err := cache.NewCache()
	if err != nil {
		t.Fatalf("second NewCache (disk read): %v", err)
	}

	if !loaded.IsFresh() {
		t.Fatal("expected cached data to be fresh after save")
	}

	if len(loaded.Formulae) != 2 {
		t.Fatalf("expected 2 cached formulae, got %d", len(loaded.Formulae))
	}

	// Verify both formulae are intact.
	byTitle := make(map[string]models.FormulaInfo)
	for _, fi := range loaded.Formulae {
		byTitle[fi.PRTitle] = fi
	}

	foo, ok := byTitle["foo 2.0 (new formula)"]
	if !ok {
		t.Fatal("missing foo in cached data")
	}
	if foo.Desc != "The foo tool" {
		t.Errorf("foo desc: got %q, want %q", foo.Desc, "The foo tool")
	}
	if foo.Homepage != "https://foo.example.com" {
		t.Errorf("foo homepage: got %q, want %q", foo.Homepage, "https://foo.example.com")
	}

	bar, ok := byTitle["bar 1.5 (new formula)"]
	if !ok {
		t.Fatal("missing bar in cached data")
	}
	if bar.Desc != "Bar utility" {
		t.Errorf("bar desc: got %q, want %q", bar.Desc, "Bar utility")
	}

	// --- Phase 3: Cache hit — stale read still loads from disk ---
	// The cache file should exist on disk.
	cacheFile := tmpDir + "/newbrew/formulae.json"
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Fatalf("expected cache file at %s to exist", cacheFile)
	}
}

// TestIntegrationFetchAndCacheWithNilCache exercises the nil-cache path
// where cache is disabled (no persistence).
func TestIntegrationFetchAndCacheWithNilCache(t *testing.T) {
	var baseURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			_, _ = io.WriteString(w, `{"items":[
				{"number":301,"title":"baz 0.1 (new formula)","closed_at":"2026-06-12T10:00:00Z"}
			]}`)
		case "/repos/Homebrew/homebrew-core/pulls/301/files":
			_, _ = io.WriteString(w, `[{"filename":"Formula/baz.rb","status":"added","raw_url":"`+baseURL+`/raw/baz.rb"}]`)
		case "/raw/baz.rb":
			_, _ = io.WriteString(w, "class Baz < Formula\n  desc \"Baz tool\"\n  homepage \"https://baz.example.com\"\nend\n")
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	baseURL = server.URL

	f := New(Config{
		HTTPClient: server.Client(),
		Now: func() time.Time {
			return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
		},
		Days:  5,
		Limit: 7,
	})
	f.apiBaseURL = server.URL

	// nil cache should not panic and should return results.
	result, err := f.FetchAndCache(nil)
	if err != nil {
		t.Fatalf("FetchAndCache(nil): %v", err)
	}
	if len(result.Formulae) != 1 {
		t.Fatalf("expected 1 formula with nil cache, got %d", len(result.Formulae))
	}
}
