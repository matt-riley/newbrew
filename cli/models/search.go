package models

import "time"

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
