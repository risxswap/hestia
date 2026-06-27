package wardrobe

import (
	"context"
	"testing"
	"time"
)

type captureWardrobeRepo struct {
	created []Item
	items   []Item
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
	return item, nil
}

func (r *captureWardrobeRepo) UpdateItem(_ context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != StatusDeleted {
			if input.RecommendationStatus != nil {
				item.RecommendationStatus = *input.RecommendationStatus
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

func TestCreateItemReturnsUnsupportedWhenRepoDoesNotSupportItems(t *testing.T) {
	service := NewService(coreOnlyWardrobeRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{Name: "黑色西装"})
	if err != ErrRepositoryUnsupported {
		t.Fatalf("expected ErrRepositoryUnsupported, got %v", err)
	}
}

func TestAdviceContextExcludesPausedAndSortsPreferredFirst(t *testing.T) {
	now := time.Now()
	repo := &captureWardrobeRepo{items: []Item{
		{PublicID: "wdi_normal", UserID: 12, Name: "蓝色牛仔裤", Category: "bottom", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true, UpdatedAt: now.Add(-time.Hour)},
		{PublicID: "wdi_paused", UserID: 12, Name: "红色长裙", Category: "dress", RecommendationStatus: RecommendationStatusPaused, Status: StatusActive, IsCore: true, UpdatedAt: now},
		{PublicID: "wdi_deleted", UserID: 12, Name: "灰色短外套", Category: "outerwear", RecommendationStatus: RecommendationStatusPreferred, Status: StatusDeleted, IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(time.Hour)},
		{PublicID: "wdi_preferred", UserID: 12, Name: "米白衬衫", Category: "top", RecommendationStatus: RecommendationStatusPreferred, Status: StatusActive, IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(-2 * time.Hour)},
	}}
	service := NewService(repo)

	items, err := service.AdviceContextItems(context.Background(), 12, AdviceContextFilter{Scene: "通勤", Limit: 10})
	if err != nil {
		t.Fatalf("advice context: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected paused and deleted items excluded, got %#v", items)
	}
	for _, item := range items {
		if item.PublicID == "wdi_deleted" {
			t.Fatalf("expected deleted item excluded, got %#v", items)
		}
	}
	if items[0].PublicID != "wdi_preferred" {
		t.Fatalf("expected preferred scene match first, got %#v", items)
	}
}
