package job

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type MySQLRepository struct {
	db *sqlx.DB
}

func NewMySQLRepository(db *sqlx.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Create(ctx context.Context, item Job) (Job, error) {
	if r == nil || r.db == nil {
		return Job{}, errors.New("job repository database is nil")
	}
	input, err := jsonText(item.InputSummary)
	if err != nil {
		return Job{}, err
	}
	result, err := r.db.ExecContext(ctx, `
INSERT INTO jobs
  (public_id, job_type, status, queue_name, related_type, related_id, user_id, input_summary, started_at)
VALUES
  (?, ?, ?, ?, NULLIF(?, ''), ?, ?, CAST(? AS JSON), ?)
`, item.PublicID, item.JobType, item.Status, item.QueueName, item.RelatedType, item.RelatedID, item.UserID, input, item.StartedAt)
	if err != nil {
		return Job{}, err
	}
	if id, err := result.LastInsertId(); err == nil {
		item.ID = id
	}
	return item, nil
}

func (r *MySQLRepository) UpdateStatus(ctx context.Context, id int64, status string, output map[string]any, errorMessage string) error {
	if r == nil || r.db == nil {
		return errors.New("job repository database is nil")
	}
	outputJSON, err := jsonText(output)
	if err != nil {
		return err
	}
	var finishedAt any
	if status == StatusSucceeded || status == StatusFailed {
		now := time.Now().UTC()
		finishedAt = now
	}
	_, err = r.db.ExecContext(ctx, `
UPDATE jobs
SET status = ?, output_summary = CAST(? AS JSON), error_message = NULLIF(?, ''), finished_at = ?
WHERE id = ?
`, status, outputJSON, errorMessage, finishedAt, id)
	return err
}

func (r *MySQLRepository) FindByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Job, error) {
	if r == nil || r.db == nil {
		return Job{}, errors.New("job repository database is nil")
	}
	var row jobRow
	err := r.db.GetContext(ctx, &row, `
SELECT
  id,
  public_id,
  job_type,
  status,
  queue_name,
  COALESCE(related_type, '') AS related_type,
  related_id,
  COALESCE(user_id, 0) AS user_id,
  COALESCE(input_summary, JSON_OBJECT()) AS input_summary,
  COALESCE(output_summary, JSON_OBJECT()) AS output_summary,
  COALESCE(error_message, '') AS error_message,
  started_at,
  finished_at
FROM jobs
WHERE public_id = ? AND user_id = ?
LIMIT 1
`, publicID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, err
	}
	return row.toJob()
}

type jobRow struct {
	ID            int64           `db:"id"`
	PublicID      string          `db:"public_id"`
	JobType       string          `db:"job_type"`
	Status        string          `db:"status"`
	QueueName     string          `db:"queue_name"`
	RelatedType   string          `db:"related_type"`
	RelatedID     *int64          `db:"related_id"`
	UserID        int64           `db:"user_id"`
	InputSummary  json.RawMessage `db:"input_summary"`
	OutputSummary json.RawMessage `db:"output_summary"`
	ErrorMessage  string          `db:"error_message"`
	StartedAt     *time.Time      `db:"started_at"`
	FinishedAt    *time.Time      `db:"finished_at"`
}

func (r jobRow) toJob() (Job, error) {
	input, err := decodeJSONMap(r.InputSummary)
	if err != nil {
		return Job{}, err
	}
	output, err := decodeJSONMap(r.OutputSummary)
	if err != nil {
		return Job{}, err
	}
	return Job{
		ID:            r.ID,
		PublicID:      r.PublicID,
		JobType:       r.JobType,
		Status:        r.Status,
		QueueName:     r.QueueName,
		RelatedType:   r.RelatedType,
		RelatedID:     r.RelatedID,
		UserID:        r.UserID,
		InputSummary:  input,
		OutputSummary: output,
		ErrorMessage:  r.ErrorMessage,
		StartedAt:     r.StartedAt,
		FinishedAt:    r.FinishedAt,
	}, nil
}

func jsonText(value any) (string, error) {
	if value == nil {
		return "null", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeJSONMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]any{}
	}
	return data, nil
}
