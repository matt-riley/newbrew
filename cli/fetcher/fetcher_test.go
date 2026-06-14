package fetcher

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matt-riley/newbrew/models"
)

type stubCache struct {
	saved []models.FormulaInfo
	err   error
}

func (s *stubCache) Save(formulae []models.FormulaInfo) error {
	s.saved = append([]models.FormulaInfo(nil), formulae...)
	return s.err
}

func newTestFetcher(client *http.Client, baseURL string) *Fetcher {
	f := New(Config{
		HTTPClient: client,
		Now: func() time.Time {
			return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
		},
		Days:  5,
		Limit: 7,
	})
	f.apiBaseURL = baseURL
	return f
}

func TestFetchAndCacheEncodesQueryAndParsesFormulae(t *testing.T) {
	var gotQuery string
	var gotUserAgent string
	var baseURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			gotQuery = r.URL.Query().Get("q")
			gotUserAgent = r.Header.Get("User-Agent")
			if got := r.URL.Query().Get("per_page"); got != "7" {
				t.Fatalf("expected per_page 7, got %q", got)
			}
			_, _ = io.WriteString(w, `{"items":[{"number":101,"title":"foo 1.0.0 (new formula)","closed_at":"2026-06-14T10:00:00Z"}]}`)
		case "/repos/Homebrew/homebrew-core/pulls/101/files":
			_, _ = io.WriteString(w, `[{"filename":"Formula/foo.rb","status":"added","raw_url":"`+baseURL+`/raw/foo.rb"}]`)
		case "/raw/foo.rb":
			_, _ = io.WriteString(w, "class Foo < Formula\n  desc \"Fresh tool\"\n  homepage \"https://foo.example.com\"\nend\n")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	baseURL = server.URL

	cache := &stubCache{}
	result, err := newTestFetcher(server.Client(), server.URL).FetchAndCache(cache)
	if err != nil {
		t.Fatalf("FetchAndCache returned error: %v", err)
	}

	wantQuery := `repo:Homebrew/homebrew-core is:pr is:closed label:"new formula" merged:>=2026-06-09`
	if gotQuery != wantQuery {
		t.Fatalf("expected query %q, got %q", wantQuery, gotQuery)
	}
	if gotUserAgent == "" {
		t.Fatalf("expected GitHub request to set User-Agent header")
	}
	if len(result.Formulae) != 1 {
		t.Fatalf("expected one formula, got %d", len(result.Formulae))
	}
	if result.Formulae[0].Desc != "Fresh tool" {
		t.Fatalf("expected parsed desc, got %q", result.Formulae[0].Desc)
	}
	if result.Formulae[0].Homepage != "https://foo.example.com" {
		t.Fatalf("expected parsed homepage, got %q", result.Formulae[0].Homepage)
	}
	if len(cache.saved) != 1 {
		t.Fatalf("expected cache save to receive one formula, got %d", len(cache.saved))
	}
}

func TestNewClampsLimitToGitHubSearchMaximum(t *testing.T) {
	f := New(Config{Limit: 250})
	if f.limit != 100 {
		t.Fatalf("expected limit to be clamped to 100, got %d", f.limit)
	}
}

func TestFetchAndCacheReturnsErrorOnGithubFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := newTestFetcher(server.Client(), server.URL).FetchAndCache(&stubCache{})
	if err == nil {
		t.Fatalf("expected FetchAndCache to return an error")
	}
	if !strings.Contains(err.Error(), "GitHub search") {
		t.Fatalf("expected GitHub search error, got %v", err)
	}
}

func TestFetchAndCacheReturnsRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer server.Close()

	_, err := newTestFetcher(server.Client(), server.URL).FetchAndCache(&stubCache{})
	if err == nil {
		t.Fatalf("expected FetchAndCache to return an error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "rate limit") {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}

func TestFetchAndCacheContinuesWhenPRFilesFail(t *testing.T) {
	var baseURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			_, _ = io.WriteString(w, `{"items":[{"number":101,"title":"broken","closed_at":"2026-06-14T10:00:00Z"},{"number":102,"title":"working","closed_at":"2026-06-14T11:00:00Z"}]}`)
		case "/repos/Homebrew/homebrew-core/pulls/101/files":
			http.Error(w, "broken files", http.StatusBadGateway)
		case "/repos/Homebrew/homebrew-core/pulls/102/files":
			_, _ = io.WriteString(w, `[{"filename":"Formula/bar.rb","status":"added","raw_url":"`+baseURL+`/raw/bar.rb"}]`)
		case "/raw/bar.rb":
			_, _ = io.WriteString(w, "class Bar < Formula\n  desc \"Bar\"\n  homepage \"https://bar.example.com\"\nend\n")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	baseURL = server.URL

	result, err := newTestFetcher(server.Client(), server.URL).FetchAndCache(&stubCache{})
	if err != nil {
		t.Fatalf("FetchAndCache returned error: %v", err)
	}
	if len(result.Formulae) != 1 {
		t.Fatalf("expected one surviving formula, got %d", len(result.Formulae))
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected partial failure warning")
	}
}

func TestFetchFormulaMetadataUsesFallbacksWhenFieldsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "class Foo < Formula\n  desc \"Only desc\"\nend\n")
	}))
	defer server.Close()

	desc, homepage, err := newTestFetcher(server.Client(), server.URL).fetchFormulaMetadata(server.URL)
	if err != nil {
		t.Fatalf("fetchFormulaMetadata returned error: %v", err)
	}
	if desc != "Only desc" {
		t.Fatalf("expected desc to be parsed, got %q", desc)
	}
	if homepage != missingHomepage {
		t.Fatalf("expected missing homepage fallback, got %q", homepage)
	}
}

func TestFetchFormulaMetadataReturnsScannerError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &errReadCloser{},
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, _, err := newTestFetcher(client, "https://example.test").fetchFormulaMetadata("https://example.test/raw/foo.rb")
	if err == nil {
		t.Fatalf("expected fetchFormulaMetadata to return scanner error")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestFetchAndCacheReturnsWarningWhenCacheSaveFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			_, _ = io.WriteString(w, `{"items":[]}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := newTestFetcher(server.Client(), server.URL).FetchAndCache(&stubCache{err: errors.New("disk full")})
	if err != nil {
		t.Fatalf("FetchAndCache returned error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warning for cache save failure")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type errReadCloser struct{}

func (e *errReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (e *errReadCloser) Close() error {
	return nil
}
