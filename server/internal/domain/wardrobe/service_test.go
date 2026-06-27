package wardrobe

import (
	"context"
	"testing"
	"time"
)

type captureWardrobeRepo struct {
	created                  []Item
	items                    []Item
	primaryAssetPublicID     string
	lastUpdatePublicID       string
	lastUpdateRecommendation *string
	lastUpdateInput          UpdateInput
}

type coreOnlyWardrobeRepo struct{}

func (r coreOnlyWardrobeRepo) CreateCoreItems(_ context.Context, items []Item) ([]Item, error) {
	return items, nil
}

func (r *captureWardrobeRepo) CreateCoreItems(_ context.Context, items []Item) ([]Item, error) {
	r.created = append(r.created, items...)
	return items, nil
}

func (r *captureWardrobeRepo) ListItems(_ context.Context, userID int64, filter ListFilter) ([]Item, error) {
	var result []Item
	for _, item := range r.items {
		if item.UserID != userID {
			continue
		}
		if filter.Category != "" && item.Category != filter.Category {
			continue
		}
		if filter.RecommendationStatus != "" && item.RecommendationStatus != filter.RecommendationStatus {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *captureWardrobeRepo) CreateItem(_ context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	r.created = append(r.created, item)
	r.primaryAssetPublicID = primaryAssetPublicID
	return item, nil
}

func (r *captureWardrobeRepo) UpdateItem(_ context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	r.lastUpdatePublicID = publicID
	r.lastUpdateInput = input
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != StatusDeleted {
			if input.RecommendationStatus != nil {
				item.RecommendationStatus = *input.RecommendationStatus
				r.lastUpdateRecommendation = input.RecommendationStatus
			}
			return item, nil
		}
	}
	return Item{}, ErrItemNotFound
}

func (r *captureWardrobeRepo) SoftDeleteItem(_ context.Context, userID int64, publicID string) error {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID {
			return nil
		}
	}
	return ErrItemNotFound
}

func TestCreateCoreItemsDefaultsRecommendationStatusNormal(t *testing.T) {
	repo := &captureWardrobeRepo{}
	service := NewService(repo)

	_, err := service.CreateCoreItems(context.Background(), 12, []Input{{Name: " 米白衬衫 ", Category: "top"}})
	if err != nil {
		t.Fatalf("create core items: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected one created item, got %#v", repo.created)
	}
	if repo.created[0].RecommendationStatus != RecommendationStatusNormal {
		t.Fatalf("expected normal recommendation status, got %q", repo.created[0].RecommendationStatus)
	}
	if !repo.created[0].IsCore {
		t.Fatal("expected onboarding core item to stay core")
	}
}

func TestCreateItemRejectsInvalidRecommendationStatus(t *testing.T) {
	service := NewService(&captureWardrobeRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{
		Name:                 "黑色西装",
		Category:             "outerwear",
		RecommendationStatus: "hidden",
	})
	if err == nil {
		t.Fatal("expected invalid recommendation status error")
	}
	if err != ErrInvalidRecommendationStatus {
		t.Fatalf("expected ErrInvalidRecommendationStatus, got %v", err)
	}
}

func TestCreateItemRejectsEmptyNameWithSentinelError(t *testing.T) {
	service := NewService(&captureWardrobeRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{Name: "   "})
	if err != ErrInvalidItemName {
		t.Fatalf("expected ErrInvalidItemName, got %v", err)
	}
}

