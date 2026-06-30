// Package fetcher queries the GitHub API for recently merged Homebrew "new
// formula" pull requests and extracts metadata from each formula's Ruby source.
package fetcher

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/matt-riley/newbrew/models"
)

const (
	defaultRepoOwner   = "Homebrew"
	defaultRepoName    = "homebrew-core"
	defaultDays        = 5
	defaultLimit       = 50
	maxSearchLimit     = 100
	defaultTimeout     = 15 * time.Second
	maxFormulaFetchers = 8
	missingDesc        = "(not found)"
	missingHomepage    = "(not found)"
)

var (
	descPattern     = regexp.MustCompile(`^\s*desc\s+["']([^"']+)["']`)
	homepagePattern = regexp.MustCompile(`^\s*homepage\s+["']([^"']+)["']`)

	// logger is the package-level structured logger. It defaults to
	// writing warnings and errors to stderr. Use SetLogger to control
	// the log level and output destination.
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
)

// SetLogger replaces the package-level structured logger. Pass nil to
// suppress all structured log output.
func SetLogger(l *slog.Logger) {
	if l == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
		return
	}
	logger = l
}

// CacheInterface is the contract that a cache implementation must satisfy for
// FetchAndCache to persist results.
type CacheInterface interface {
	Save([]models.FormulaInfo) error
}

// Config holds the tunable parameters for creating a Fetcher.
// All fields are optional; zero values trigger sensible defaults.
type Config struct {
	HTTPClient *http.Client     // HTTP client to use; a default with Timeout=15s is created when nil
	Now        func() time.Time // clock function for time-dependent queries; time.Now when nil
	Days       int              // look-back window in days; defaults to 5
	Limit      int              // max PRs to inspect; defaults to 50, capped at 100
	Token      string           // GitHub personal access token; read from GITHUB_TOKEN env when empty
	Version    string           // application version for the User-Agent header; "dev" when empty
}

// Result is the outcome of a FetchAndCache call.
type Result struct {
	Formulae []models.FormulaInfo // discovered formula metadata
	Warnings []string             // non-fatal problems encountered during the fetch (partial failures, cache errors)
}

// Fetcher queries the GitHub API for recently-merged "new formula" pull requests
// and extracts desc/homepage metadata from each formula's Ruby source.
// Create one with New.
type Fetcher struct {
	httpClient *http.Client
	now        func() time.Time
	days       int
	limit      int
	token      string
	version    string
	apiBaseURL string
	repoOwner  string
	repoName   string
}

// New creates a Fetcher from the supplied Config. Every zero field is replaced
// with a sensible default (see Config for details).
func New(config Config) *Fetcher {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	} else if httpClient.Timeout == 0 {
		httpClient = &http.Client{
			Transport:     httpClient.Transport,
			CheckRedirect: httpClient.CheckRedirect,
			Jar:           httpClient.Jar,
			Timeout:       defaultTimeout,
		}
	}

	now := config.Now
	if now == nil {
		now = time.Now
	}

	days := config.Days
	if days <= 0 {
		days = defaultDays
	}

	limit := config.Limit
	if limit <= 0 {
		limit = defaultLimit
	} else if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	token := config.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	version := config.Version
	if version == "" {
		version = "dev"
	}

	return &Fetcher{
		httpClient: httpClient,
		now:        now,
		days:       days,
		limit:      limit,
		token:      token,
		version:    version,
		apiBaseURL: "https://api.github.com",
		repoOwner:  defaultRepoOwner,
		repoName:   defaultRepoName,
	}
}

// Default returns a Fetcher with all-zero Config (which triggers every default).
func Default() *Fetcher {
	return New(Config{})
}

// FetchAndCache is a convenience wrapper that creates a default Fetcher,
// calls FetchAndCache, and returns only the formula slice.
func FetchAndCache(c CacheInterface) ([]models.FormulaInfo, error) {
	result, err := Default().FetchAndCache(c)
	if err != nil {
		return nil, err
	}
	return result.Formulae, nil
}

// FetchAndCache searches for recently-merged "new formula" PRs, extracts
// metadata from each formula's Ruby source, and (when c is non-nil) persists
// the results through the supplied cache.
func (f *Fetcher) FetchAndCache(c CacheInterface) (Result, error) {
	since := f.now().Add(-time.Duration(f.days) * 24 * time.Hour)

	logger.Info("fetching new formula PRs",
		"since", since.Format("2006-01-02"),
		"days", f.days,
		"limit", f.limit,
	)

	prNumbers, info, err := f.fetchNewFormulaPRs(since)
	if err != nil {
		logger.Error("GitHub search failed", "error", err)
		return Result{}, err
	}

	logger.Info("search complete", "pr_count", len(prNumbers))

	var result Result
	var prFileFailures int

	for _, prNum := range prNumbers {
		files, ferr := f.fetchPRFiles(prNum)
		if ferr != nil {
			prFileFailures++
			logger.Warn("PR file fetch failed", "pr", prNum, "error", ferr)
			result.Warnings = append(result.Warnings, ferr.Error())
			continue
		}

		base := info[prNum]
		formulae, warnings := f.fetchFormulaInfoParallel(files, base)
		result.Formulae = append(result.Formulae, formulae...)
		result.Warnings = append(result.Warnings, warnings...)
	}

	if prFileFailures > 0 {
		msg := fmt.Sprintf("skipped %d pull request file list(s) due to upstream errors", prFileFailures)
		logger.Warn("partial failure", "skipped_pr_file_lists", prFileFailures)
		result.Warnings = append(result.Warnings, msg)
	}

	if c != nil {
		logger.Debug("saving to cache", "formula_count", len(result.Formulae))
		if err := c.Save(result.Formulae); err != nil {
			logger.Warn("cache save failed", "error", err)
			result.Warnings = append(result.Warnings, fmt.Sprintf("cache save failed: %v", err))
		}
	} else {
		logger.Debug("cache disabled — skipping save")
	}

	return result, nil
}

