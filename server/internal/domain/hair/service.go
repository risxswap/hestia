package hair

import (
	"context"
	"errors"
	"strings"
	"time"

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
	ErrInvalidRecommendationStatus = errors.New("invalid hair recommendation status")
	ErrInvalidItemName             = errors.New("invalid hair name")
	ErrInvalidPrimaryAsset         = errors.New("invalid hair primary asset")
	ErrItemNotFound                = errors.New("hair item not found")
	ErrRepositoryUnsupported       = errors.New("hair repository unsupported")
)

type Repository interface {
	ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error)
	FindItemForUser(ctx context.Context, userID int64, publicID string) (Item, error)
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

func (s *Service) ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error) {
	if s == nil || s.repo == nil {
		return nil, ErrRepositoryUnsupported
	}
	return s.repo.ListItems(ctx, userID, filter)
}

func (s *Service) GetItem(ctx context.Context, userID int64, publicID string) (Item, error) {
	if s == nil || s.repo == nil {
		return Item{}, ErrRepositoryUnsupported
	}
	return s.repo.FindItemForUser(ctx, userID, strings.TrimSpace(publicID))
}

func (s *Service) CreateItem(ctx context.Context, userID int64, input CreateInput) (Item, error) {
	if s == nil || s.repo == nil {
		return Item{}, ErrRepositoryUnsupported
	}
	name := strings.TrimSpace(input.Name)
	if name == "" && strings.TrimSpace(input.PrimaryAssetPublicID) == "" {
		return Item{}, ErrInvalidItemName
	}
	if name == "" {
		name = "未命名发型"
	}
	recommendationStatus := strings.TrimSpace(input.RecommendationStatus)
	if recommendationStatus == "" {
		recommendationStatus = RecommendationStatusNormal
	}
	if !validRecommendationStatus(recommendationStatus) {
		return Item{}, ErrInvalidRecommendationStatus
	}
	now := time.Now().UTC()
	item := Item{
		PublicID:             id.NewPublicID("hai"),
		UserID:               userID,
		Name:                 name,
		Length:               strings.TrimSpace(input.Length),
		Shape:                strings.TrimSpace(input.Shape),
		Bangs:                strings.TrimSpace(input.Bangs),
		Color:                strings.TrimSpace(input.Color),
		CareTime:             strings.TrimSpace(input.CareTime),
		SceneTags:            trimStrings(input.SceneTags),
		SuitabilityNotes:     strings.TrimSpace(input.SuitabilityNotes),
		AvoidanceNotes:       strings.TrimSpace(input.AvoidanceNotes),
		UserNotes:            strings.TrimSpace(input.UserNotes),
		RecommendationStatus: recommendationStatus,
		Status:               StatusActive,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	return s.repo.CreateItem(ctx, item, strings.TrimSpace(input.PrimaryAssetPublicID))
}

func (s *Service) UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	if s == nil || s.repo == nil {
		return Item{}, ErrRepositoryUnsupported
	}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" {
			return Item{}, ErrInvalidItemName
		}
		input.Name = &value
	}
	if input.RecommendationStatus != nil {
		value := strings.TrimSpace(*input.RecommendationStatus)
		if value == "" || !validRecommendationStatus(value) {
			return Item{}, ErrInvalidRecommendationStatus
		}
		input.RecommendationStatus = &value
	}
	trimStringPtr(input.Length)
	trimStringPtr(input.Shape)
	trimStringPtr(input.Bangs)
	trimStringPtr(input.Color)
	trimStringPtr(input.CareTime)
	trimStringPtr(input.SuitabilityNotes)
	trimStringPtr(input.AvoidanceNotes)
	trimStringPtr(input.UserNotes)
	trimStringPtr(input.PrimaryAssetPublicID)
	if input.SceneTags != nil {
		value := trimStrings(*input.SceneTags)
		input.SceneTags = &value
	}
	return s.repo.UpdateItem(ctx, userID, strings.TrimSpace(publicID), input)
}

func (s *Service) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	if s == nil || s.repo == nil {
		return ErrRepositoryUnsupported
	}
	return s.repo.SoftDeleteItem(ctx, userID, strings.TrimSpace(publicID))
}

func validRecommendationStatus(value string) bool {
	switch value {
	case RecommendationStatusPreferred, RecommendationStatusNormal, RecommendationStatusPaused:
		return true
	default:
		return false
	}
}

func trimStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
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
