package wardrobe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"hestia/server/internal/common/dbutil"

	"github.com/jmoiron/sqlx"
)

const (
	wardrobePrimaryAssetType   = "wardrobe_item_photo"
	wardrobePrimaryAssetSource = "miniapp_upload"
	localOnboardingBucket      = "local-onboarding"
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
		return nil, errors.New("wardrobe repository database is nil")
	}
	for i := range items {
		if items[i].RecommendationStatus == "" {
			items[i].RecommendationStatus = RecommendationStatusNormal
		}
		result, err := r.ext.ExecContext(ctx, `
INSERT INTO wardrobe_items
  (public_id, user_id, name, category, color, silhouette, material, season, user_notes, is_core, recommendation_status, recognition_status, status)
VALUES
  (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?)
`, items[i].PublicID, items[i].UserID, items[i].Name, items[i].Category, items[i].Color, items[i].Silhouette, items[i].Material, items[i].Season, items[i].UserNotes, items[i].IsCore, items[i].RecommendationStatus, recognitionStatusOrDefault(items[i].RecognitionStatus), items[i].Status)
		if err != nil {
			return nil, err
		}
		id, err := dbutil.RequireLastInsertID(result, "wardrobe item create")
		if err != nil {
			return nil, err
		}
		items[i].ID = id
	}
	return items, nil
}

func (r *MySQLRepository) ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error) {
	if r == nil || r.ext == nil {
		return nil, errors.New("wardrobe repository database is nil")
	}
	query := wardrobeItemSelectSQL(`
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

	var rows []wardrobeItemRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, query, args...); err != nil {
		return nil, err
	}
	return wardrobeItemsFromRows(rows)
}

func (r *MySQLRepository) ListWardrobeOptions(ctx context.Context) (WardrobeOptions, error) {
	if r == nil || r.ext == nil {
		return WardrobeOptions{}, errors.New("wardrobe repository database is nil")
	}
	var rows []systemConfigOptionRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, `
SELECT `+"`key`, `value`"+`
FROM system_configs
WHERE `+"`group`"+` = ?
  AND status = ?
  AND `+"`key`"+` IN ('categories', 'materials', 'seasons', 'silhouettes')
`, "wardrobe.item_options", StatusActive); err != nil {
		return WardrobeOptions{}, err
	}

	options := WardrobeOptions{}
	for _, row := range rows {
		items, err := parseOptionItems(row.Value)
		if err != nil {
			return WardrobeOptions{}, err
		}
		switch row.Key {
		case "categories":
			options.Categories = items
		case "materials":
			options.Materials = items
		case "seasons":
			options.Seasons = items
		case "silhouettes":
			options.Silhouettes = items
		}
	}
	return mergeWithDefaultWardrobeOptions(options), nil
}

func (r *MySQLRepository) CreateItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	if r == nil || r.ext == nil {
		return Item{}, errors.New("wardrobe repository database is nil")
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
		return Item{}, errors.New("wardrobe repository database is nil")
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
INSERT INTO wardrobe_items
  (public_id, user_id, name, category, color, silhouette, material, season, scene_tags, user_notes, is_core, recommendation_status, recognition_status, status)
VALUES
  (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON), NULLIF(?, ''), ?, ?, ?, ?)
`, item.PublicID, item.UserID, item.Name, item.Category, item.Color, item.Silhouette, item.Material, item.Season, sceneTags, item.UserNotes, item.IsCore, item.RecommendationStatus, item.RecognitionStatus, item.Status)
	if err != nil {
		return Item{}, err
	}
	id, err := dbutil.RequireLastInsertID(result, "wardrobe item create")
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
		return Item{}, errors.New("wardrobe repository database is nil")
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
		return Item{}, errors.New("wardrobe repository database is nil")
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
UPDATE wardrobe_items
SET `+strings.Join(sets, ", ")+`
WHERE id = ?
  AND user_id = ?
  AND status <> 'deleted'
  AND deleted_at IS NULL
`, args...)
		if err != nil {
			return Item{}, err
		}
		if err := dbutil.RequireRowsAffected(result, "wardrobe item update"); err != nil {
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

func (r *MySQLRepository) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	if r == nil || r.ext == nil {
		return errors.New("wardrobe repository database is nil")
	}
	var row struct {
		ID        int64        `db:"id"`
		Status    string       `db:"status"`
		DeletedAt sql.NullTime `db:"deleted_at"`
	}
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT id, status, deleted_at
FROM wardrobe_items
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
UPDATE wardrobe_items
SET status = ?, deleted_at = CURRENT_TIMESTAMP(3)
WHERE id = ?
  AND user_id = ?
`, StatusDeleted, row.ID, userID)
	if err != nil {
		return err
	}
	return dbutil.RequireRowsAffected(result, "wardrobe item soft delete")
}