func (f *Fetcher) fetchNewFormulaPRs(since time.Time) ([]int, map[int]models.FormulaInfo, error) {
	dateStr := since.Format("2006-01-02")
	query := fmt.Sprintf(
		"repo:%s/%s is:pr is:closed label:\"new formula\" merged:>=%s",
		f.repoOwner, f.repoName, dateStr,
	)

	searchURL, err := url.Parse(f.apiBaseURL + "/search/issues")
	if err != nil {
		return nil, nil, fmt.Errorf("build GitHub search URL: %w", err)
	}

	params := searchURL.Query()
	params.Set("q", query)
	params.Set("per_page", fmt.Sprintf("%d", f.limit))
	searchURL.RawQuery = params.Encode()

	logger.Debug("GitHub search request", "url", searchURL.String())

	resp, err := f.githubRequest(searchURL.String())
	if err != nil {
		return nil, nil, fmt.Errorf("GitHub search request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	logRemaining := resp.Header.Get("X-RateLimit-Remaining")
	logger.Debug("GitHub search response", "status", resp.StatusCode, "rate_limit_remaining", logRemaining)

	if err := checkGitHubResponse(resp, "GitHub search"); err != nil {
		return nil, nil, err
	}

	var result models.SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("GitHub search decode failed: %w", err)
	}

	var prNumbers []int
	info := make(map[int]models.FormulaInfo, len(result.Items))
	for _, item := range result.Items {
		prNumbers = append(prNumbers, item.Number)
		info[item.Number] = models.FormulaInfo{
			PRTitle:  item.Title,
			MergedAt: item.MergedAt,
		}
	}

	return prNumbers, info, nil
}

func (f *Fetcher) fetchPRFiles(prNumber int) ([]models.File, error) {
	url := fmt.Sprintf(
		"%s/repos/%s/%s/pulls/%d/files",
		f.apiBaseURL,
		f.repoOwner,
		f.repoName,
		prNumber,
	)

	resp, err := f.githubRequest(url)
	if err != nil {
		return nil, fmt.Errorf("pull request %d file request failed: %w", prNumber, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := checkGitHubResponse(resp, fmt.Sprintf("pull request %d files", prNumber)); err != nil {
		return nil, err
	}

	var files []models.File
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("pull request %d file decode failed: %w", prNumber, err)
	}

	return files, nil
}

func (f *Fetcher) fetchFormulaMetadata(rawURL string) (desc, homepage string, err error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("build formula request: %w", err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch formula source: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("fetch formula source: unexpected status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if desc == "" {
			if m := descPattern.FindStringSubmatch(line); m != nil {
				desc = m[1]
			}
		}
		if homepage == "" {
			if m := homepagePattern.FindStringSubmatch(line); m != nil {
				homepage = m[1]
			}
		}
		if desc != "" && homepage != "" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("read formula source: %w", err)
	}

	if desc == "" {
		desc = missingDesc
	}
	if homepage == "" {
		homepage = missingHomepage
	}

	return desc, homepage, nil
}

func (f *Fetcher) fetchFormulaInfoParallel(files []models.File, base models.FormulaInfo) ([]models.FormulaInfo, []string) {
	var wg sync.WaitGroup
	results := make(chan models.FormulaInfo, len(files))
	warnings := make(chan string, len(files))
	sem := make(chan struct{}, maxFormulaFetchers)

	for _, file := range files {
		if !isFormulaFile(file) {
			continue
		}

		wg.Add(1)
		go func(file models.File) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			desc, homepage, err := f.fetchFormulaMetadata(file.RawURL)
			formula := base
			if err != nil {
				desc = "(error fetching desc)"
				homepage = "(error fetching homepage)"
				warnings <- fmt.Sprintf("formula metadata fetch failed for %s: %v", file.Filename, err)
			}
			formula.Desc = desc
			formula.Homepage = homepage
			results <- formula
		}(file)
	}

	wg.Wait()
	close(results)
	close(warnings)

	formulae := make([]models.FormulaInfo, 0, len(results))
	for formula := range results {
		formulae = append(formulae, formula)
	}

	collectedWarnings := make([]string, 0, len(warnings))
	for warning := range warnings {
		collectedWarnings = append(collectedWarnings, warning)
	}

	return formulae, collectedWarnings
}

// userAgentString builds the User-Agent header value from the fetcher's
// version, following GitHub's recommendation to include a contact URL.
func (f *Fetcher) userAgentString() string {
	return "newbrew/" + f.version + " (+https://github.com/matt-riley/newbrew)"
}

func (f *Fetcher) githubRequest(requestURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", f.userAgentString())

	return f.httpClient.Do(req)
}

// isFormulaFile reports whether a changed file represents a newly-added
// Homebrew formula (status "added", under Formula/, with .rb extension).
func isFormulaFile(file models.File) bool {
	return file.Status == "added" &&
		strings.HasPrefix(file.Filename, "Formula/") &&
		strings.HasSuffix(file.Filename, ".rb")
}

func checkGitHubResponse(resp *http.Response, action string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return fmt.Errorf("%s failed: GitHub API rate limit exceeded", action)
	}

	return fmt.Errorf("%s failed: unexpected status %d", action, resp.StatusCode)
}
