package hair

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"hestia/server/internal/common/dbutil"

	"github.com/jmoiron/sqlx"
)

const (
	hairAssetType         = "hair_photo"
	miniappUploadSource   = "miniapp_upload"
	localOnboardingBucket = "local-onboarding"
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

func (r *MySQLRepository) ListItems(ctx context.Context, userID int64, _ ListFilter) ([]Item, error) {
	if r == nil || r.ext == nil {
		return nil, errors.New("hair repository database is nil")
	}
	rows := []itemRow{}
	if err := sqlx.SelectContext(ctx, r.ext, &rows, itemSelectSQL(`
WHERE h.user_id = ?
  AND h.status <> ?
  AND h.deleted_at IS NULL
ORDER BY h.updated_at DESC, h.id DESC
`), userID, StatusDeleted); err != nil {
		return nil, err
	}
	return itemsFromRows(rows)
}

func (r *MySQLRepository) FindItemForUser(ctx context.Context, userID int64, publicID string) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("hair repository database is nil")
	}
	rows := []itemRow{}
	if err := sqlx.SelectContext(ctx, r.ext, &rows, itemSelectSQL(`
WHERE h.user_id = ?
  AND h.public_id = ?
  AND h.status <> ?
  AND h.deleted_at IS NULL
`), userID, publicID, StatusDeleted); err != nil {
		return Item{}, err
	}
	items, err := itemsFromRows(rows)
	if err != nil {
		return Item{}, err
	}
	if len(items) == 0 {
		return Item{}, ErrItemNotFound
	}
	return items[0], nil
}

func (r *MySQLRepository) CreateItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("hair repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Item{}, err
		}
		created, err := (&MySQLRepository{ext: tx}).createItem(ctx, item, primaryAssetPublicID)
		if err != nil {
			_ = tx.Rollback()
			return Item{}, err
		}
		if err := tx.Commit(); err != nil {
			return Item{}, err
		}
		return created, nil
	}
	return r.createItem(ctx, item, primaryAssetPublicID)
}

func (r *MySQLRepository) createItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO hair
  (public_id, user_id, name, length, shape, bangs, color, care_time, suitability_notes, avoidance_notes, user_notes, recommendation_status, status)
VALUES
  (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)
`, item.PublicID, item.UserID, item.Name, item.Length, item.Shape, item.Bangs, item.Color, item.CareTime, item.SuitabilityNotes, item.AvoidanceNotes, item.UserNotes, item.RecommendationStatus, item.Status)
	if err != nil {
		return Item{}, err
	}
	item.ID, err = dbutil.RequireLastInsertID(result, "hair create")
	if err != nil {
		return Item{}, err
	}
	if err := r.replaceSceneTags(ctx, item.ID, item.SceneTags); err != nil {
		return Item{}, err
	}
	if strings.TrimSpace(primaryAssetPublicID) != "" {
		image, err := r.setPrimaryAsset(ctx, item.UserID, item.ID, primaryAssetPublicID)
		if err != nil {
			return Item{}, err
		}
		item.PrimaryImage = &image
	}
	return item, nil
}

func (r *MySQLRepository) UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("hair repository database is nil")
	}
	current, err := r.FindItemForUser(ctx, userID, publicID)
	if err != nil {
		return Item{}, err
	}
	sets := []string{}
	args := []any{}
	addSet := func(column string, value any) {
		sets = append(sets, column+" = ?")
		args = append(args, value)
	}
	if input.Name != nil {
		addSet("name", *input.Name)
	}
	if input.Length != nil {
		addSet("length", nullIfEmpty(*input.Length))
	}
	if input.Shape != nil {
		addSet("shape", nullIfEmpty(*input.Shape))
	}
	if input.Bangs != nil {
		addSet("bangs", nullIfEmpty(*input.Bangs))
	}
	if input.Color != nil {
		addSet("color", nullIfEmpty(*input.Color))
	}
	if input.CareTime != nil {
		addSet("care_time", nullIfEmpty(*input.CareTime))
	}
	if input.SuitabilityNotes != nil {
		addSet("suitability_notes", nullIfEmpty(*input.SuitabilityNotes))
	}
	if input.AvoidanceNotes != nil {
		addSet("avoidance_notes", nullIfEmpty(*input.AvoidanceNotes))
	}
	if input.UserNotes != nil {
		addSet("user_notes", nullIfEmpty(*input.UserNotes))
	}
	if input.RecommendationStatus != nil {
		addSet("recommendation_status", *input.RecommendationStatus)
	}
	if len(sets) > 0 {
		args = append(args, current.ID, userID)
		result, err := r.ext.ExecContext(ctx, `
