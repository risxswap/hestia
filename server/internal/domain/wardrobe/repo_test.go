package wardrobe

import (
	"database/sql"
	"testing"
	"time"
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

func routeBaseWardrobeItemRow(publicID string, assetPublicID string, relationID int64, sortOrder int64) wardrobeItemRow {
	now := time.Now().UTC()
	return wardrobeItemRow{
		ID:                   1,
		PublicID:             publicID,
		UserID:               12,
		Name:                 "米白衬衫",
		Category:             "top",
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
