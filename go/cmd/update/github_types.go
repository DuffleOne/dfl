package main

type GitHubRelease struct {
	TagName         string  `json:"tag_name"`
	TargetCommitish string  `json:"target_commitish"`
	Name            string  `json:"name"`
	Draft           bool    `json:"draft"`
	Prerelease      bool    `json:"prerelease"`
	Assets          []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	State              string `json:"state"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}
