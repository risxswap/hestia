package clothes

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestClothesItemsFromRowsDeduplicatesRepeatedPrimaryRows(t *testing.T) {
	rows := []clothItemRow{
		routeBaseClothesItemRow("wdi_owned", "ast_old", 10, 0),
		routeBaseClothesItemRow("wdi_owned", "ast_new", 12, 0),
	}

	items, err := clothItemsFromRows(rows)
	if err != nil {
		t.Fatalf("convert rows: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected one deduplicated item, got %#v", items)
	}
	if items[0].PrimaryImage == nil {
		t.Fatalf("expected primary image, got %#v", items[0])
	}
	if items[0].PrimaryImage.AssetPublicID != "ast_new" {
		t.Fatalf("expected newest relation id to win sort tie, got %#v", items[0].PrimaryImage)
	}
}

func TestFirstClothesItemFromRowsSelectsDeterministicPrimary(t *testing.T) {
	rows := []clothItemRow{
		routeBaseClothesItemRow("wdi_owned", "ast_later_relation", 20, 3),
		routeBaseClothesItemRow("wdi_owned", "ast_lower_sort", 5, 1),
		routeBaseClothesItemRow("wdi_owned", "ast_higher_sort", 30, 8),
	}

	item, err := firstClothesItemFromRows(rows)
	if err != nil {
		t.Fatalf("convert first row: %v", err)
	}

	if item.PrimaryImage == nil {
		t.Fatalf("expected primary image, got %#v", item)
	}
	if item.PrimaryImage.AssetPublicID != "ast_lower_sort" {
		t.Fatalf("expected lowest sort_order primary image, got %#v", item.PrimaryImage)
	}
}

func TestClothesItemsFromRowsCollectsAllImagesForSwiper(t *testing.T) {
	rows := []clothItemRow{
		routeBaseClothesItemRow("wdi_owned", "ast_primary", 20, 0),
		routeBaseClothesItemRow("wdi_owned", "ast_side", 21, 1),
		routeBaseClothesItemRow("wdi_owned", "ast_detail", 22, 2),
	}
	rows[0].AssetRelationID = sql.NullInt64{Int64: 20, Valid: true}
	rows[0].AssetSortOrder = sql.NullInt64{Int64: 0, Valid: true}
	rows[0].AssetIsPrimary = sql.NullBool{Bool: true, Valid: true}
	rows[0].AssetPublicID = sql.NullString{String: "ast_primary", Valid: true}
	rows[0].AssetObjectKey = sql.NullString{String: "users/12/clothes/ast_primary.jpg", Valid: true}
	rows[1].AssetRelationID = sql.NullInt64{Int64: 21, Valid: true}
	rows[1].AssetSortOrder = sql.NullInt64{Int64: 1, Valid: true}
	rows[1].AssetIsPrimary = sql.NullBool{Bool: false, Valid: true}
	rows[1].AssetPublicID = sql.NullString{String: "ast_side", Valid: true}
	rows[1].AssetObjectKey = sql.NullString{String: "users/12/clothes/ast_side.jpg", Valid: true}
	rows[2].AssetRelationID = sql.NullInt64{Int64: 22, Valid: true}
	rows[2].AssetSortOrder = sql.NullInt64{Int64: 2, Valid: true}
	rows[2].AssetIsPrimary = sql.NullBool{Bool: false, Valid: true}
	rows[2].AssetPublicID = sql.NullString{String: "ast_detail", Valid: true}
	rows[2].AssetObjectKey = sql.NullString{String: "users/12/clothes/ast_detail.jpg", Valid: true}

	items, err := clothItemsFromRows(rows)
	if err != nil {
		t.Fatalf("convert rows: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %#v", items)
	}
	if len(items[0].Images) != 3 {
		t.Fatalf("expected three images, got %#v", items[0].Images)
	}
	if items[0].Images[0].AssetPublicID != "ast_primary" ||
		items[0].Images[1].AssetPublicID != "ast_side" ||
		items[0].Images[2].AssetPublicID != "ast_detail" {
		t.Fatalf("expected sorted image assets, got %#v", items[0].Images)
	}
}

