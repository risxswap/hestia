package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"hestia/server/internal/common/dbutil"

	"github.com/jmoiron/sqlx"
)

const (
	tableFact      = "fact"
	tablePref      = "pref"
	tableInference = "inference"
	tableMemory    = "memory"
)

type MySQLRepository struct {
	ext sqlx.ExtContext
}

func NewMySQLRepository(db *sqlx.DB) *MySQLRepository {
	return NewMySQLRepositoryWithExt(db)
}

func NewMySQLRepositoryWithExt(ext sqlx.ExtContext) *MySQLRepository {
	return &MySQLRepository{ext: ext}
}

func (r *MySQLRepository) ListItems(ctx context.Context, userID int64) ([]Item, error) {
	if r == nil || r.ext == nil {
		return nil, ErrRepositoryUnsupported
	}
	var rows []memoryRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, `
SELECT
  CONCAT('fact_', id) AS public_id,
  id,
  user_id,
  'fact' AS origin,
  'fact' AS memory_type,
  fact_key AS memory_key,
  fact_value AS memory_value,
  'neutral' AS polarity,
  NULL AS confidence,
  'visible' AS visibility,
  'active' AS status,
  source AS source_label,
  NULL AS correction_note,
  confirmed_at AS user_corrected_at,
  created_at,
  updated_at
FROM profile_facts
WHERE user_id = ?
  AND deleted_at IS NULL
UNION ALL
SELECT
  CONCAT('pref_', id) AS public_id,
  id,
  user_id,
  'pref' AS origin,
  IF(polarity = 'negative', 'avoidance', 'preference') AS memory_type,
  pref_key AS memory_key,
  pref_value AS memory_value,
  polarity,
  confidence,
  'visible' AS visibility,
  'active' AS status,
  source AS source_label,
  NULL AS correction_note,
  NULL AS user_corrected_at,
  created_at,
  updated_at
FROM profile_prefs
WHERE user_id = ?
  AND deleted_at IS NULL
UNION ALL
SELECT
  CONCAT('inference_', id) AS public_id,
  id,
  user_id,
  'inference' AS origin,
  IF(confirmed_by_user = 0, 'pending', 'inference') AS memory_type,
  inference_key AS memory_key,
  inference_value AS memory_value,
  'neutral' AS polarity,
  confidence,
  'visible' AS visibility,
  status,
  '系统推断' AS source_label,
  NULL AS correction_note,
  NULL AS user_corrected_at,
  created_at,
  updated_at
FROM profile_inferences
WHERE user_id = ?
  AND status = 'active'
  AND deleted_at IS NULL
UNION ALL
SELECT
  public_id,
  id,
  user_id,
  'memory' AS origin,
  memory_type,
  memory_key,
  memory_value,
  polarity,
  confidence,
  visibility,
  status,
  '' AS source_label,
  correction_note,
  user_corrected_at,
  created_at,
  updated_at
FROM memories
WHERE user_id = ?
  AND visibility = 'visible'
  AND status = 'active'
  AND deleted_at IS NULL
ORDER BY updated_at DESC, id DESC
`, userID, userID, userID, userID); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		item, err := row.toItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *MySQLRepository) UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, ErrRepositoryUnsupported
	}
	ref, err := parsePublicID(publicID)
	if err != nil {
		return Item{}, err
	}
	valueJSON, err := json.Marshal(input.MemoryValue)
	if err != nil {
		return Item{}, err
	}
	now := time.Now().UTC()
	switch ref.origin {
	case tableFact:
		if err := r.updateFact(ctx, userID, ref.id, string(valueJSON), now); err != nil {
			return Item{}, err
		}
	case tablePref:
		if err := r.updatePref(ctx, userID, ref.id, string(valueJSON), input.MemoryValue, now); err != nil {
			return Item{}, err
		}
	case tableInference:
		if err := r.updateInference(ctx, userID, ref.id, string(valueJSON), now); err != nil {
			return Item{}, err
		}
	case tableMemory:
		if err := r.updateMemory(ctx, userID, strings.TrimSpace(publicID), string(valueJSON), input.CorrectionNote, now); err != nil {
			return Item{}, err
		}
	default:
		return Item{}, ErrItemNotFound
	}
	item, err := r.findItemForUser(ctx, userID, publicID)
	if err != nil {
		return Item{}, err
	}
	if input.CorrectionNote != "" {
		item.CorrectionNote = input.CorrectionNote
	}
	return item, nil
}

func (r *MySQLRepository) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	if r == nil || r.ext == nil {
		return ErrRepositoryUnsupported
	}
	ref, err := parsePublicID(publicID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	switch ref.origin {
	case tableFact:
		return requireAffected(r.ext.ExecContext(ctx, `UPDATE profile_facts SET deleted_at = ? WHERE user_id = ? AND id = ? AND deleted_at IS NULL`, now, userID, ref.id))
	case tablePref:
		return requireAffected(r.ext.ExecContext(ctx, `UPDATE profile_prefs SET deleted_at = ? WHERE user_id = ? AND id = ? AND deleted_at IS NULL`, now, userID, ref.id))
	case tableInference:
		return requireAffected(r.ext.ExecContext(ctx, `UPDATE profile_inferences SET status = 'deleted', deleted_at = ?, updated_at = ? WHERE user_id = ? AND id = ? AND status <> 'deleted' AND deleted_at IS NULL`, now, now, userID, ref.id))
	case tableMemory:
		return requireAffected(r.ext.ExecContext(ctx, `UPDATE memories SET status = 'deleted', deleted_at = ?, updated_at = ? WHERE user_id = ? AND public_id = ? AND status <> 'deleted' AND deleted_at IS NULL`, now, now, userID, strings.TrimSpace(publicID)))
	default:
		return ErrItemNotFound
	}
}

