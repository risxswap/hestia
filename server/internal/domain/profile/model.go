package profile

import (
	"encoding/json"
	"time"
)

type Profile struct {
	ID                 int64  `db:"id"`
	PublicID           string `db:"public_id"`
	UserID             int64  `db:"user_id"`
	Status             string `db:"status"`
	Gender             string `db:"gender"`
	HeightCM           *int   `db:"height_cm"`
	BodyNotes          string `db:"body_notes"`
	SkinNotes          string `db:"skin_notes"`
	HairNotes          string `db:"hair_notes"`
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
	BodyNotes          PatchString      `json:"body_notes"`
	SkinNotes          PatchString      `json:"skin_notes"`
	HairNotes          PatchString      `json:"hair_notes"`
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
	BodyNotes          PatchString
	SkinNotes          PatchString
	HairNotes          PatchString
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
	BodyNotes          string   `json:"body_notes"`
	SkinNotes          string   `json:"skin_notes"`
	HairNotes          string   `json:"hair_notes"`
	LifestyleScenarios []string `json:"lifestyle_scenarios"`
	StyleGoalSummary   string   `json:"style_goal_summary"`
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
	Preferences   PreferencesSummary   `json:"preferences"`
	MemorySummary MemorySummary        `json:"memory_summary"`
	LatestReport  *LatestReportSummary `json:"latest_report"`
	QuickEntries  []QuickEntry         `json:"quick_entries"`
}