func TestMySQLRepositoryListClothesOptionsReadsSystemConfigs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))
	rows := sqlmock.NewRows([]string{"key", "value"}).
		AddRow("categories", `["上装"]`).
		AddRow("colors", `["雾霾蓝"]`).
		AddRow("materials", `["棉"]`).
		AddRow("seasons", `["春秋"]`).
		AddRow("silhouettes", `["微宽松"]`)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `key`, `value`")).
		WillReturnRows(rows)

	options, err := repo.ListClothesOptions(context.Background())
	if err != nil {
		t.Fatalf("list clothes options: %v", err)
	}

	if options.Categories[0] != "上装" ||
		options.Colors[0] != "雾霾蓝" ||
		options.Materials[0] != "棉" ||
		options.Seasons[0] != "春秋" ||
		options.Silhouettes[0] != "微宽松" {
		t.Fatalf("expected options from system configs, got %#v", options)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestPrimaryImageJoinSQLReadsFilesTable(t *testing.T) {
	sql := primaryImageJoinSQL()
	if !regexp.MustCompile(`(?s)JOIN files a_pick`).MatchString(sql) {
		t.Fatalf("expected primary image subquery to join files table, got %s", sql)
	}
	if !regexp.MustCompile(`(?s)LEFT JOIN files a`).MatchString(sql) {
		t.Fatalf("expected primary image join to read files table, got %s", sql)
	}
	if regexp.MustCompile(`(?s)JOIN assets|LEFT JOIN assets`).MatchString(sql) {
		t.Fatalf("primary image join should not read assets table, got %s", sql)
	}
}

func TestMySQLRepositoryUpdateItemAllowsUnchangedRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))
	current := routeBaseClothesItemRow("wdi_owned", "", 0, 0)
	name := current.Name

	mock.ExpectBegin()
	expectFindClothesItem(mock, current)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE clothes")).
		WithArgs(name, current.ID, current.UserID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectFindClothesItem(mock, current)
	mock.ExpectCommit()

	item, err := repo.UpdateItem(context.Background(), current.UserID, current.PublicID, UpdateInput{Name: &name})
	if err != nil {
		t.Fatalf("update unchanged clothes item: %v", err)
	}
	if item.PublicID != current.PublicID {
		t.Fatalf("expected updated item %q, got %#v", current.PublicID, item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestMySQLRepositoryFindItemLoadsImagesWithSeparateQuery(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(matchClothesDetailQueries)))
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))
	front := routeBaseClothesItemRow("wdi_owned", "ast_front", 20, 0)
	side := routeBaseClothesItemRow("wdi_owned", "ast_side", 21, 1)
	side.AssetIsPrimary = sql.NullBool{Bool: false, Valid: true}

	expectFindClothesItemWithoutImageJoin(mock, front)
	expectFindClothesItemAssetRelationsWithMatcher(mock, front, side)
	expectFindClothesItemImageFilesWithMatcher(mock, front, side)

	item, err := repo.FindItemForUser(context.Background(), 12, "wdi_owned")
	if err != nil {
		t.Fatalf("find clothes item: %v", err)
	}
	if len(item.Images) != 2 {
		t.Fatalf("expected two separately loaded images, got %#v", item.Images)
	}
	if item.Images[0].AssetPublicID != "ast_front" || item.Images[1].AssetPublicID != "ast_side" {
		t.Fatalf("expected separately loaded images in order, got %#v", item.Images)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func expectFindClothesItem(mock sqlmock.Sqlmock, rows ...clothItemRow) {
	if len(rows) == 0 {
		panic("expectFindClothesItem requires at least one row")
	}
	row := rows[0]
	mock.ExpectQuery("(?s)SELECT.*FROM clothes wi.*WHERE wi.user_id = \\?.*wi.public_id = \\?.*wi.deleted_at IS NULL.*LIMIT 1\\s*$").
		WithArgs(row.UserID, row.PublicID, StatusDeleted).
		WillReturnRows(clothesItemRows(false, rows...))
	expectFindClothesItemAssetRelations(mock, rows...)
	expectFindClothesItemImageFiles(mock, rows...)
}

func expectFindClothesItemWithoutImageJoin(mock sqlmock.Sqlmock, rows ...clothItemRow) {
	if len(rows) == 0 {
		panic("expectFindClothesItemWithoutImageJoin requires at least one row")
	}
	row := rows[0]
	mock.ExpectQuery("clothes-item-detail-query").
		WithArgs(row.UserID, row.PublicID, StatusDeleted).
		WillReturnRows(clothesItemRows(false, rows...))
}

func expectFindClothesItemAssetRelations(mock sqlmock.Sqlmock, rows ...clothItemRow) {
	expectFindClothesItemAssetRelationsWithPattern(mock, "(?s)SELECT.*FROM clothes_assets wia.*WHERE wia.clothes_id = \\?.*ORDER BY wia.sort_order ASC, wia.is_primary DESC, wia.id ASC\\s*$", rows...)
}

func expectFindClothesItemAssetRelationsWithMatcher(mock sqlmock.Sqlmock, rows ...clothItemRow) {
	expectFindClothesItemAssetRelationsWithPattern(mock, "clothes-item-asset-relations-query", rows...)
}

func expectFindClothesItemAssetRelationsWithPattern(mock sqlmock.Sqlmock, pattern string, rows ...clothItemRow) {
	if len(rows) == 0 {
		panic("expectFindClothesItemAssetRelations requires at least one row")
	}
	row := rows[0]
	mock.ExpectQuery(pattern).
		WithArgs(row.ID).
		WillReturnRows(clothesAssetRelationRows(validImageRows(rows)...))
}

func expectFindClothesItemImageFiles(mock sqlmock.Sqlmock, rows ...clothItemRow) {
	expectFindClothesItemImageFilesWithPattern(mock, "(?s)SELECT.*FROM files.*WHERE id IN \\(.*\\).*owner_user_id = \\?.*ORDER BY FIELD\\(id,.*\\)\\s*$", rows...)
}

func expectFindClothesItemImageFilesWithMatcher(mock sqlmock.Sqlmock, rows ...clothItemRow) {
	expectFindClothesItemImageFilesWithPattern(mock, "clothes-item-image-files-query", rows...)
}

func expectFindClothesItemImageFilesWithPattern(mock sqlmock.Sqlmock, pattern string, rows ...clothItemRow) {
	validRows := validImageRows(rows)
	if len(validRows) == 0 {
		return
	}
	args := make([]driver.Value, 0, len(validRows)+4)
	for _, row := range validRows {
		args = append(args, row.AssetID)
	}
	args = append(args, validRows[0].UserID, clothPrimaryAssetType, clothPrimaryAssetSource, localOnboardingBucket)
	for _, row := range validRows {
		args = append(args, row.AssetID)
	}
	mock.ExpectQuery(pattern).
		WithArgs(args...).
		WillReturnRows(clothesFileRows(validRows...))
}

func matchClothesDetailQueries(expectedSQL string, actualSQL string) error {
	switch expectedSQL {
	case "clothes-item-detail-query":
		if strings.Contains(actualSQL, "clothes_assets") {
			return fmt.Errorf("detail item query must not read clothes_assets: %s", actualSQL)
		}
		required := []string{
			"FROM clothes wi",
			"WHERE wi.user_id = ?",
			"wi.public_id = ?",
			"wi.deleted_at IS NULL",
			"LIMIT 1",
		}
		for _, fragment := range required {
			if !strings.Contains(actualSQL, fragment) {
				return fmt.Errorf("detail item query missing %q: %s", fragment, actualSQL)
			}
		}
		return nil
	case "clothes-item-asset-relations-query":
		if strings.Contains(actualSQL, "JOIN") {
			return fmt.Errorf("detail asset relation query must not join files: %s", actualSQL)
		}
		required := []string{
			"FROM clothes_assets wia",
			"WHERE wia.clothes_id = ?",
			"ORDER BY wia.sort_order ASC, wia.is_primary DESC, wia.id ASC",
		}
		for _, fragment := range required {
			if !strings.Contains(actualSQL, fragment) {
				return fmt.Errorf("detail images query missing %q: %s", fragment, actualSQL)
			}
		}
		return nil
	case "clothes-item-image-files-query":
		if strings.Contains(actualSQL, "JOIN") {
			return fmt.Errorf("detail image file query must not join other tables: %s", actualSQL)
		}
		required := []string{
			"FROM files",
			"WHERE id IN",
			"owner_user_id = ?",
			"asset_type = ?",
			"source = ?",
			"bucket <> ?",
			"ORDER BY FIELD(id,",
		}
		for _, fragment := range required {
			if !strings.Contains(actualSQL, fragment) {
				return fmt.Errorf("detail image file query missing %q: %s", fragment, actualSQL)
			}
		}
		return nil
	default:
		matched, err := regexp.MatchString(expectedSQL, actualSQL)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("could not match actual sql: %s with expected regexp: %s", actualSQL, expectedSQL)
		}
		return nil
	}
}

