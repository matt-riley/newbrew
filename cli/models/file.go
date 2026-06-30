package models

// File represents a single file changed within a GitHub pull request, as returned
// by the /repos/{owner}/{repo}/pulls/{number}/files API endpoint.
type File struct {
	Filename string `json:"filename"` // Path of the changed file within the repository
	Status   string `json:"status"`   // Change status: "added", "modified", "removed", etc.
	RawURL   string `json:"raw_url"`  // URL to fetch the raw file content
}
