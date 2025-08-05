package models

type File struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`
	RawURL   string `json:"raw_url"`
}