func (r *MySQLRepository) FindRecognizableAsset(ctx context.Context, userID int64, assetPublicID string) (Image, error) {
	if r == nil || r.ext == nil {
		return Image{}, errors.New("wardrobe repository database is nil")
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
	var rows []wardrobeItemRow
	err := sqlx.SelectContext(ctx, r.ext, &rows, wardrobeItemSelectSQL(`
WHERE wi.user_id = ?
  AND wi.public_id = ?
  AND wi.status <> ?
  AND wi.deleted_at IS NULL
LIMIT 1
`), userID, publicID, StatusDeleted)
	if err != nil {
		return Item{}, err
	}
	return firstWardrobeItemFromRows(rows)
}

func (r *MySQLRepository) setPrimaryAsset(ctx context.Context, userID int64, wardrobeItemID int64, assetPublicID string) (Image, error) {
	asset, err := r.findAssetForUser(ctx, userID, assetPublicID)
	if err != nil {
		return Image{}, err
	}
	if err := r.clearPrimaryAssets(ctx, wardrobeItemID); err != nil {
		return Image{}, err
	}
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO wardrobe_item_assets
  (wardrobe_item_id, asset_id, is_primary, sort_order)
VALUES
  (?, ?, 1, 0)
`, wardrobeItemID, asset.ID)
	if err != nil {
		return Image{}, err
	}
	if err := dbutil.RequireRowsAffected(result, "wardrobe item primary asset create"); err != nil {
		return Image{}, err
	}
	return Image{AssetPublicID: asset.PublicID, ObjectKey: asset.ObjectKey}, nil
}

func (r *MySQLRepository) addItemAsset(ctx context.Context, userID int64, wardrobeItemID int64, assetPublicID string, isPrimary bool, sortOrder int) (Image, error) {
	asset, err := r.findAssetForUser(ctx, userID, assetPublicID)
	if err != nil {
		return Image{}, err
	}
	if isPrimary {
		if err := r.clearPrimaryAssets(ctx, wardrobeItemID); err != nil {
			return Image{}, err
		}
	}
	primary := 0
	if isPrimary {
		primary = 1
	}
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO wardrobe_item_assets
  (wardrobe_item_id, asset_id, is_primary, sort_order)
VALUES
  (?, ?, ?, ?)
`, wardrobeItemID, asset.ID, primary, sortOrder)
	if err != nil {
		return Image{}, err
	}
	if err := dbutil.RequireRowsAffected(result, "wardrobe item asset create"); err != nil {
		return Image{}, err
	}
	return Image{AssetPublicID: asset.PublicID, ObjectKey: asset.ObjectKey}, nil
}

func (r *MySQLRepository) clearPrimaryAssets(ctx context.Context, wardrobeItemID int64) error {
	_, err := r.ext.ExecContext(ctx, `
UPDATE wardrobe_item_assets
SET is_primary = 0
WHERE wardrobe_item_id = ?
`, wardrobeItemID)
	return err
}

func (r *MySQLRepository) findAssetForUser(ctx context.Context, userID int64, publicID string) (wardrobeAssetRow, error) {
	var row wardrobeAssetRow
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
		return wardrobeAssetRow{}, ErrItemNotFound
	}
	if err != nil {
		return wardrobeAssetRow{}, err
	}
	if !row.eligibleWardrobePrimaryAsset(userID) {
		return wardrobeAssetRow{}, ErrInvalidPrimaryAsset
	}
	return row, nil
}

func wardrobeItemSelectSQL(where string) string {
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
  a.object_key AS primary_object_key
FROM wardrobe_items wi
` + primaryImageJoinSQL() + where
}

func primaryImageJoinSQL() string {
	return `LEFT JOIN wardrobe_item_assets primary_wia
  ON primary_wia.id = (
    SELECT wia_pick.id
    FROM wardrobe_item_assets wia_pick
    JOIN files a_pick
      ON a_pick.id = wia_pick.asset_id
      AND a_pick.deleted_at IS NULL
      AND a_pick.status <> 'deleted'
      AND a_pick.asset_type = '` + wardrobePrimaryAssetType + `'
      AND a_pick.source = '` + wardrobePrimaryAssetSource + `'
      AND a_pick.bucket <> '` + localOnboardingBucket + `'
      AND a_pick.object_key LIKE CONCAT('users/', wi.user_id, '/wardrobe/', a_pick.public_id, '.%')
    WHERE wia_pick.wardrobe_item_id = wi.id
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

type txStarter interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

type wardrobeItemRow struct {
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
	CreatedAt              time.Time       `db:"created_at"`
	UpdatedAt              time.Time       `db:"updated_at"`
}

func (r wardrobeItemRow) toItem() (Item, error) {
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

func wardrobeItemsFromRows(rows []wardrobeItemRow) ([]Item, error) {
	items := make([]Item, 0, len(rows))
	indexByID := make(map[int64]int, len(rows))
	primaryByID := make(map[int64]primaryImageCandidate, len(rows))
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
	}
	return items, nil
}

func firstWardrobeItemFromRows(rows []wardrobeItemRow) (Item, error) {
	items, err := wardrobeItemsFromRows(rows)
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

func (r wardrobeItemRow) primaryImageCandidate() (primaryImageCandidate, bool) {
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

type wardrobeAssetRow struct {
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

func parseOptionItems(raw []byte) ([]OptionItem, error) {
	var items []OptionItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	result := make([]OptionItem, 0, len(items))
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		value := strings.TrimSpace(item.Value)
		if label == "" || value == "" {
			continue
		}
		result = append(result, OptionItem{
			Label: label,
			Value: value,
		})
	}
	return result, nil
}

func (r wardrobeAssetRow) eligibleWardrobePrimaryAsset(userID int64) bool {
	if strings.TrimSpace(r.PublicID) == "" || strings.TrimSpace(r.Bucket) == "" {
		return false
	}
	if r.Bucket == localOnboardingBucket {
		return false
	}
	if r.AssetType != wardrobePrimaryAssetType || r.Source != wardrobePrimaryAssetSource {
		return false
	}
	prefix := fmt.Sprintf("users/%d/wardrobe/", userID)
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
