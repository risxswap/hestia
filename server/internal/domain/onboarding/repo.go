package onboarding

import (
	"context"
	"database/sql"
	"errors"

	"hestia/server/internal/common/dbutil"

	"github.com/jmoiron/sqlx"
)

var ErrDraftNotFound = errors.New("onboarding draft not found")

type DraftRepository interface {
	FindActiveByUserID(ctx context.Context, userID int64) (Draft, error)
	Create(ctx context.Context, draft Draft) (Draft, error)
	Update(ctx context.Context, draft Draft) (Draft, error)
	ClaimDraft(ctx context.Context, draftID int64, userID int64, contentHash string) error
	MarkSubmitted(ctx context.Context, draftID int64, userID int64) error
}

type MySQLDraftRepository struct {
	ext sqlx.ExtContext
}

func NewMySQLDraftRepository(db *sqlx.DB) *MySQLDraftRepository {
	return NewMySQLDraftRepositoryWithExt(db)
}

func NewMySQLDraftRepositoryWithExt(ext sqlx.ExtContext) *MySQLDraftRepository {
	return &MySQLDraftRepository{ext: ext}
}

func (r *MySQLDraftRepository) FindActiveByUserID(ctx context.Context, userID int64) (Draft, error) {
	if r == nil || r.ext == nil {
		return Draft{}, errors.New("onboarding repository database is nil")
	}

	var draft Draft
	err := sqlx.GetContext(ctx, r.ext, &draft, `
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
	if r == nil || r.ext == nil {
		return Draft{}, errors.New("onboarding repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO onboarding_drafts
  (public_id, user_id, status, current_step, draft_data, content_hash, version)
VALUES
  (?, ?, ?, ?, ?, ?, ?)
`, draft.PublicID, draft.UserID, draft.Status, draft.CurrentStep, draft.DraftData, draft.ContentHash, draft.Version)
	if err != nil {
		return Draft{}, err
	}
	id, err := dbutil.RequireLastInsertID(result, "onboarding draft create")
	if err != nil {
		return Draft{}, err
	}
	draft.ID = id
	return draft, nil
}

func (r *MySQLDraftRepository) Update(ctx context.Context, draft Draft) (Draft, error) {
	if r == nil || r.ext == nil {
		return Draft{}, errors.New("onboarding repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `
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
	if err := rowsAffectedError(result); err != nil {
		return Draft{}, err
	}
	return draft, nil
}

func (r *MySQLDraftRepository) MarkSubmitted(ctx context.Context, draftID int64, userID int64) error {
	if r == nil || r.ext == nil {
		return errors.New("onboarding repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `
UPDATE onboarding_drafts
SET status = 'submitted', submitted_at = CURRENT_TIMESTAMP(3)
WHERE id = ?
  AND user_id = ?
  AND status = 'draft'
  AND deleted_at IS NULL
`, draftID, userID)
	if err != nil {
		return err
	}
	return rowsAffectedError(result)
}

func (r *MySQLDraftRepository) ClaimDraft(ctx context.Context, draftID int64, userID int64, contentHash string) error {
	if r == nil || r.ext == nil {
		return errors.New("onboarding repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `
UPDATE onboarding_drafts
SET status = 'submitted', submitted_at = CURRENT_TIMESTAMP(3)
WHERE id = ?
  AND user_id = ?
  AND content_hash = ?
  AND status = 'draft'
  AND deleted_at IS NULL
`, draftID, userID, contentHash)
	if err != nil {
		return err
	}
	return rowsAffectedError(result)
}

func rowsAffectedError(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDraftNotFound
	}
	return nil
}
