package clothes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"hestia/server/internal/common/dbutil"

	"github.com/jmoiron/sqlx"
)

const (
	clothPrimaryAssetType   = "clothes_item_photo"
	clothPrimaryAssetSource = "miniapp_upload"
	localOnboardingBucket   = "local-onboarding"
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

func (r *MySQLRepository) CreateCoreItems(ctx context.Context, items []Item) ([]Item, error) {
	if r == nil || r.ext == nil {
		return nil, errors.New("clothes repository database is nil")
	}
	for i := range items {
		if items[i].RecommendationStatus == "" {
			items[i].RecommendationStatus = RecommendationStatusNormal
		}
		result, err := r.ext.ExecContext(ctx, `
INSERT INTO clothes
  (public_id, user_id, name, category, color, silhouette, material, season, user_notes, is_core, recommendation_status, recognition_status, status)
VALUES
  (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)
`, items[i].PublicID, items[i].UserID, items[i].Name, items[i].Category, items[i].Color, items[i].Silhouette, items[i].Material, items[i].Season, items[i].UserNotes, items[i].IsCore, items[i].RecommendationStatus, recognitionStatusOrDefault(items[i].RecognitionStatus), items[i].Status)
		if err != nil {
			return nil, err
		}
		id, err := dbutil.RequireLastInsertID(result, "clothes item create")
		if err != nil {
			return nil, err
		}
		items[i].ID = id
	}
	return items, nil
}

func (r *MySQLRepository) ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error) {
	if r == nil || r.ext == nil {
		return nil, errors.New("clothes repository database is nil")
	}
	query := clothItemSelectSQL(`
WHERE wi.user_id = ?
  AND wi.status <> ?
  AND wi.deleted_at IS NULL`)
	args := []any{userID, StatusDeleted}
	if filter.Category != "" {
		query += `
  AND wi.category = ?`
		args = append(args, filter.Category)
	}
	if filter.RecommendationStatus != "" {
		query += `
  AND wi.recommendation_status = ?`
		args = append(args, filter.RecommendationStatus)
	}
	if filter.IsCore != nil {
		query += `
  AND wi.is_core = ?`
		args = append(args, *filter.IsCore)
	}
	query += `
ORDER BY wi.is_core DESC, wi.updated_at DESC, wi.id DESC`

	var rows []clothItemRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, query, args...); err != nil {
		return nil, err
	}
	return clothItemsFromRows(rows)
}

func (r *MySQLRepository) ListClothesOptions(ctx context.Context) (ClothesOptions, error) {
	if r == nil || r.ext == nil {
		return ClothesOptions{}, errors.New("clothes repository database is nil")
	}
	var rows []systemConfigOptionRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, `
SELECT `+"`key`, `value`"+`
FROM system_configs
WHERE `+"`group`"+` = ?
  AND status = ?
  AND `+"`key`"+` IN ('categories', 'colors', 'materials', 'seasons', 'silhouettes')
`, "clothes.item_options", StatusActive); err != nil {
		return ClothesOptions{}, err
	}

	options := ClothesOptions{}
	for _, row := range rows {
		items, err := parseOptionItems(row.Value)
		if err != nil {
			return ClothesOptions{}, err
		}
		switch row.Key {
		case "categories":
			options.Categories = items
		case "colors":
			options.Colors = items
		case "materials":
			options.Materials = items
		case "seasons":
			options.Seasons = items
		case "silhouettes":
			options.Silhouettes = items
		}
	}
	return mergeWithDefaultClothesOptions(options), nil
}

func (r *MySQLRepository) CreateItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("clothes repository database is nil")
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

func (r *MySQLRepository) CreateItemWithAssets(ctx context.Context, item Item, assetPublicIDs []string) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("clothes repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Item{}, err
		}
		created, err := (&MySQLRepository{ext: tx}).createItemWithAssets(ctx, item, assetPublicIDs)
		if err != nil {
			_ = tx.Rollback()
			return Item{}, err
		}
		if err := tx.Commit(); err != nil {
			return Item{}, err
		}
		return created, nil
	}
	return r.createItemWithAssets(ctx, item, assetPublicIDs)
}

