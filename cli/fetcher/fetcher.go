package fetcher

import (
	"bufio"
	"encoding/json"
	"fmt"
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
	defaultTimeout     = 15 * time.Second
	maxFormulaFetchers = 8
	missingDesc        = "(not found)"
	missingHomepage    = "(not found)"
)

type CacheInterface interface {
	Save([]models.FormulaInfo) error
}

type Config struct {
	HTTPClient *http.Client
	Now        func() time.Time
	Days       int
	Limit      int
	Token      string
}

type Result struct {
	Formulae []models.FormulaInfo
	Warnings []string
}

type Fetcher struct {
	httpClient *http.Client
	now        func() time.Time
	days       int
	limit      int
	token      string
	apiBaseURL string
	repoOwner  string
	repoName   string
}

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
	}

	token := config.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	return &Fetcher{
		httpClient: httpClient,
		now:        now,
		days:       days,
		limit:      limit,
		token:      token,
		apiBaseURL: "https://api.github.com",
		repoOwner:  defaultRepoOwner,
		repoName:   defaultRepoName,
	}
}

func Default() *Fetcher {
	return New(Config{})
}

func FetchAndCache(c CacheInterface) ([]models.FormulaInfo, error) {
	result, err := Default().FetchAndCache(c)
	if err != nil {
		return nil, err
	}
	return result.Formulae, nil
}

func (f *Fetcher) FetchAndCache(c CacheInterface) (Result, error) {
	since := f.now().Add(-time.Duration(f.days) * 24 * time.Hour)

	prNumbers, info, err := f.fetchNewFormulaPRs(since)
	if err != nil {
		return Result{}, err
	}

	var result Result
	var prFileFailures int

	for _, prNum := range prNumbers {
		files, ferr := f.fetchPRFiles(prNum)
		if ferr != nil {
			prFileFailures++
			result.Warnings = append(result.Warnings, ferr.Error())
			continue
		}

		base := info[prNum]
		formulae, warnings := f.fetchFormulaInfoParallel(files, base)
		result.Formulae = append(result.Formulae, formulae...)
		result.Warnings = append(result.Warnings, warnings...)
	}

	if prFileFailures > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("skipped %d pull request file list(s) due to upstream errors", prFileFailures))
	}

	if c != nil {
		if err := c.Save(result.Formulae); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("cache save failed: %v", err))
		}
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

	resp, err := f.githubRequest(searchURL.String())
	if err != nil {
		return nil, nil, fmt.Errorf("GitHub search request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	reDesc := regexp.MustCompile(`^\s*desc\s+["']([^"']+)["']`)
	reHome := regexp.MustCompile(`^\s*homepage\s+["']([^"']+)["']`)

	for scanner.Scan() {
		line := scanner.Text()
		if desc == "" {
			if m := reDesc.FindStringSubmatch(line); m != nil {
				desc = m[1]
			}
		}
		if homepage == "" {
			if m := reHome.FindStringSubmatch(line); m != nil {
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

func (f *Fetcher) githubRequest(requestURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	return f.httpClient.Do(req)
}

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
