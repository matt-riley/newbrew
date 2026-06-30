package models

import "time"

// SearchResult represents the response from a GitHub search/issues API query.
// Only the fields consumed by newbrew are decoded; the full response may contain
// additional metadata.
type SearchResult struct {
	Items []struct {
		Number   int       `json:"number"`
		Title    string    `json:"title"`
		HTMLURL  string    `json:"html_url"`
		MergedAt time.Time `json:"closed_at"`
		User     struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"items"`
}
