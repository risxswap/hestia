package memory

import "time"

const (
	StatusActive  = "active"
	StatusDeleted = "deleted"

	TypeFact       = "fact"
	TypePreference = "preference"
	TypeAvoidance  = "avoidance"
	TypeInference  = "inference"
	TypePending    = "pending"

	PolarityPositive = "positive"
	PolarityNegative = "negative"
	PolarityNeutral  = "neutral"

	VisibilityVisible = "visible"
)

type Item struct {
	ID              int64      `json:"-"`
	PublicID        string     `json:"public_id"`
	UserID          int64      `json:"-"`
	MemoryType      string     `json:"memory_type"`
	TypeLabel       string     `json:"type_label"`
	MemoryKey       string     `json:"memory_key"`
	MemoryValue     string     `json:"memory_value"`
	DisplayText     string     `json:"display_text"`
	Polarity        string     `json:"polarity"`
	Confidence      *float64   `json:"confidence,omitempty"`
	Visibility      string     `json:"visibility"`
	Status          string     `json:"status"`
	SourceLabel     string     `json:"source_label"`
	CorrectionNote  string     `json:"correction_note,omitempty"`
	UserCorrectedAt *time.Time `json:"user_corrected_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at,omitempty"`
}

type UpdateRequest struct {
	MemoryValue    string `json:"memory_value"`
	CorrectionNote string `json:"correction_note"`
}

type UpdateInput struct {
	MemoryValue    string
	CorrectionNote string
}