func (r *MySQLRepository) ReplaceItemAssets(ctx context.Context, userID int64, publicID string, assetPublicIDs []string) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("clothes repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Item{}, err
		}
		updated, err := (&MySQLRepository{ext: tx}).replaceItemAssets(ctx, userID, publicID, assetPublicIDs)
		if err != nil {
			_ = tx.Rollback()
			return Item{}, err
		}
		if err := tx.Commit(); err != nil {
			return Item{}, err
		}
		return updated, nil
	}
	return r.replaceItemAssets(ctx, userID, publicID, assetPublicIDs)
}

func (r *MySQLRepository) createItemWithAssets(ctx context.Context, item Item, assetPublicIDs []string) (Item, error) {
	created, err := r.createItem(ctx, item, "")
	if err != nil {
		return Item{}, err
	}
	for index, assetPublicID := range trimStringSlice(assetPublicIDs) {
		image, err := r.addItemAsset(ctx, created.UserID, created.ID, assetPublicID, index == 0, index)
		if err != nil {
			return Item{}, err
		}
		if index == 0 {
			created.PrimaryImage = &image
		}
	}
	return created, nil
}

func (r *MySQLRepository) replaceItemAssets(ctx context.Context, userID int64, publicID string, assetPublicIDs []string) (Item, error) {
	current, err := r.findItemByPublicIDForUser(ctx, userID, publicID)
	if err != nil {
		return Item{}, err
	}
	if err := r.deleteItemAssets(ctx, current.ID); err != nil {
		return Item{}, err
	}
	for index, assetPublicID := range trimStringSlice(assetPublicIDs) {
		if _, err := r.addItemAsset(ctx, userID, current.ID, assetPublicID, index == 0, index); err != nil {
			return Item{}, err
		}
	}
	return r.findItemByPublicIDForUser(ctx, userID, publicID)
}

func (r *MySQLRepository) createItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	sceneTags, err := jsonText(item.SceneTags)
	if err != nil {
		return Item{}, err
	}
	if item.Status == "" {
		item.Status = StatusActive
	}
	if item.RecommendationStatus == "" {
		item.RecommendationStatus = RecommendationStatusNormal
	}
	if item.RecognitionStatus == "" {
		item.RecognitionStatus = RecognitionStatusSucceeded
	}
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO clothes
  (public_id, user_id, name, category, color, silhouette, material, season, scene_tags, user_notes, is_core, recommendation_status, recognition_status, status)
VALUES
  (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON), NULLIF(?, ''), ?, ?, ?, ?)