func TestCreateItemTrimsAndDefaultsFields(t *testing.T) {
	repo := &captureWardrobeRepo{}
	service := NewService(repo)

	item, err := service.CreateItem(context.Background(), 12, CreateInput{
		Name:                 " 黑色西装 ",
		Category:             "   ",
		Color:                " 黑色 ",
		SceneTags:            []string{" 通勤 ", "", " 晚宴 "},
		UserNotes:            " 可配白衬衫 ",
		PrimaryAssetPublicID: " ast_primary ",
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	if item.Name != "黑色西装" {
		t.Fatalf("expected trimmed name, got %q", item.Name)
	}
	if item.Category != "unknown" {
		t.Fatalf("expected default category unknown, got %q", item.Category)
	}
	if item.RecommendationStatus != RecommendationStatusNormal {
		t.Fatalf("expected default recommendation status normal, got %q", item.RecommendationStatus)
	}
	if !item.IsCore {
		t.Fatal("expected create item default is_core true")
	}
	if item.Color != "黑色" {
		t.Fatalf("expected trimmed color, got %q", item.Color)
	}
	if len(item.SceneTags) != 2 || item.SceneTags[0] != "通勤" || item.SceneTags[1] != "晚宴" {
		t.Fatalf("expected trimmed scene tags, got %#v", item.SceneTags)
	}
	if repo.primaryAssetPublicID != "ast_primary" {
		t.Fatalf("expected trimmed primary asset public id, got %q", repo.primaryAssetPublicID)
	}
}

func TestCreateItemReturnsUnsupportedWhenRepoDoesNotSupportItems(t *testing.T) {
	service := NewService(coreOnlyWardrobeRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{Name: "黑色西装"})
	if err != ErrRepositoryUnsupported {
		t.Fatalf("expected ErrRepositoryUnsupported, got %v", err)
	}
}

func TestUpdateItemTrimsNameRecommendationStatusAndSceneTags(t *testing.T) {
	repo := &captureWardrobeRepo{items: []Item{
		{PublicID: "wdi_blazer", UserID: 12, Name: "黑色西装", Category: "outerwear", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true},
	}}
	service := NewService(repo)

	name := " 黑色短西装 "
	recommendationStatus := " preferred "
	sceneTags := []string{" 通勤 ", "", " 约会 "}
	_, err := service.UpdateItem(context.Background(), 12, " wdi_blazer ", UpdateInput{
		Name:                 &name,
		RecommendationStatus: &recommendationStatus,
		SceneTags:            &sceneTags,
	})
	if err != nil {
		t.Fatalf("update item: %v", err)
	}

	if repo.lastUpdatePublicID != "wdi_blazer" {
		t.Fatalf("expected trimmed public id, got %q", repo.lastUpdatePublicID)
	}
	if repo.lastUpdateInput.Name == nil || *repo.lastUpdateInput.Name != "黑色短西装" {
		t.Fatalf("expected trimmed update name, got %#v", repo.lastUpdateInput.Name)
	}
	if repo.lastUpdateInput.RecommendationStatus == nil || *repo.lastUpdateInput.RecommendationStatus != RecommendationStatusPreferred {
		t.Fatalf("expected trimmed recommendation status, got %#v", repo.lastUpdateInput.RecommendationStatus)
	}
	if repo.lastUpdateInput.SceneTags == nil || len(*repo.lastUpdateInput.SceneTags) != 2 || (*repo.lastUpdateInput.SceneTags)[0] != "通勤" || (*repo.lastUpdateInput.SceneTags)[1] != "约会" {
		t.Fatalf("expected trimmed scene tags, got %#v", repo.lastUpdateInput.SceneTags)
	}
}

func TestUpdateItemRejectsEmptyNameWithSentinelError(t *testing.T) {
	service := NewService(&captureWardrobeRepo{items: []Item{
		{PublicID: "wdi_blazer", UserID: 12, Name: "黑色西装", Category: "outerwear", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true},
	}})

	name := "   "
	_, err := service.UpdateItem(context.Background(), 12, "wdi_blazer", UpdateInput{Name: &name})
	if err != ErrInvalidItemName {
		t.Fatalf("expected ErrInvalidItemName, got %v", err)
	}
}

func TestAdviceContextExcludesPausedAndSortsPreferredFirst(t *testing.T) {
	now := time.Now()
	repo := &captureWardrobeRepo{items: []Item{
		{PublicID: "wdi_normal", UserID: 12, Name: "蓝色牛仔裤", Category: "bottom", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true, UpdatedAt: now.Add(-time.Hour)},
		{PublicID: "wdi_paused", UserID: 12, Name: "红色长裙", Category: "dress", RecommendationStatus: RecommendationStatusPaused, Status: StatusActive, IsCore: true, UpdatedAt: now},
		{PublicID: "wdi_inactive", UserID: 12, Name: "旧外套", Category: "outerwear", RecommendationStatus: RecommendationStatusPreferred, Status: "inactive", IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(2 * time.Hour)},
		{PublicID: "wdi_deleted", UserID: 12, Name: "灰色短外套", Category: "outerwear", RecommendationStatus: RecommendationStatusPreferred, Status: StatusDeleted, IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(time.Hour)},
		{PublicID: "wdi_preferred", UserID: 12, Name: "米白衬衫", Category: "top", RecommendationStatus: RecommendationStatusPreferred, Status: StatusActive, IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(-2 * time.Hour)},
	}}
	service := NewService(repo)

	items, err := service.AdviceContextItems(context.Background(), 12, AdviceContextFilter{Scene: "通勤", Limit: 10})
	if err != nil {
		t.Fatalf("advice context: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected paused and inactive items excluded, got %#v", items)
	}
	for _, item := range items {
		if item.PublicID == "wdi_deleted" || item.PublicID == "wdi_inactive" {
			t.Fatalf("expected inactive item excluded, got %#v", items)
		}
	}
	if items[0].PublicID != "wdi_preferred" {
		t.Fatalf("expected preferred scene match first, got %#v", items)
	}
}

func TestAdviceContextAppliesLimitAndSortsCoreBeforeScene(t *testing.T) {
	now := time.Now()
	repo := &captureWardrobeRepo{items: []Item{
		{PublicID: "wdi_noncore_scene", UserID: 12, Name: "浅色围巾", Category: "accessory", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: false, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(2 * time.Hour)},
		{PublicID: "wdi_core_no_scene", UserID: 12, Name: "直筒牛仔裤", Category: "bottom", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true, UpdatedAt: now},
		{PublicID: "wdi_other", UserID: 12, Name: "黑色乐福鞋", Category: "shoes", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: false, UpdatedAt: now.Add(time.Hour)},
	}}
	service := NewService(repo)

	items, err := service.AdviceContextItems(context.Background(), 12, AdviceContextFilter{Scene: "通勤", Limit: 2})
	if err != nil {
		t.Fatalf("advice context: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected limit applied, got %#v", items)
	}
	if items[0].PublicID != "wdi_core_no_scene" {
		t.Fatalf("expected core item before scene-only item, got %#v", items)
	}
	if items[1].PublicID != "wdi_noncore_scene" {
		t.Fatalf("expected scene item before other normal item, got %#v", items)
	}
}
