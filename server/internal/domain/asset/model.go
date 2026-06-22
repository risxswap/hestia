package asset

type Asset struct {
	ID           int64
	PublicID     string
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
}

type Input struct {
	ObjectKey string
	MimeType  string
	FileSize  int64
	Width     *int
	Height    *int
	AssetType string
}
