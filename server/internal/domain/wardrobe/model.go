package wardrobe

import "time"

type Item struct {
	ID                     int64     `json:"-"`
	PublicID               string    `json:"public_id"`
	UserID                 int64     `json:"-"`
	Name                   string    `json:"name"`
	Category               string    `json:"category"`
	Color                  string    `json:"color,omitempty"`
	Silhouette             string    `json:"silhouette,omitempty"`
	Material               string    `json:"material,omitempty"`
	Season                 string    `json:"season,omitempty"`
	SceneTags              []string  `json:"scene_tags,omitempty"`
	UserNotes              string    `json:"user_notes,omitempty"`
	IsCore                 bool      `json:"is_core"`
	RecommendationStatus   string    `json:"recommendation_status"`
	RecognitionStatus      string    `json:"recognition_status"`
	RecognitionJobPublicID string    `json:"recognition_job_public_id,omitempty"`
	Status                 string    `json:"status"`
	PrimaryImage           *Image    `json:"primary_image,omitempty"`
	CreatedAt              time.Time `json:"created_at,omitempty"`
	UpdatedAt              time.Time `json:"updated_at,omitempty"`
}

type Image struct {
	AssetPublicID string `json:"asset_public_id"`
	ObjectKey     string `json:"object_key,omitempty"`
	PreviewURL    string `json:"preview_url,omitempty"`
	OriginalURL   string `json:"original_url,omitempty"`
}

type OptionItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type WardrobeOptions struct {
	Categories  []OptionItem `json:"categories"`
	Materials   []OptionItem `json:"materials"`
	Seasons     []OptionItem `json:"seasons"`
	Silhouettes []OptionItem `json:"silhouettes"`
}

type Input struct {
	Name       string
	Category   string
	Color      string
	Silhouette string
	Material   string
	Season     string
	Notes      string
}

type CreateInput struct {
	Name                 string   `json:"name"`
	Category             string   `json:"category"`
	Color                string   `json:"color"`
	Silhouette           string   `json:"silhouette"`
	Material             string   `json:"material"`
	Season               string   `json:"season"`
	SceneTags            []string `json:"scene_tags"`
	UserNotes            string   `json:"user_notes"`
	IsCore               *bool    `json:"is_core"`
	RecommendationStatus string   `json:"recommendation_status"`
	RecognitionStatus    string   `json:"recognition_status"`
	PrimaryAssetPublicID string   `json:"primary_asset_public_id"`
	AssetPublicIDs       []string `json:"asset_public_ids"`
}

type UpdateInput struct {
	Name                 *string   `json:"name"`
	Category             *string   `json:"category"`
	Color                *string   `json:"color"`
	Silhouette           *string   `json:"silhouette"`
	Material             *string   `json:"material"`
	Season               *string   `json:"season"`
	SceneTags            *[]string `json:"scene_tags"`
	UserNotes            *string   `json:"user_notes"`
	IsCore               *bool     `json:"is_core"`
	RecommendationStatus *string   `json:"recommendation_status"`
	PrimaryAssetPublicID *string   `json:"primary_asset_public_id"`
	RecognitionStatus    *string   `json:"recognition_status"`
}

type ListFilter struct {
	Category             string
	RecommendationStatus string
	IsCore               *bool
}

type AdviceContextFilter struct {
	Scene string
	Limit int
}

type RecognizeImageInput struct {
	AssetPublicID string `json:"asset_public_id"`
	ImageURL      string `json:"image_url,omitempty"`
	ItemPublicID  string `json:"item_public_id,omitempty"`
	Overwrite     bool   `json:"overwrite,omitempty"`
}

type RecognizedItemFields struct {
	Name       string   `json:"name,omitempty"`
	Category   string   `json:"category,omitempty"`
	Color      string   `json:"color,omitempty"`
	Silhouette string   `json:"silhouette,omitempty"`
	Material   string   `json:"material,omitempty"`
	Season     string   `json:"season,omitempty"`
	SceneTags  []string `json:"scene_tags,omitempty"`
	UserNotes  string   `json:"user_notes,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
}