`, item.PublicID, item.UserID, item.Name, item.Category, item.Color, item.Silhouette, item.Material, item.Season, sceneTags, item.UserNotes, item.IsCore, item.RecommendationStatus, item.RecognitionStatus, item.Status)
	if err != nil {
		return Item{}, err
	}
	id, err := dbutil.RequireLastInsertID(result, "clothes item create")
	if err != nil {
		return Item{}, err
	}
	item.ID = id
	if primaryAssetPublicID != "" {
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
		return Item{}, errors.New("clothes repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Item{}, err
		}
		updated, err := (&MySQLRepository{ext: tx}).updateItem(ctx, userID, publicID, input)
		if err != nil {
			_ = tx.Rollback()
			return Item{}, err
		}
		if err := tx.Commit(); err != nil {
			return Item{}, err
		}
		return updated, nil
	}
	return r.updateItem(ctx, userID, publicID, input)
}

func (r *MySQLRepository) FindItemForUser(ctx context.Context, userID int64, publicID string) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("clothes repository database is nil")
	}
	return r.findItemByPublicIDForUser(ctx, userID, strings.TrimSpace(publicID))
}

func (r *MySQLRepository) updateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	current, err := r.findItemByPublicIDForUser(ctx, userID, publicID)
	if err != nil {
		return Item{}, err
	}
	sets := make([]string, 0, 11)
	args := make([]any, 0, 13)
	addSet := func(column string, value any) {
		sets = append(sets, column+" = ?")
		args = append(args, value)
	}
	if input.Name != nil {
		addSet("name", *input.Name)
	}
	if input.Category != nil {
		addSet("category", *input.Category)
	}
	if input.Color != nil {
		addSet("color", nullIfEmpty(*input.Color))
	}
	if input.Silhouette != nil {
		addSet("silhouette", nullIfEmpty(*input.Silhouette))
	}
	if input.Material != nil {
		addSet("material", nullIfEmpty(*input.Material))
	}
	if input.Season != nil {
		addSet("season", nullIfEmpty(*input.Season))
	}
	if input.SceneTags != nil {
		sceneTags, err := jsonText(*input.SceneTags)
		if err != nil {
			return Item{}, err
		}
		sets = append(sets, "scene_tags = CAST(? AS JSON)")
		args = append(args, sceneTags)
	}
	if input.UserNotes != nil {
		addSet("user_notes", nullIfEmpty(*input.UserNotes))
	}
	if input.IsCore != nil {
		addSet("is_core", *input.IsCore)
	}
	if input.RecommendationStatus != nil {
		addSet("recommendation_status", *input.RecommendationStatus)
	}
	if input.RecognitionStatus != nil {
		addSet("recognition_status", *input.RecognitionStatus)
	}
	if len(sets) > 0 {
		args = append(args, current.ID, userID)
		result, err := r.ext.ExecContext(ctx, `
UPDATE clothes
SET `+strings.Join(sets, ", ")+`
WHERE id = ?
  AND user_id = ?
  AND status <> 'deleted'
  AND deleted_at IS NULL
`, args...)
		if err != nil {
			return Item{}, err
		}
		if err := allowZeroRowsAffected(result, "clothes item update"); err != nil {
			return Item{}, err
		}
	}
	if input.PrimaryAssetPublicID != nil {
		primaryAssetPublicID := strings.TrimSpace(*input.PrimaryAssetPublicID)
		if primaryAssetPublicID == "" {
			if err := r.clearPrimaryAssets(ctx, current.ID); err != nil {
				return Item{}, err
			}
		} else if _, err := r.setPrimaryAsset(ctx, userID, current.ID, primaryAssetPublicID); err != nil {
			return Item{}, err
		}
	}
	return r.findItemByPublicIDForUser(ctx, userID, publicID)
}

func allowZeroRowsAffected(result sql.Result, operation string) error {
	if _, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	return nil
}

func (r *MySQLRepository) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	if r == nil || r.ext == nil {
		return errors.New("clothes repository database is nil")
	}
	var row struct {
		ID        int64        `db:"id"`
		Status    string       `db:"status"`
		DeletedAt sql.NullTime `db:"deleted_at"`
	}
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT id, status, deleted_at
FROM clothes
WHERE user_id = ?
  AND public_id = ?
LIMIT 1
`, userID, publicID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrItemNotFound
	}
	if err != nil {
		return err
	}
	if row.Status == StatusDeleted || row.DeletedAt.Valid {
		return nil
	}
	result, err := r.ext.ExecContext(ctx, `
UPDATE clothes
SET status = ?, deleted_at = CURRENT_TIMESTAMP(3)
WHERE id = ?
  AND user_id = ?
`, StatusDeleted, row.ID, userID)
	if err != nil {
		return err
	}
	return dbutil.RequireRowsAffected(result, "clothes item soft delete")
}

func (r *MySQLRepository) FindRecognizableAsset(ctx context.Context, userID int64, assetPublicID string) (Image, error) {
	if r == nil || r.ext == nil {
		return Image{}, errors.New("clothes repository database is nil")
	}
	asset, err := r.findAssetForUser(ctx, userID, strings.TrimSpace(assetPublicID))
	if err != nil {
		return Image{}, err
	}
	return Image{
		AssetPublicID: asset.PublicID,
		ObjectKey:     asset.ObjectKey,
	}, nil
}

