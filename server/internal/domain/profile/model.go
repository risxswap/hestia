package profile

import "time"

type Profile struct {
	ID                 int64
	PublicID           string
	UserID             int64
	Status             string
	Gender             string
	HeightCM           *int
	BodyNotes          string
	SkinNotes          string
	HairNotes          string
	LifestyleScenarios []string
	StyleGoalSummary   string
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

type QuickEntry struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type Summary struct {
	User          UserSummary          `json:"user"`
	Profile       *ProfileSummary      `json:"profile"`
	MemorySummary MemorySummary        `json:"memory_summary"`
	LatestReport  *LatestReportSummary `json:"latest_report"`
	QuickEntries  []QuickEntry         `json:"quick_entries"`
}
