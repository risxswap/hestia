package profile

import (
	"encoding/json"
	"time"
)

type Profile struct {
	ID                 int64    `db:"id"`
	PublicID           string   `db:"public_id"`
	UserID             int64    `db:"user_id"`
	Status             string   `db:"status"`
	Gender             string   `db:"gender"`
	HeightCM           *int     `db:"height_cm"`
	WeightKG           *float64 `db:"weight_kg"`
	BodyNotes          string   `db:"body_notes"`
	SkinNotes          string   `db:"skin_notes"`
	HairNotes          string   `db:"hair_notes"`
	FaceShape          string   `db:"face_shape"`
	UpperBodyNotes     string   `db:"upper_body_notes"`
	LowerBodyNotes     string   `db:"lower_body_notes"`
	SizeNotes          string   `db:"size_notes"`
	LifestyleScenarios []string
	StyleGoalSummary   string `db:"style_goal_summary"`
}

type Fact struct {
	Key    string
	Value  any
	Source string
}

type Pref struct {
	Type       string
	Key        string
	Value      any
	Polarity   string
	Source     string
	Confidence *float64
}

type Inference struct {
	UserID      int64
	ProfileID   int64
	Key         string
	Value       any
	Confidence  *float64
	SourceJobID int64
}

type OnboardingInput struct {
	Gender             string
	HeightCM           *int
	BodyNotes          string
	SkinNotes          string
	HairNotes          string
	LifestyleScenarios []string
	StyleGoals         []string
	Avoidances         []string
}

type UpdateProfileRequest struct {
	Nickname           PatchString      `json:"nickname"`
	Gender             PatchString      `json:"gender"`
	HeightCM           PatchInt         `json:"height_cm"`
	WeightKG           PatchFloat       `json:"weight_kg"`
	BodyNotes          PatchString      `json:"body_notes"`
	SkinNotes          PatchString      `json:"skin_notes"`
	HairNotes          PatchString      `json:"hair_notes"`
	FaceShape          PatchString      `json:"face_shape"`
	UpperBodyNotes     PatchString      `json:"upper_body_notes"`
	LowerBodyNotes     PatchString      `json:"lower_body_notes"`
	SizeNotes          PatchString      `json:"size_notes"`
	LifestyleScenarios PatchStringSlice `json:"lifestyle_scenarios"`
}

type UpdatePreferencesRequest struct {
	StyleGoals          []string `json:"style_goals"`
	Avoidances          []string `json:"avoidances"`
	ScenarioPreferences []string `json:"scenario_preferences"`
}

type UpdateProfileInput struct {
	Nickname           PatchString
	Gender             PatchString
	HeightCM           PatchInt
	WeightKG           PatchFloat
	BodyNotes          PatchString
	SkinNotes          PatchString
	HairNotes          PatchString
	FaceShape          PatchString
	UpperBodyNotes     PatchString
	LowerBodyNotes     PatchString
	SizeNotes          PatchString
	LifestyleScenarios PatchStringSlice
}

type UpdatePreferencesInput struct {
	StyleGoals          []string
	Avoidances          []string
	ScenarioPreferences []string
}

type PatchString struct {
	Present bool
	Value   string
}

func (p *PatchString) UnmarshalJSON(raw []byte) error {
	p.Present = true
	if string(raw) == "null" {
		p.Value = ""
		return nil
	}
	return json.Unmarshal(raw, &p.Value)
}

type PatchInt struct {
	Present bool
	Value   *int
}