func (r *MySQLRepository) findItemByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Item, error) {
	var row clothItemRow
	err := sqlx.GetContext(ctx, r.ext, &row, clothItemDetailSelectSQL(`
WHERE wi.user_id = ?
  AND wi.public_id = ?
  AND wi.status <> ?
  AND wi.deleted_at IS NULL
LIMIT 1
`), userID, publicID, StatusDeleted)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrItemNotFound
	}
	if err != nil {
		return Item{}, err
	}
	item, err := row.toItem()
	if err != nil {
		return Item{}, err
	}
	images, err := r.findItemImages(ctx, item.UserID, item.ID)
	if err != nil {
		return Item{}, err
	}
	if len(images) > 0 {
		item.Images = images
		item.PrimaryImage = &images[0]
	} else if item.PrimaryImage != nil {
		item.Images = []Image{*item.PrimaryImage}
	}
	return item, nil
}

func (r *MySQLRepository) findItemImages(ctx context.Context, userID int64, clothItemID int64) ([]Image, error) {
	var relationRows []clothItemAssetRelationRow
	if err := sqlx.SelectContext(ctx, r.ext, &relationRows, `
SELECT
  wia.id AS asset_relation_id,
  wia.asset_id,
  wia.sort_order AS asset_sort_order,
  wia.is_primary AS asset_is_primary
FROM clothes_assets wia
WHERE wia.clothes_id = ?
ORDER BY wia.sort_order ASC, wia.is_primary DESC, wia.id ASC
`, clothItemID); err != nil {
		return nil, err
	}
	if len(relationRows) == 0 {
		return nil, nil
	}
	assetIDs := make([]int64, 0, len(relationRows))
	for _, row := range relationRows {
		if row.AssetID.Valid {
			assetIDs = append(assetIDs, row.AssetID.Int64)
		}
	}
	if len(assetIDs) == 0 {
		return nil, nil
	}
	return r.findImagesByAssetIDs(ctx, userID, assetIDs)
}

func (r *MySQLRepository) findImagesByAssetIDs(ctx context.Context, userID int64, assetIDs []int64) ([]Image, error) {
	query, args, err := sqlx.In(`
SELECT
  id,
  public_id AS asset_public_id,
  object_key AS asset_object_key
FROM files
WHERE id IN (?)
  AND owner_user_id = ?
  AND deleted_at IS NULL
  AND status <> 'deleted'
  AND asset_type = ?
  AND source = ?
  AND bucket <> ?
  AND object_key LIKE CONCAT('users/', owner_user_id, '/clothes/', public_id, '.%')
ORDER BY FIELD(id, ?)
`, assetIDs, userID, clothPrimaryAssetType, clothPrimaryAssetSource, localOnboardingBucket, assetIDs)
	if err != nil {
		return nil, err
	}
	query = sqlx.Rebind(sqlx.BindType("mysql"), query)
	var rows []clothItemImageRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, query, args...); err != nil {
		return nil, err
	}
	images := make([]Image, 0, len(rows))
	for _, row := range rows {
		image, ok := row.image()
		if !ok {
			continue
		}
		images = append(images, image)
	}
	return images, nil
}