func clothesItemRows(includeJoinedImages bool, rows ...clothItemRow) *sqlmock.Rows {
	columns := []string{
		"id",
		"public_id",
		"user_id",
		"name",
		"category",
		"color",
		"silhouette",
		"material",
		"season",
		"scene_tags",
		"user_notes",
		"is_core",
		"recommendation_status",
		"recognition_status",
		"status",
		"created_at",
		"updated_at",
		"primary_asset_relation_id",
		"primary_asset_sort_order",
		"primary_asset_public_id",
		"primary_object_key",
	}
	if includeJoinedImages {
		columns = append(columns,
			"asset_relation_id",
			"asset_id",
			"asset_sort_order",
			"asset_is_primary",
			"asset_public_id",
			"asset_object_key",
		)
	}
	result := sqlmock.NewRows(columns)
	for _, row := range rows {
		values := []driver.Value{
			row.ID,
			row.PublicID,
			row.UserID,
			row.Name,
			row.Category,
			row.Color,
			row.Silhouette,
			row.Material,
			row.Season,
			row.SceneTags,
			row.UserNotes,
			row.IsCore,
			row.RecommendationStatus,
			row.RecognitionStatus,
			row.Status,
			row.CreatedAt,
			row.UpdatedAt,
			row.PrimaryAssetRelationID,
			row.PrimaryAssetSortOrder,
			row.PrimaryAssetPublicID,
			row.PrimaryObjectKey,
		}
		if includeJoinedImages {
			values = append(values,
				row.AssetRelationID,
				row.AssetID,
				row.AssetSortOrder,
				row.AssetIsPrimary,
				row.AssetPublicID,
				row.AssetObjectKey,
			)
		}
		result.AddRow(values...)
	}
	return result
}