UPDATE hair
SET `+strings.Join(sets, ", ")+`
WHERE id = ?
  AND user_id = ?
  AND status <> 'deleted'
  AND deleted_at IS NULL
`, args...)
		if err != nil {
			return Item{}, err
		}
		if err := dbutil.RequireRowsAffected(result, "hair update"); err != nil {
			return Item{}, err
		}
	}
	if input.SceneTags != nil {
		if err := r.replaceSceneTags(ctx, current.ID, *input.SceneTags); err != nil {
			return Item{}, err
		}
	}
	if input.PrimaryAssetPublicID != nil {
		if strings.TrimSpace(*input.PrimaryAssetPublicID) == "" {
			_ = r.clearPrimaryAssets(ctx, current.ID)
		} else if _, err := r.setPrimaryAsset(ctx, userID, current.ID, *input.PrimaryAssetPublicID); err != nil {
			return Item{}, err
		}
	}
	return r.FindItemForUser(ctx, userID, publicID)
}

func (r *MySQLRepository) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	if r == nil || r.ext == nil {
		return errors.New("hair repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `
UPDATE hair
SET status = ?, deleted_at = CURRENT_TIMESTAMP(3)
WHERE user_id = ?
  AND public_id = ?
  AND status <> ?
  AND deleted_at IS NULL
`, StatusDeleted, userID, publicID, StatusDeleted)
	if err != nil {
		return err
	}
	return dbutil.RequireRowsAffected(result, "hair soft delete")
}

func (r *MySQLRepository) replaceSceneTags(ctx context.Context, hairID int64, tags []string) error {
	if _, err := r.ext.ExecContext(ctx, `DELETE FROM hair_scene_tags WHERE hair_id = ?`, hairID); err != nil {
		return err
	}
	for index, tag := range trimStrings(tags) {
		if _, err := r.ext.ExecContext(ctx, `
INSERT INTO hair_scene_tags (hair_id, tag, sort_order)
VALUES (?, ?, ?)
`, hairID, tag, index); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) setPrimaryAsset(ctx context.Context, userID int64, hairID int64, assetPublicID string) (Image, error) {
	asset, err := r.findAssetForUser(ctx, userID, assetPublicID)
	if err != nil {
		return Image{}, err
	}
	if err := r.clearPrimaryAssets(ctx, hairID); err != nil {
		return Image{}, err
	}
	if _, err := r.ext.ExecContext(ctx, `
INSERT INTO hair_assets (hair_id, asset_id, is_primary, sort_order)
VALUES (?, ?, 1, 0)
`, hairID, asset.ID); err != nil {
		return Image{}, err
	}
	return Image{AssetPublicID: asset.PublicID, ObjectKey: asset.ObjectKey}, nil
}

func (r *MySQLRepository) clearPrimaryAssets(ctx context.Context, hairID int64) error {
	_, err := r.ext.ExecContext(ctx, `UPDATE hair_assets SET is_primary = 0 WHERE hair_id = ?`, hairID)
	return err
}

func (r *MySQLRepository) findAssetForUser(ctx context.Context, userID int64, publicID string) (assetRow, error) {
	row := assetRow{}
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT id, public_id, bucket, object_key, asset_type, source
FROM files
WHERE public_id = ?
  AND owner_user_id = ?
  AND status <> 'deleted'
  AND deleted_at IS NULL
LIMIT 1
`, strings.TrimSpace(publicID), userID)
	if errors.Is(err, sql.ErrNoRows) {
		return assetRow{}, ErrInvalidPrimaryAsset
	}
	if err != nil {
		return assetRow{}, err
	}
	if !row.eligible(userID) {
		return assetRow{}, ErrInvalidPrimaryAsset
	}
	return row, nil
}

