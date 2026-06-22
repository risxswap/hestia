package asset

type Asset struct {
	ID           int64
	PublicID     string
	ClientRef    string
	OwnerUserID  int64
	Bucket       string
	ObjectKey    string
	MimeType     string
	FileSize     int64
	Width        *int
	Height       *int
	AssetType    string
	Source       string
	Status       string
	ReviewStatus string
	Note         string
	Metadata     map[string]any
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