func (r *MySQLRepository) setPrimaryAsset(ctx context.Context, userID int64, clothItemID int64, assetPublicID string) (Image, error) {
	asset, err := r.findAssetForUser(ctx, userID, assetPublicID)
	if err != nil {
		return Image{}, err
	}
	if err := r.clearPrimaryAssets(ctx, clothItemID); err != nil {
		return Image{}, err
	}
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO clothes_assets
  (clothes_id, asset_id, is_primary, sort_order)
VALUES
  (?, ?, 1, 0)
`, clothItemID, asset.ID)
	if err != nil {
		return Image{}, err
	}
	if err := dbutil.RequireRowsAffected(result, "clothes item primary asset create"); err != nil {
		return Image{}, err
	}
	return Image{AssetPublicID: asset.PublicID, ObjectKey: asset.ObjectKey}, nil
}

func (r *MySQLRepository) addItemAsset(ctx context.Context, userID int64, clothItemID int64, assetPublicID string, isPrimary bool, sortOrder int) (Image, error) {
	asset, err := r.findAssetForUser(ctx, userID, assetPublicID)
	if err != nil {
		return Image{}, err
	}
	if isPrimary {
		if err := r.clearPrimaryAssets(ctx, clothItemID); err != nil {
			return Image{}, err
		}
	}
	primary := 0
	if isPrimary {
		primary = 1
	}
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO clothes_assets
  (clothes_id, asset_id, is_primary, sort_order)
VALUES
  (?, ?, ?, ?)
`, clothItemID, asset.ID, primary, sortOrder)
	if err != nil {
		return Image{}, err
	}
	if err := dbutil.RequireRowsAffected(result, "clothes item asset create"); err != nil {
		return Image{}, err
	}
	return Image{AssetPublicID: asset.PublicID, ObjectKey: asset.ObjectKey}, nil
}

func (r *MySQLRepository) clearPrimaryAssets(ctx context.Context, clothItemID int64) error {
	_, err := r.ext.ExecContext(ctx, `
UPDATE clothes_assets
SET is_primary = 0
WHERE clothes_id = ?
`, clothItemID)
	return err
}

func (r *MySQLRepository) deleteItemAssets(ctx context.Context, clothItemID int64) error {
	_, err := r.ext.ExecContext(ctx, `
DELETE FROM clothes_assets
WHERE clothes_id = ?
`, clothItemID)
	return err
}