func itemSelectSQL(where string) string {
	return `
SELECT
  h.id, h.public_id, h.user_id, h.name, h.length, h.shape, h.bangs, h.color, h.care_time,
  h.suitability_notes, h.avoidance_notes, h.user_notes, h.recommendation_status, h.status,
  h.created_at, h.updated_at,
  a.public_id AS primary_asset_public_id,
  a.object_key AS primary_object_key,
  st.tag AS scene_tag,
  st.sort_order AS scene_tag_sort_order
FROM hair h
LEFT JOIN hair_assets ha ON ha.hair_id = h.id AND ha.is_primary = 1
LEFT JOIN files a ON a.id = ha.asset_id AND a.deleted_at IS NULL AND a.status <> 'deleted'
LEFT JOIN hair_scene_tags st ON st.hair_id = h.id
` + where
}

type txStarter interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

type itemRow struct {
	ID                   int64          `db:"id"`
	PublicID             string         `db:"public_id"`
	UserID               int64          `db:"user_id"`
	Name                 string         `db:"name"`
	Length               sql.NullString `db:"length"`
	Shape                sql.NullString `db:"shape"`
	Bangs                sql.NullString `db:"bangs"`
	Color                sql.NullString `db:"color"`
	CareTime             sql.NullString `db:"care_time"`
	SuitabilityNotes     sql.NullString `db:"suitability_notes"`
	AvoidanceNotes       sql.NullString `db:"avoidance_notes"`
	UserNotes            sql.NullString `db:"user_notes"`
	RecommendationStatus string         `db:"recommendation_status"`
	Status               string         `db:"status"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"`
	PrimaryAssetPublicID sql.NullString `db:"primary_asset_public_id"`
	PrimaryObjectKey     sql.NullString `db:"primary_object_key"`
	SceneTag             sql.NullString `db:"scene_tag"`
	SceneTagSortOrder    sql.NullInt64  `db:"scene_tag_sort_order"`
}

type assetRow struct {
	ID        int64  `db:"id"`
	PublicID  string `db:"public_id"`
	Bucket    string `db:"bucket"`
	ObjectKey string `db:"object_key"`
	AssetType string `db:"asset_type"`
	Source    string `db:"source"`
}

func itemsFromRows(rows []itemRow) ([]Item, error) {
	items := []Item{}
	indexByID := map[int64]int{}
	for _, row := range rows {
		index, ok := indexByID[row.ID]
		if !ok {
			item := Item{
				ID:                   row.ID,
				PublicID:             row.PublicID,
				UserID:               row.UserID,
				Name:                 row.Name,
				Length:               nullString(row.Length),
				Shape:                nullString(row.Shape),
				Bangs:                nullString(row.Bangs),
				Color:                nullString(row.Color),
				CareTime:             nullString(row.CareTime),
				SuitabilityNotes:     nullString(row.SuitabilityNotes),
				AvoidanceNotes:       nullString(row.AvoidanceNotes),
				UserNotes:            nullString(row.UserNotes),
				RecommendationStatus: row.RecommendationStatus,
				Status:               row.Status,
				CreatedAt:            row.CreatedAt,
				UpdatedAt:            row.UpdatedAt,
			}
			if row.PrimaryAssetPublicID.Valid {
				item.PrimaryImage = &Image{AssetPublicID: row.PrimaryAssetPublicID.String, ObjectKey: nullString(row.PrimaryObjectKey)}
			}
			indexByID[row.ID] = len(items)
			items = append(items, item)
			index = len(items) - 1
		}
		if row.SceneTag.Valid {
			items[index].SceneTags = append(items[index].SceneTags, row.SceneTag.String)
		}
	}
	return items, nil
}

func (r assetRow) eligible(userID int64) bool {
	if r.Bucket == localOnboardingBucket || r.AssetType != hairAssetType || r.Source != miniappUploadSource {
		return false
	}
	prefix := fmt.Sprintf("users/%d/hair/", userID)
	return strings.HasPrefix(r.ObjectKey, prefix)
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
