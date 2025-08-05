package fetcher

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/matt-riley/newbrew/models"
)

const (
	repoOwner = "Homebrew"
	repoName  = "homebrew-core"
	days      = 5
)

type Fetcher struct{}

type CacheInterface interface {
	Save([]models.FormulaInfo) error
}

func githubRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{}
	return client.Do(req)
}

func fetchNewFormulaPRs(since time.Time) ([]int, map[int]models.FormulaInfo, error) {
	dateStr := since.Format("2006-01-02")
	query := fmt.Sprintf(
		"repo:%s/%s is:pr is:closed label:\"new formula\" merged:>=%s",
		repoOwner, repoName, dateStr,
	)
	url := fmt.Sprintf("https://api.github.com/search/issues?q=%s&per_page=50", strings.ReplaceAll(query, " ", "+"))
	resp, err := githubRequest(url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	var result models.SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}
	var prNumbers []int
	info := make(map[int]models.FormulaInfo)
	for _, item := range result.Items {
		prNumbers = append(prNumbers, item.Number)
		info[item.Number] = models.FormulaInfo{
			PRTitle:  item.Title,
			MergedAt: item.MergedAt,
		}
	}
	return prNumbers, info, nil
}

func fetchPRFiles(prNumber int) ([]models.File, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/pulls/%d/files",
		repoOwner, repoName, prNumber,
	)
	resp, err := githubRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var files []models.File
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, err
	}
	return files, nil
}

func fetchDescAndHomepageFromFormula(rawURL string) (desc, homepage string, err error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
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
	if desc == "" {
		desc = "(not found)"
	}
	if homepage == "" {
		homepage = "(not found)"
	}
	return desc, homepage, nil
}

func fetchFormulaInfoParallel(files []models.File, base models.FormulaInfo) []models.FormulaInfo {
	var wg sync.WaitGroup
	results := make(chan models.FormulaInfo, len(files))

	// Limit concurrency to 8 (or whatever you like)
	sem := make(chan struct{}, 8)

	for _, file := range files {
		if file.Status == "added" && strings.HasPrefix(file.Filename, "Formula/") && strings.HasSuffix(file.Filename, ".rb") {
			wg.Add(1)
			go func(file models.File) {
				defer wg.Done()
				sem <- struct{}{} // acquire
				desc, homepage, err := fetchDescAndHomepageFromFormula(file.RawURL)
				<-sem // release
				f := base
				if err != nil {
					desc = "(error fetching desc)"
					homepage = "(error fetching homepage)"
				}
				f.Desc = desc
				f.Homepage = homepage
				results <- f
			}(file)
		}
	}

	wg.Wait()
	close(results)

	var formulae []models.FormulaInfo
	for f := range results {
		formulae = append(formulae, f)
	}
	return formulae
}

func FetchAndCache(c CacheInterface) ([]models.FormulaInfo, error) {
	since := time.Now().Add(-days * 24 * time.Hour)
	prNumbers, info, err := fetchNewFormulaPRs(since)
	if err != nil {
		return nil, err
	}
	var allFormulae []models.FormulaInfo
	for _, prNum := range prNumbers {
		files, ferr := fetchPRFiles(prNum)
		if ferr != nil {
			continue
		}
		base := info[prNum]
		formulae := fetchFormulaInfoParallel(files, base)
		allFormulae = append(allFormulae, formulae...)
	}
	_ = c.Save(allFormulae)
	return allFormulae, nil
}
