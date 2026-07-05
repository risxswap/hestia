package makeup

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
	makeupAssetType       = "makeup_photo"
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
		return nil, errors.New("makeup repository database is nil")
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
		return Item{}, errors.New("makeup repository database is nil")
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
		return Item{}, errors.New("makeup repository database is nil")
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
INSERT INTO makeup
  (public_id, user_id, name, makeup_type, focus, color_palette, finish, suitability_notes, avoidance_notes, user_notes, recommendation_status, status)
VALUES
  (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)
`, item.PublicID, item.UserID, item.Name, item.MakeupType, item.Focus, item.ColorPalette, item.Finish, item.SuitabilityNotes, item.AvoidanceNotes, item.UserNotes, item.RecommendationStatus, item.Status)
	if err != nil {
		return Item{}, err
	}
	item.ID, err = dbutil.RequireLastInsertID(result, "makeup create")
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
		return Item{}, errors.New("makeup repository database is nil")
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
	if input.MakeupType != nil {
		addSet("makeup_type", nullIfEmpty(*input.MakeupType))
	}
	if input.Focus != nil {
		addSet("focus", nullIfEmpty(*input.Focus))
	}
	if input.ColorPalette != nil {
		addSet("color_palette", nullIfEmpty(*input.ColorPalette))
	}
	if input.Finish != nil {
		addSet("finish", nullIfEmpty(*input.Finish))
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
UPDATE makeup
SET `+strings.Join(sets, ", ")+`
WHERE id = ?
  AND user_id = ?
  AND status <> 'deleted'
  AND deleted_at IS NULL
`, args...)
		if err != nil {
			return Item{}, err
		}
		if err := dbutil.RequireRowsAffected(result, "makeup update"); err != nil {
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
		return errors.New("makeup repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `
UPDATE makeup
SET status = ?, deleted_at = CURRENT_TIMESTAMP(3)
WHERE user_id = ?
  AND public_id = ?
  AND status <> ?
  AND deleted_at IS NULL
`, StatusDeleted, userID, publicID, StatusDeleted)
	if err != nil {
		return err
	}
	return dbutil.RequireRowsAffected(result, "makeup soft delete")
}

func (r *MySQLRepository) replaceSceneTags(ctx context.Context, makeupID int64, tags []string) error {
	if _, err := r.ext.ExecContext(ctx, `DELETE FROM makeup_scene_tags WHERE makeup_id = ?`, makeupID); err != nil {
		return err
	}
	for index, tag := range trimStrings(tags) {
		if _, err := r.ext.ExecContext(ctx, `
INSERT INTO makeup_scene_tags (makeup_id, tag, sort_order)
VALUES (?, ?, ?)
`, makeupID, tag, index); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) setPrimaryAsset(ctx context.Context, userID int64, makeupID int64, assetPublicID string) (Image, error) {
	asset, err := r.findAssetForUser(ctx, userID, assetPublicID)
	if err != nil {
		return Image{}, err
	}
	if err := r.clearPrimaryAssets(ctx, makeupID); err != nil {
		return Image{}, err
	}
	if _, err := r.ext.ExecContext(ctx, `
INSERT INTO makeup_assets (makeup_id, asset_id, is_primary, sort_order)
VALUES (?, ?, 1, 0)
`, makeupID, asset.ID); err != nil {
		return Image{}, err
	}
	return Image{AssetPublicID: asset.PublicID, ObjectKey: asset.ObjectKey}, nil
}

func (r *MySQLRepository) clearPrimaryAssets(ctx context.Context, makeupID int64) error {
	_, err := r.ext.ExecContext(ctx, `UPDATE makeup_assets SET is_primary = 0 WHERE makeup_id = ?`, makeupID)
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
  h.id, h.public_id, h.user_id, h.name, h.makeup_type, h.focus, h.color_palette, h.finish,
  h.suitability_notes, h.avoidance_notes, h.user_notes, h.recommendation_status, h.status,
  h.created_at, h.updated_at,
  a.public_id AS primary_asset_public_id,
  a.object_key AS primary_object_key,
  st.tag AS scene_tag,
  st.sort_order AS scene_tag_sort_order
FROM makeup h
LEFT JOIN makeup_assets ha ON ha.makeup_id = h.id AND ha.is_primary = 1
LEFT JOIN files a ON a.id = ha.asset_id AND a.deleted_at IS NULL AND a.status <> 'deleted'
LEFT JOIN makeup_scene_tags st ON st.makeup_id = h.id
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
	MakeupType           sql.NullString `db:"makeup_type"`
	Focus                sql.NullString `db:"focus"`
	ColorPalette         sql.NullString `db:"color_palette"`
	Finish               sql.NullString `db:"finish"`
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
				MakeupType:           nullString(row.MakeupType),
				Focus:                nullString(row.Focus),
				ColorPalette:         nullString(row.ColorPalette),
				Finish:               nullString(row.Finish),
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
	if r.Bucket == localOnboardingBucket || r.AssetType != makeupAssetType || r.Source != miniappUploadSource {
		return false
	}
	prefix := fmt.Sprintf("users/%d/makeup/", userID)
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