func (r *MySQLRepository) findAssetForUser(ctx context.Context, userID int64, publicID string) (clothAssetRow, error) {
	var row clothAssetRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT id, public_id, bucket, object_key, asset_type, source
FROM files
WHERE public_id = ?
  AND owner_user_id = ?
  AND status <> 'deleted'
  AND deleted_at IS NULL
LIMIT 1
`, publicID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return clothAssetRow{}, ErrItemNotFound
	}
	if err != nil {
		return clothAssetRow{}, err
	}
	if !row.eligibleClothesPrimaryAsset(userID) {
		return clothAssetRow{}, ErrInvalidPrimaryAsset
	}
	return row, nil
}

func clothItemSelectSQL(where string) string {
	return `
SELECT
  wi.id,
  wi.public_id,
  wi.user_id,
  wi.name,
  wi.category,
  wi.color,
  wi.silhouette,
  wi.material,
  wi.season,
  wi.scene_tags,
  wi.user_notes,
  wi.is_core,
  wi.recommendation_status,
  wi.recognition_status,
  wi.status,
  wi.created_at,
  wi.updated_at,
  primary_wia.id AS primary_asset_relation_id,
  primary_wia.sort_order AS primary_asset_sort_order,
  a.public_id AS primary_asset_public_id,
  a.object_key AS primary_object_key,
  all_wia.id AS asset_relation_id,
  all_wia.asset_id AS asset_id,
  all_wia.sort_order AS asset_sort_order,
  all_wia.is_primary AS asset_is_primary,
  all_a.public_id AS asset_public_id,
  all_a.object_key AS asset_object_key
FROM clothes wi
` + primaryImageJoinSQL() + allImagesJoinSQL() + where
}

func clothItemDetailSelectSQL(where string) string {
	return `
SELECT
  wi.id,
  wi.public_id,
  wi.user_id,
  wi.name,
  wi.category,
  wi.color,
  wi.silhouette,
  wi.material,
  wi.season,
  wi.scene_tags,
  wi.user_notes,
  wi.is_core,
  wi.recommendation_status,
  wi.recognition_status,
  wi.status,
  wi.created_at,
  wi.updated_at
FROM clothes wi
` + where
}

func primaryImageJoinSQL() string {
	return `LEFT JOIN clothes_assets primary_wia
  ON primary_wia.id = (
    SELECT wia_pick.id
    FROM clothes_assets wia_pick
    JOIN files a_pick
      ON a_pick.id = wia_pick.asset_id
      AND a_pick.deleted_at IS NULL
      AND a_pick.status <> 'deleted'
      AND a_pick.asset_type = '` + clothPrimaryAssetType + `'
      AND a_pick.source = '` + clothPrimaryAssetSource + `'
      AND a_pick.bucket <> '` + localOnboardingBucket + `'
      AND a_pick.object_key LIKE CONCAT('users/', wi.user_id, '/clothes/', a_pick.public_id, '.%')
    WHERE wia_pick.clothes_id = wi.id
      AND wia_pick.is_primary = 1
    ORDER BY wia_pick.sort_order ASC, wia_pick.id DESC
    LIMIT 1
  )
LEFT JOIN files a
  ON a.id = primary_wia.asset_id
  AND a.deleted_at IS NULL
  AND a.status <> 'deleted'
`
}

func allImagesJoinSQL() string {
	return `LEFT JOIN clothes_assets all_wia
  ON all_wia.clothes_id = wi.id
LEFT JOIN files all_a
  ON all_a.id = all_wia.asset_id
  AND all_a.deleted_at IS NULL
  AND all_a.status <> 'deleted'
  AND all_a.asset_type = '` + clothPrimaryAssetType + `'
  AND all_a.source = '` + clothPrimaryAssetSource + `'
  AND all_a.bucket <> '` + localOnboardingBucket + `'
  AND all_a.object_key LIKE CONCAT('users/', wi.user_id, '/clothes/', all_a.public_id, '.%')
`
}

type txStarter interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

type clothItemRow struct {
	ID                     int64           `db:"id"`
	PublicID               string          `db:"public_id"`
	UserID                 int64           `db:"user_id"`
	Name                   string          `db:"name"`
	Category               string          `db:"category"`
	Color                  sql.NullString  `db:"color"`
	Silhouette             sql.NullString  `db:"silhouette"`
	Material               sql.NullString  `db:"material"`
	Season                 sql.NullString  `db:"season"`
	SceneTags              json.RawMessage `db:"scene_tags"`
	UserNotes              sql.NullString  `db:"user_notes"`
	IsCore                 bool            `db:"is_core"`
	RecommendationStatus   string          `db:"recommendation_status"`
	RecognitionStatus      string          `db:"recognition_status"`
	Status                 string          `db:"status"`
	PrimaryAssetRelationID sql.NullInt64   `db:"primary_asset_relation_id"`
	PrimaryAssetSortOrder  sql.NullInt64   `db:"primary_asset_sort_order"`
	PrimaryAssetPublicID   sql.NullString  `db:"primary_asset_public_id"`
	PrimaryObjectKey       sql.NullString  `db:"primary_object_key"`
	AssetRelationID        sql.NullInt64   `db:"asset_relation_id"`
	AssetID                sql.NullInt64   `db:"asset_id"`
	AssetSortOrder         sql.NullInt64   `db:"asset_sort_order"`
	AssetIsPrimary         sql.NullBool    `db:"asset_is_primary"`
	AssetPublicID          sql.NullString  `db:"asset_public_id"`
	AssetObjectKey         sql.NullString  `db:"asset_object_key"`
	CreatedAt              time.Time       `db:"created_at"`
	UpdatedAt              time.Time       `db:"updated_at"`
}

type clothItemImageRow struct {
	ID              int64          `db:"id"`
	AssetRelationID sql.NullInt64  `db:"asset_relation_id"`
	AssetSortOrder  sql.NullInt64  `db:"asset_sort_order"`
	AssetIsPrimary  sql.NullBool   `db:"asset_is_primary"`
	AssetPublicID   sql.NullString `db:"asset_public_id"`
	AssetObjectKey  sql.NullString `db:"asset_object_key"`
}

type clothItemAssetRelationRow struct {
	AssetRelationID sql.NullInt64 `db:"asset_relation_id"`
	AssetID         sql.NullInt64 `db:"asset_id"`
	AssetSortOrder  sql.NullInt64 `db:"asset_sort_order"`
	AssetIsPrimary  sql.NullBool  `db:"asset_is_primary"`
}

func (r clothItemRow) toItem() (Item, error) {
	sceneTags, err := decodeStringSlice(r.SceneTags)
	if err != nil {
		return Item{}, err
	}
	item := Item{
		ID:                   r.ID,
		PublicID:             r.PublicID,
		UserID:               r.UserID,
		Name:                 r.Name,
		Category:             r.Category,
		Color:                nullStringValue(r.Color),
		Silhouette:           nullStringValue(r.Silhouette),
		Material:             nullStringValue(r.Material),
		Season:               nullStringValue(r.Season),
		SceneTags:            sceneTags,
		UserNotes:            nullStringValue(r.UserNotes),
		IsCore:               r.IsCore,
		RecommendationStatus: r.RecommendationStatus,
		RecognitionStatus:    recognitionStatusOrDefault(r.RecognitionStatus),
		Status:               r.Status,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
	if r.PrimaryAssetPublicID.Valid {
		item.PrimaryImage = &Image{
			AssetPublicID: r.PrimaryAssetPublicID.String,
			ObjectKey:     nullStringValue(r.PrimaryObjectKey),
		}
	}
	return item, nil
}

func clothItemsFromRows(rows []clothItemRow) ([]Item, error) {
	items := make([]Item, 0, len(rows))
	indexByID := make(map[int64]int, len(rows))
	primaryByID := make(map[int64]primaryImageCandidate, len(rows))
	imagesByID := make(map[int64][]imageCandidate, len(rows))
	seenImageByID := make(map[int64]map[int64]bool, len(rows))
	for _, row := range rows {
		index, ok := indexByID[row.ID]
		if !ok {
			item, err := row.toItem()
			if err != nil {
				return nil, err
			}
			indexByID[row.ID] = len(items)
			items = append(items, item)
			index = len(items) - 1
		}
		candidate, ok := row.primaryImageCandidate()
		if !ok {
			continue
		}
		current, hasCurrent := primaryByID[row.ID]
		if !hasCurrent || candidate.betterThan(current) {
			primaryByID[row.ID] = candidate
			items[index].PrimaryImage = &Image{
				AssetPublicID: candidate.assetPublicID,
				ObjectKey:     candidate.objectKey,
			}
		}
		imageCandidate, ok := row.imageCandidate()
		if !ok {
			continue
		}
		seenByRelationID := seenImageByID[row.ID]
		if seenByRelationID == nil {
			seenByRelationID = map[int64]bool{}
			seenImageByID[row.ID] = seenByRelationID
		}
		if seenByRelationID[imageCandidate.relationID] {
			continue
		}
		seenByRelationID[imageCandidate.relationID] = true
		imagesByID[row.ID] = append(imagesByID[row.ID], imageCandidate)
	}
	for i := range items {
		candidates := imagesByID[items[i].ID]
		sort.SliceStable(candidates, func(left, right int) bool {
			return candidates[left].betterThan(candidates[right])
		})
		if len(candidates) == 0 && items[i].PrimaryImage != nil {
			items[i].Images = []Image{*items[i].PrimaryImage}
			continue
		}
		images := make([]Image, 0, len(candidates))
		for _, candidate := range candidates {
			images = append(images, Image{
				AssetPublicID: candidate.assetPublicID,
				ObjectKey:     candidate.objectKey,
			})
		}
		items[i].Images = images
	}
	return items, nil
}

func firstClothesItemFromRows(rows []clothItemRow) (Item, error) {
	items, err := clothItemsFromRows(rows)
	if err != nil {
		return Item{}, err
	}
	if len(items) == 0 {
		return Item{}, ErrItemNotFound
	}
	return items[0], nil
}

type primaryImageCandidate struct {
	relationID    int64
	sortOrder     int64
	assetPublicID string
	objectKey     string
}

func (r clothItemRow) primaryImageCandidate() (primaryImageCandidate, bool) {
	if !r.PrimaryAssetPublicID.Valid {
		return primaryImageCandidate{}, false
	}
	candidate := primaryImageCandidate{
		assetPublicID: r.PrimaryAssetPublicID.String,
		objectKey:     nullStringValue(r.PrimaryObjectKey),
	}
	if r.PrimaryAssetRelationID.Valid {
		candidate.relationID = r.PrimaryAssetRelationID.Int64
	}
	if r.PrimaryAssetSortOrder.Valid {
		candidate.sortOrder = r.PrimaryAssetSortOrder.Int64
	}
	return candidate, true
}

func (c primaryImageCandidate) betterThan(other primaryImageCandidate) bool {
	if c.sortOrder != other.sortOrder {
		return c.sortOrder < other.sortOrder
	}
	return c.relationID > other.relationID
}

type imageCandidate struct {
	relationID    int64
	sortOrder     int64
	isPrimary     bool
	assetPublicID string
	objectKey     string
}

func (r clothItemRow) imageCandidate() (imageCandidate, bool) {
	if !r.AssetPublicID.Valid {
		return imageCandidate{}, false
	}
	candidate := imageCandidate{
		assetPublicID: r.AssetPublicID.String,
		objectKey:     nullStringValue(r.AssetObjectKey),
	}
	if r.AssetRelationID.Valid {
		candidate.relationID = r.AssetRelationID.Int64
	}
	if r.AssetSortOrder.Valid {
		candidate.sortOrder = r.AssetSortOrder.Int64
	}
	if r.AssetIsPrimary.Valid {
		candidate.isPrimary = r.AssetIsPrimary.Bool
	}
	return candidate, true
}

func (r clothItemImageRow) image() (Image, bool) {
	if !r.AssetPublicID.Valid {
		return Image{}, false
	}
	return Image{
		AssetPublicID: r.AssetPublicID.String,
		ObjectKey:     nullStringValue(r.AssetObjectKey),
	}, true
}

func (c imageCandidate) betterThan(other imageCandidate) bool {
	if c.sortOrder != other.sortOrder {
		return c.sortOrder < other.sortOrder
	}
	if c.isPrimary != other.isPrimary {
		return c.isPrimary
	}
	return c.relationID < other.relationID
}

type clothAssetRow struct {
	ID        int64  `db:"id"`
	PublicID  string `db:"public_id"`
	Bucket    string `db:"bucket"`
	ObjectKey string `db:"object_key"`
	AssetType string `db:"asset_type"`
	Source    string `db:"source"`
}

type systemConfigOptionRow struct {
	Key   string `db:"key"`
	Value []byte `db:"value"`
}

func parseOptionItems(raw []byte) ([]string, error) {
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result, nil
}

func (r clothAssetRow) eligibleClothesPrimaryAsset(userID int64) bool {
	if strings.TrimSpace(r.PublicID) == "" || strings.TrimSpace(r.Bucket) == "" {
		return false
	}
	if r.Bucket == localOnboardingBucket {
		return false
	}
	if r.AssetType != clothPrimaryAssetType || r.Source != clothPrimaryAssetSource {
		return false
	}
	prefix := fmt.Sprintf("users/%d/clothes/", userID)
	objectKey := strings.TrimSpace(r.ObjectKey)
	if !strings.HasPrefix(objectKey, prefix) {
		return false
	}
	filename := strings.TrimPrefix(objectKey, prefix)
	return strings.HasPrefix(filename, r.PublicID+".")
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

func decodeStringSlice(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func recognitionStatusOrDefault(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return RecognitionStatusSucceeded
	}
	return value
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