func (p *PatchInt) UnmarshalJSON(raw []byte) error {
	p.Present = true
	if string(raw) == "null" {
		p.Value = nil
		return nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	p.Value = &value
	return nil
}

type PatchFloat struct {
	Present bool
	Value   *float64
}

func (p *PatchFloat) UnmarshalJSON(raw []byte) error {
	p.Present = true
	if string(raw) == "null" {
		p.Value = nil
		return nil
	}
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	p.Value = &value
	return nil
}

type PatchStringSlice struct {
	Present bool
	Value   []string
}

func (p *PatchStringSlice) UnmarshalJSON(raw []byte) error {
	p.Present = true
	if string(raw) == "null" {
		p.Value = []string{}
		return nil
	}
	return json.Unmarshal(raw, &p.Value)
}

type UserSummary struct {
	UserPublicID     string `json:"user_public_id"`
	Nickname         string `json:"nickname"`
	OnboardingStatus string `json:"onboarding_status"`
}

type ProfileSummary struct {
	ProfilePublicID    string   `json:"profile_public_id"`
	Gender             string   `json:"gender"`
	HeightCM           *int     `json:"height_cm"`
	WeightKG           *float64 `json:"weight_kg"`
	BodyNotes          string   `json:"body_notes"`
	SkinNotes          string   `json:"skin_notes"`
	HairNotes          string   `json:"hair_notes"`
	FaceShape          string   `json:"face_shape"`
	UpperBodyNotes     string   `json:"upper_body_notes"`
	LowerBodyNotes     string   `json:"lower_body_notes"`
	SizeNotes          string   `json:"size_notes"`
	LifestyleScenarios []string `json:"lifestyle_scenarios"`
	StyleGoalSummary   string   `json:"style_goal_summary"`
}

type ProfilePhoto struct {
	PublicID      string             `json:"public_id"`
	AssetPublicID string             `json:"asset_public_id"`
	PhotoType     string             `json:"photo_type"`
	Angle         string             `json:"angle"`
	Note          string             `json:"note"`
	SortOrder     int                `json:"sort_order"`
	Status        string             `json:"status"`
	Image         *ProfilePhotoImage `json:"image,omitempty"`
}

type ProfilePhotoImage struct {
	ObjectKey string `json:"object_key,omitempty"`
	URL       string `json:"url,omitempty"`
}

type CreateProfilePhotoRequest struct {
	AssetPublicID string `json:"asset_public_id"`
	PhotoType     string `json:"photo_type"`
	Angle         string `json:"angle"`
	Note          string `json:"note"`
	SortOrder     int    `json:"sort_order"`
}

type CreateProfilePhotoInput struct {
	AssetPublicID string
	PhotoType     string
	Angle         string
	Note          string
	SortOrder     int
}

type UpdateProfilePhotoRequest struct {
	PhotoType PatchString `json:"photo_type"`
	Angle     PatchString `json:"angle"`
	Note      PatchString `json:"note"`
	SortOrder PatchInt    `json:"sort_order"`
	Status    PatchString `json:"status"`
}

type UpdateProfilePhotoInput struct {
	PhotoType PatchString
	Angle     PatchString
	Note      PatchString
	SortOrder PatchInt
	Status    PatchString
}

type MemorySummary struct {
	FactCount                int `json:"fact_count" db:"fact_count"`
	PreferenceCount          int `json:"preference_count" db:"preference_count"`
	AvoidanceCount           int `json:"avoidance_count" db:"avoidance_count"`
	InferenceCount           int `json:"inference_count" db:"inference_count"`
	PendingConfirmationCount int `json:"pending_confirmation_count" db:"pending_confirmation_count"`
}

type LatestReportSummary struct {
	PublicID    string     `json:"public_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	GeneratedAt *time.Time `json:"generated_at"`
}

type PreferencesSummary struct {
	StyleGoals          []string `json:"style_goals"`
	Avoidances          []string `json:"avoidances"`
	ScenarioPreferences []string `json:"scenario_preferences"`
}

type QuickEntry struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type Summary struct {
	User          UserSummary          `json:"user"`
	Profile       *ProfileSummary      `json:"profile"`
	ProfilePhotos []ProfilePhoto       `json:"profile_photos"`
	Preferences   PreferencesSummary   `json:"preferences"`
	MemorySummary MemorySummary        `json:"memory_summary"`
	LatestReport  *LatestReportSummary `json:"latest_report"`
	QuickEntries  []QuickEntry         `json:"quick_entries"`
}