func clothesAssetRelationRows(rows ...clothItemRow) *sqlmock.Rows {
	result := sqlmock.NewRows([]string{
		"asset_relation_id",
		"asset_id",
		"asset_sort_order",
		"asset_is_primary",
	})
	for _, row := range rows {
		result.AddRow(
			row.AssetRelationID,
			row.AssetID,
			row.AssetSortOrder,
			row.AssetIsPrimary,
		)
	}
	return result
}

func clothesFileRows(rows ...clothItemRow) *sqlmock.Rows {
	result := sqlmock.NewRows([]string{
		"id",
		"asset_public_id",
		"asset_object_key",
	})
	for _, row := range rows {
		result.AddRow(
			row.AssetID,
			row.AssetPublicID,
			row.AssetObjectKey,
		)
	}
	return result
}

func validImageRows(rows []clothItemRow) []clothItemRow {
	result := make([]clothItemRow, 0, len(rows))
	for _, row := range rows {
		if row.AssetPublicID.Valid {
			result = append(result, row)
		}
	}
	return result
}

func routeBaseClothesItemRow(publicID string, assetPublicID string, relationID int64, sortOrder int64) clothItemRow {
	now := time.Now().UTC()
	return clothItemRow{
		ID:                   1,
		PublicID:             publicID,
		UserID:               12,
		Name:                 "米白衬衫",
		Category:             "上装",
		SceneTags:            []byte(`["通勤"]`),
		IsCore:               true,
		RecommendationStatus: RecommendationStatusNormal,
		RecognitionStatus:    RecognitionStatusSucceeded,
		Status:               StatusActive,
		PrimaryAssetRelationID: sql.NullInt64{
			Int64: relationID,
			Valid: true,
		},
		PrimaryAssetSortOrder: sql.NullInt64{
			Int64: sortOrder,
			Valid: true,
		},
		PrimaryAssetPublicID: sql.NullString{
			String: assetPublicID,
			Valid:  true,
		},
		PrimaryObjectKey: sql.NullString{
			String: assetPublicID + ".jpg",
			Valid:  true,
		},
		AssetRelationID: sql.NullInt64{
			Int64: relationID,
			Valid: assetPublicID != "",
		},
		AssetID: sql.NullInt64{
			Int64: relationID + 1000,
			Valid: assetPublicID != "",
		},
		AssetSortOrder: sql.NullInt64{
			Int64: sortOrder,
			Valid: assetPublicID != "",
		},
		AssetIsPrimary: sql.NullBool{
			Bool:  true,
			Valid: assetPublicID != "",
		},
		AssetPublicID: sql.NullString{
			String: assetPublicID,
			Valid:  assetPublicID != "",
		},
		AssetObjectKey: sql.NullString{
			String: assetPublicID + ".jpg",
			Valid:  assetPublicID != "",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
