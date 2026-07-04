package wardrobe

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestWardrobeItemsFromRowsDeduplicatesRepeatedPrimaryRows(t *testing.T) {
	rows := []wardrobeItemRow{
		routeBaseWardrobeItemRow("wdi_owned", "ast_old", 10, 0),
		routeBaseWardrobeItemRow("wdi_owned", "ast_new", 12, 0),
	}

	items, err := wardrobeItemsFromRows(rows)
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

func TestFirstWardrobeItemFromRowsSelectsDeterministicPrimary(t *testing.T) {
	rows := []wardrobeItemRow{
		routeBaseWardrobeItemRow("wdi_owned", "ast_later_relation", 20, 3),
		routeBaseWardrobeItemRow("wdi_owned", "ast_lower_sort", 5, 1),
		routeBaseWardrobeItemRow("wdi_owned", "ast_higher_sort", 30, 8),
	}

	item, err := firstWardrobeItemFromRows(rows)
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

func TestMySQLRepositoryListWardrobeOptionsReadsSystemConfigs(t *testing.T) {
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

	options, err := repo.ListWardrobeOptions(context.Background())
	if err != nil {
		t.Fatalf("list wardrobe options: %v", err)
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

func routeBaseWardrobeItemRow(publicID string, assetPublicID string, relationID int64, sortOrder int64) wardrobeItemRow {
	now := time.Now().UTC()
	return wardrobeItemRow{
		ID:                   1,
		PublicID:             publicID,
		UserID:               12,
		Name:                 "米白衬衫",
		Category:             "上装",
		SceneTags:            []byte(`["通勤"]`),
		IsCore:               true,
		RecommendationStatus: RecommendationStatusNormal,
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
		CreatedAt: now,
		UpdatedAt: now,
	}
}
