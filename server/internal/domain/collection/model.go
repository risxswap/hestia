package collection

type Summary struct {
	Types       []TypeSummary `json:"types"`
	RecentItems []RecentItem  `json:"recent_items"`
}

type TypeSummary struct {
	Type      string `json:"type"`
	Label     string `json:"label"`
	Count     int    `json:"count"`
	Hint      string `json:"hint"`
	Enabled   bool   `json:"enabled"`
	EntryPath string `json:"entry_path"`
}

type RecentItem struct {
	Type      string       `json:"type"`
	PublicID  string       `json:"public_id"`
	Title     string       `json:"title"`
	Subtitle  string       `json:"subtitle"`
	Image     *RecentImage `json:"image,omitempty"`
	EntryPath string       `json:"entry_path"`
}

type RecentImage struct {
	PreviewURL string `json:"preview_url,omitempty"`
}
