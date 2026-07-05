package hair

import "time"

type Item struct {
	ID                   int64     `json:"-"`
	PublicID             string    `json:"public_id"`
	UserID               int64     `json:"-"`
	Name                 string    `json:"name"`
	Length               string    `json:"length,omitempty"`
	Shape                string    `json:"shape,omitempty"`
	Bangs                string    `json:"bangs,omitempty"`
	Color                string    `json:"color,omitempty"`
	CareTime             string    `json:"care_time,omitempty"`
	SceneTags            []string  `json:"scene_tags,omitempty"`
	SuitabilityNotes     string    `json:"suitability_notes,omitempty"`
	AvoidanceNotes       string    `json:"avoidance_notes,omitempty"`
	UserNotes            string    `json:"user_notes,omitempty"`
	RecommendationStatus string    `json:"recommendation_status"`
	Status               string    `json:"status"`
	PrimaryImage         *Image    `json:"primary_image,omitempty"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

type Image struct {
	AssetPublicID string `json:"asset_public_id"`
	ObjectKey     string `json:"object_key,omitempty"`
	PreviewURL    string `json:"preview_url,omitempty"`
	OriginalURL   string `json:"original_url,omitempty"`
}

type CreateInput struct {
	Name                 string   `json:"name"`
	Length               string   `json:"length"`
	Shape                string   `json:"shape"`
	Bangs                string   `json:"bangs"`
	Color                string   `json:"color"`
	CareTime             string   `json:"care_time"`
	SceneTags            []string `json:"scene_tags"`
	SuitabilityNotes     string   `json:"suitability_notes"`
	AvoidanceNotes       string   `json:"avoidance_notes"`
	UserNotes            string   `json:"user_notes"`
	RecommendationStatus string   `json:"recommendation_status"`
	PrimaryAssetPublicID string   `json:"primary_asset_public_id"`
}

type UpdateInput struct {
	Name                 *string   `json:"name"`
	Length               *string   `json:"length"`
	Shape                *string   `json:"shape"`
	Bangs                *string   `json:"bangs"`
	Color                *string   `json:"color"`
	CareTime             *string   `json:"care_time"`
	SceneTags            *[]string `json:"scene_tags"`
	SuitabilityNotes     *string   `json:"suitability_notes"`
	AvoidanceNotes       *string   `json:"avoidance_notes"`
	UserNotes            *string   `json:"user_notes"`
	RecommendationStatus *string   `json:"recommendation_status"`
	PrimaryAssetPublicID *string   `json:"primary_asset_public_id"`
}

type ListFilter struct{}
