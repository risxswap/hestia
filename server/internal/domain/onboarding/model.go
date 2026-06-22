package onboarding

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	DraftStatusDraft      = "draft"
	DraftStatusNotStarted = "not_started"
	DraftStatusSubmitted  = "submitted"
)

type DraftData map[string]json.RawMessage

func (d DraftData) Value() (driver.Value, error) {
	if len(d) == 0 {
		return []byte("{}"), nil
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (d *DraftData) Scan(value any) error {
	if d == nil {
		return fmt.Errorf("onboarding draft data receiver is nil")
	}
	if value == nil {
		*d = DraftData{}
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported onboarding draft data type %T", value)
	}
	if len(raw) == 0 || string(raw) == "null" {
		*d = DraftData{}
		return nil
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if data == nil {
		data = map[string]json.RawMessage{}
	}
	*d = DraftData(data)
	return nil
}

func (d DraftData) Clone() DraftData {
	if len(d) == 0 {
		return DraftData{}
	}
	copied := make(DraftData, len(d))
	for key, value := range d {
		copied[key] = append(json.RawMessage(nil), value...)
	}
	return copied
}

type Draft struct {
	ID          int64      `db:"id"`
	PublicID    string     `db:"public_id"`
	UserID      int64      `db:"user_id"`
	Status      string     `db:"status"`
	CurrentStep string     `db:"current_step"`
	DraftData   DraftData  `db:"draft_data"`
	ContentHash string     `db:"content_hash"`
	Version     int        `db:"version"`
	SubmittedAt *time.Time `db:"submitted_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

type SaveDraftRequest struct {
	Step string          `json:"step"`
	Data json.RawMessage `json:"data"`
}

type SaveDraftInput struct {
	Step string
	Data DraftData
}

type DraftResponse struct {
	Status      string    `json:"status"`
	CurrentStep string    `json:"current_step"`
	Version     int       `json:"version"`
	DraftData   DraftData `json:"draft_data"`
}

type SubmitResponse struct {
	JobPublicID    string `json:"job_public_id"`
	ReportPublicID string `json:"report_public_id"`
}
