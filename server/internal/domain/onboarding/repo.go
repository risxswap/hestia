package onboarding

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

var ErrDraftNotFound = errors.New("onboarding draft not found")

type DraftRepository interface {
	FindActiveByUserID(ctx context.Context, userID int64) (Draft, error)
	Create(ctx context.Context, draft Draft) (Draft, error)
	Update(ctx context.Context, draft Draft) (Draft, error)
}

type MySQLDraftRepository struct {
	db *sqlx.DB
}

func NewMySQLDraftRepository(db *sqlx.DB) *MySQLDraftRepository {
	return &MySQLDraftRepository{db: db}
}

func (r *MySQLDraftRepository) FindActiveByUserID(ctx context.Context, userID int64) (Draft, error) {
	if r == nil || r.db == nil {
		return Draft{}, errors.New("onboarding repository database is nil")
	}

	var draft Draft
	err := r.db.GetContext(ctx, &draft, `
SELECT
  id,
  public_id,
  user_id,
  status,
  COALESCE(current_step, '') AS current_step,
  COALESCE(draft_data, JSON_OBJECT()) AS draft_data,
  content_hash,
  version,
  submitted_at,
  created_at,
  updated_at
FROM onboarding_drafts
WHERE user_id = ?
  AND status = 'draft'
  AND deleted_at IS NULL
ORDER BY id DESC
LIMIT 1
`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, ErrDraftNotFound
	}
	return draft, err
}

func (r *MySQLDraftRepository) Create(ctx context.Context, draft Draft) (Draft, error) {
	if r == nil || r.db == nil {
		return Draft{}, errors.New("onboarding repository database is nil")
	}
	result, err := r.db.ExecContext(ctx, `
INSERT INTO onboarding_drafts
  (public_id, user_id, status, current_step, draft_data, content_hash, version)
VALUES
  (?, ?, ?, ?, ?, ?, ?)
`, draft.PublicID, draft.UserID, draft.Status, draft.CurrentStep, draft.DraftData, draft.ContentHash, draft.Version)
	if err != nil {
		return Draft{}, err
	}
	if id, err := result.LastInsertId(); err == nil {
		draft.ID = id
	}
	return draft, nil
}

func (r *MySQLDraftRepository) Update(ctx context.Context, draft Draft) (Draft, error) {
	if r == nil || r.db == nil {
		return Draft{}, errors.New("onboarding repository database is nil")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE onboarding_drafts
SET
  current_step = ?,
  draft_data = ?,
  content_hash = ?,
  version = ?
WHERE id = ?
  AND user_id = ?
  AND deleted_at IS NULL
`, draft.CurrentStep, draft.DraftData, draft.ContentHash, draft.Version, draft.ID, draft.UserID)
	if err != nil {
		return Draft{}, err
	}
	return draft, nil
}