func (r *MySQLRepository) updateFact(ctx context.Context, userID int64, id int64, valueJSON string, now time.Time) error {
	return requireAffected(r.ext.ExecContext(ctx, `
UPDATE profile_facts
SET fact_value = CAST(? AS JSON),
    source = 'user',
    confirmed_at = ?,
    updated_at = ?
WHERE user_id = ?
  AND id = ?
  AND deleted_at IS NULL
`, valueJSON, now, now, userID, id))
}

func (r *MySQLRepository) updatePref(ctx context.Context, userID int64, id int64, valueJSON string, prefKey string, now time.Time) error {
	return requireAffected(r.ext.ExecContext(ctx, `
UPDATE profile_prefs
SET pref_value = CAST(? AS JSON),
    pref_key = ?,
    source = 'user',
    updated_at = ?
WHERE user_id = ?
  AND id = ?
  AND deleted_at IS NULL
`, valueJSON, prefKey, now, userID, id))
}

func (r *MySQLRepository) updateInference(ctx context.Context, userID int64, id int64, valueJSON string, now time.Time) error {
	return requireAffected(r.ext.ExecContext(ctx, `
UPDATE profile_inferences
SET inference_value = CAST(? AS JSON),
    confirmed_by_user = 1,
    updated_at = ?
WHERE user_id = ?
  AND id = ?
  AND status = 'active'
  AND deleted_at IS NULL
`, valueJSON, now, userID, id))
}

func (r *MySQLRepository) updateMemory(ctx context.Context, userID int64, publicID string, valueJSON string, correctionNote string, now time.Time) error {
	return requireAffected(r.ext.ExecContext(ctx, `
UPDATE memories
SET memory_value = CAST(? AS JSON),
    correction_note = NULLIF(?, ''),
    user_corrected_at = ?,
    updated_at = ?
WHERE user_id = ?
  AND public_id = ?
  AND status = 'active'
  AND deleted_at IS NULL
`, valueJSON, correctionNote, now, now, userID, publicID))
}

func requireAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	if err := dbutil.RequireRowsAffected(result, "memory write"); err != nil {
		return ErrItemNotFound
	}
	return nil
}

func (r *MySQLRepository) findItemForUser(ctx context.Context, userID int64, publicID string) (Item, error) {
	items, err := r.ListItems(ctx, userID)
	if err != nil {
		return Item{}, err
	}
	for _, item := range items {
		if item.PublicID == strings.TrimSpace(publicID) {
			return item, nil
		}
	}
	return Item{}, ErrItemNotFound
}

type memoryRef struct {
	origin string
	id     int64
}

func parsePublicID(publicID string) (memoryRef, error) {
	value := strings.TrimSpace(publicID)
	for _, prefix := range []string{tableFact, tablePref, tableInference} {
		fullPrefix := prefix + "_"
		if strings.HasPrefix(value, fullPrefix) {
			id, err := strconv.ParseInt(strings.TrimPrefix(value, fullPrefix), 10, 64)
			if err != nil || id <= 0 {
				return memoryRef{}, ErrItemNotFound
			}
			return memoryRef{origin: prefix, id: id}, nil
		}
	}
	if value != "" {
		return memoryRef{origin: tableMemory}, nil
	}
	return memoryRef{}, ErrItemNotFound
}

type memoryRow struct {
	PublicID        string          `db:"public_id"`
	ID              int64           `db:"id"`
	UserID          int64           `db:"user_id"`
	Origin          string          `db:"origin"`
	MemoryType      string          `db:"memory_type"`
	MemoryKey       string          `db:"memory_key"`
	MemoryValue     json.RawMessage `db:"memory_value"`
	Polarity        string          `db:"polarity"`
	Confidence      sql.NullFloat64 `db:"confidence"`
	Visibility      string          `db:"visibility"`
	Status          string          `db:"status"`
	SourceLabel     sql.NullString  `db:"source_label"`
	CorrectionNote  sql.NullString  `db:"correction_note"`
	UserCorrectedAt sql.NullTime    `db:"user_corrected_at"`
	CreatedAt       time.Time       `db:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at"`
}

func (r memoryRow) toItem() (Item, error) {
	value, err := decodeMemoryValue(r.MemoryValue)
	if err != nil {
		return Item{}, fmt.Errorf("decode memory %s: %w", r.PublicID, err)
	}
	item := Item{
		ID:          r.ID,
		PublicID:    r.PublicID,
		UserID:      r.UserID,
		MemoryType:  r.MemoryType,
		MemoryKey:   r.MemoryKey,
		MemoryValue: value,
		Polarity:    r.Polarity,
		Visibility:  r.Visibility,
		Status:      r.Status,
		DisplayText: value,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	if r.Confidence.Valid {
		item.Confidence = &r.Confidence.Float64
	}
	if r.UserCorrectedAt.Valid && r.Origin == tableMemory {
		value := r.UserCorrectedAt.Time
		item.UserCorrectedAt = &value
	}
	if r.CorrectionNote.Valid {
		item.CorrectionNote = r.CorrectionNote.String
	}
	if r.SourceLabel.Valid {
		item.SourceLabel = sourceLabelText(r.SourceLabel.String)
	}
	return item, nil
}

func decodeMemoryValue(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	switch typed := value.(type) {
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, strings.TrimSpace(fmt.Sprint(item)))
		}
		return strings.Join(nonEmpty(parts), "、"), nil
	default:
		return strings.TrimSpace(fmt.Sprint(typed)), nil
	}
}

func sourceLabelText(source string) string {
	switch strings.TrimSpace(source) {
	case "user":
		return "用户确认"
	case "onboarding":
		return "onboarding"
	case "":
		return ""
	default:
		return source
	}
}

func nonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
