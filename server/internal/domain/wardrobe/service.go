package wardrobe

import (
	"context"
	"errors"
	"sort"
	"strings"

	"hestia/server/internal/common/id"
)

const (
	StatusActive  = "active"
	StatusDeleted = "deleted"

	RecommendationStatusPreferred = "preferred"
	RecommendationStatusNormal    = "normal"
	RecommendationStatusPaused    = "paused"
)

var (
	ErrInvalidRecommendationStatus = errors.New("invalid recommendation status")
	ErrInvalidItemName             = errors.New("invalid wardrobe item name")
	ErrItemNotFound                = errors.New("wardrobe item not found")
	ErrRepositoryUnsupported       = errors.New("wardrobe repository unsupported")
)

type Repository interface {
	CreateCoreItems(ctx context.Context, items []Item) ([]Item, error)
}

type itemRepository interface {
	ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error)
	CreateItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error)
	UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error)
	SoftDeleteItem(ctx context.Context, userID int64, publicID string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateCoreItems(ctx context.Context, userID int64, inputs []Input) ([]Item, error) {
	if len(inputs) == 0 {
		return []Item{}, nil
	}
	items := make([]Item, 0, len(inputs))
	for _, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			continue
		}
		category := strings.TrimSpace(input.Category)
		if category == "" {
			category = "unknown"
		}
		items = append(items, Item{
			PublicID:             id.NewPublicID("wdi"),
			UserID:               userID,
			Name:                 name,
			Category:             category,
			Color:                input.Color,
			Silhouette:           input.Silhouette,
			Material:             input.Material,
			Season:               input.Season,
			UserNotes:            input.Notes,
			IsCore:               true,
			RecommendationStatus: RecommendationStatusNormal,
			Status:               StatusActive,
		})
	}
	if len(items) == 0 {
		return []Item{}, nil
	}
	return s.repo.CreateCoreItems(ctx, items)
}

func (s *Service) ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error) {
	filter.Category = strings.TrimSpace(filter.Category)
	filter.RecommendationStatus = strings.TrimSpace(filter.RecommendationStatus)
	if filter.RecommendationStatus != "" && !isValidRecommendationStatus(filter.RecommendationStatus) {
		return nil, ErrInvalidRecommendationStatus
	}
	repo, err := s.itemRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListItems(ctx, userID, filter)
}

func (s *Service) CreateItem(ctx context.Context, userID int64, input CreateInput) (Item, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Item{}, ErrInvalidItemName
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = "unknown"
	}
	recommendationStatus := strings.TrimSpace(input.RecommendationStatus)
	if recommendationStatus == "" {
		recommendationStatus = RecommendationStatusNormal
	}
	if !isValidRecommendationStatus(recommendationStatus) {
		return Item{}, ErrInvalidRecommendationStatus
	}
	repo, err := s.itemRepo()
	if err != nil {
		return Item{}, err
	}
	isCore := true
	if input.IsCore != nil {
		isCore = *input.IsCore
	}
	item := Item{
		PublicID:             id.NewPublicID("wdi"),
		UserID:               userID,
		Name:                 name,
		Category:             category,
		Color:                strings.TrimSpace(input.Color),
		Silhouette:           strings.TrimSpace(input.Silhouette),
		Material:             strings.TrimSpace(input.Material),
		Season:               strings.TrimSpace(input.Season),
		SceneTags:            trimStringSlice(input.SceneTags),
		UserNotes:            strings.TrimSpace(input.UserNotes),
		IsCore:               isCore,
		RecommendationStatus: recommendationStatus,
		Status:               StatusActive,
	}
	return repo.CreateItem(ctx, item, strings.TrimSpace(input.PrimaryAssetPublicID))
}

func (s *Service) UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Item{}, ErrInvalidItemName
		}
		input.Name = &name
	}
	if input.RecommendationStatus != nil {
		status := strings.TrimSpace(*input.RecommendationStatus)
		if status == "" || !isValidRecommendationStatus(status) {
			return Item{}, ErrInvalidRecommendationStatus
		}
		input.RecommendationStatus = &status
	}
	if input.SceneTags != nil {
		sceneTags := trimStringSlice(*input.SceneTags)
		input.SceneTags = &sceneTags
	}
	trimStringPtr(input.Category)
	trimStringPtr(input.Color)
	trimStringPtr(input.Silhouette)
	trimStringPtr(input.Material)
	trimStringPtr(input.Season)
	trimStringPtr(input.UserNotes)
	trimStringPtr(input.PrimaryAssetPublicID)
	repo, err := s.itemRepo()
	if err != nil {
		return Item{}, err
	}
	return repo.UpdateItem(ctx, userID, strings.TrimSpace(publicID), input)
}

func (s *Service) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	repo, err := s.itemRepo()
	if err != nil {
		return err
	}
	return repo.SoftDeleteItem(ctx, userID, strings.TrimSpace(publicID))
}

func (s *Service) AdviceContextItems(ctx context.Context, userID int64, filter AdviceContextFilter) ([]Item, error) {
	repo, err := s.itemRepo()
	if err != nil {
		return nil, err
	}
	items, err := repo.ListItems(ctx, userID, ListFilter{})
	if err != nil {
		return nil, err
	}
	scene := strings.TrimSpace(filter.Scene)
	result := make([]Item, 0, len(items))
	for _, item := range items {
		if item.Status != StatusActive || item.RecommendationStatus == RecommendationStatusPaused {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := adviceRank(result[i], scene)
		right := adviceRank(result[j], scene)
		if left != right {
			return left > right
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

func (s *Service) itemRepo() (itemRepository, error) {
	repo, ok := s.repo.(itemRepository)
	if !ok {
		return nil, ErrRepositoryUnsupported
	}
	return repo, nil
}

func isValidRecommendationStatus(status string) bool {
	switch status {
	case RecommendationStatusPreferred, RecommendationStatusNormal, RecommendationStatusPaused:
		return true
	default:
		return false
	}
}

func adviceRank(item Item, scene string) int {
	rank := 0
	if item.RecommendationStatus == RecommendationStatusPreferred {
		rank += 100
	}
	if item.IsCore {
		rank += 20
	}
	if scene != "" && hasSceneTag(item.SceneTags, scene) {
		rank += 10
	}
	return rank
}

func hasSceneTag(tags []string, scene string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), scene) {
			return true
		}
	}
	return false
}

func trimStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func trimStringPtr(value *string) {
	if value == nil {
		return
	}
	trimmed := strings.TrimSpace(*value)
	*value = trimmed
}
