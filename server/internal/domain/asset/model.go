package asset

type Asset struct {
	ID           int64          `json:"-"`
	PublicID     string         `json:"public_id"`
	ClientRef    string         `json:"client_ref,omitempty"`
	OwnerUserID  int64          `json:"owner_user_id"`
	Bucket       string         `json:"bucket"`
	ObjectKey    string         `json:"object_key"`
	MimeType     string         `json:"mime_type"`
	FileSize     int64          `json:"file_size"`
	Width        *int           `json:"width,omitempty"`
	Height       *int           `json:"height,omitempty"`
	AssetType    string         `json:"asset_type"`
	Source       string         `json:"source"`
	Status       string         `json:"status"`
	ReviewStatus string         `json:"review_status"`
	Note         string         `json:"note,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type Input struct {
	AssetPublicID string
	ClientRef     string
	ObjectKey     string
	MimeType      string
	FileSize      int64
	Width         *int
	Height        *int
	AssetType     string
	Note          string
}

type UploadTokenInput struct {
	AssetType string `json:"asset_type"`
	MimeType  string `json:"mime_type"`
	FileSize  int64  `json:"file_size"`
}

type UploadTokenResult struct {
	AssetPublicID string `json:"asset_public_id"`
	Bucket        string `json:"bucket"`
	ObjectKey     string `json:"object_key"`
	UploadURL     string `json:"upload_url"`
	UploadToken   string `json:"upload_token"`
	ExpiresAt     string `json:"expires_at"`
}

type ConfirmInput struct {
	AssetPublicID string `json:"asset_public_id"`
	Bucket        string `json:"bucket"`
	ObjectKey     string `json:"object_key"`
	MimeType      string `json:"mime_type"`
	FileSize      int64  `json:"file_size"`
	Width         *int   `json:"width"`
	Height        *int   `json:"height"`
	AssetType     string `json:"asset_type"`
}

type ConfirmResult struct {
	AssetPublicID string `json:"asset_public_id"`
	ObjectKey     string `json:"object_key"`
	URL           string `json:"url"`
	AssetType     string `json:"asset_type"`
}
