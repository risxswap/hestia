package memory

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrRepositoryUnsupported = errors.New("memory repository unsupported")
	ErrItemNotFound          = errors.New("memory item not found")
	ErrInvalidMemoryValue    = errors.New("invalid memory value")
)

type Repository interface {
	ListItems(ctx context.Context, userID int64) ([]Item, error)
	UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error)
	SoftDeleteItem(ctx context.Context, userID int64, publicID string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListItems(ctx context.Context, userID int64) ([]Item, error) {
	if s == nil || s.repo == nil {
		return nil, ErrRepositoryUnsupported
	}
	items, err := s.repo.ListItems(ctx, userID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index] = DecorateItem(items[index])
	}
	return items, nil
}

func (s *Service) UpdateItem(ctx context.Context, userID int64, publicID string, request UpdateRequest) (Item, error) {
	if s == nil || s.repo == nil {
		return Item{}, ErrRepositoryUnsupported
	}
	input := UpdateInput{
		MemoryValue:    strings.TrimSpace(request.MemoryValue),
		CorrectionNote: strings.TrimSpace(request.CorrectionNote),
	}
	if input.MemoryValue == "" {
		return Item{}, ErrInvalidMemoryValue
	}
	item, err := s.repo.UpdateItem(ctx, userID, strings.TrimSpace(publicID), input)
	if err != nil {
		return Item{}, err
	}
	return DecorateItem(item), nil
}

func (s *Service) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	if s == nil || s.repo == nil {
		return ErrRepositoryUnsupported
	}
	return s.repo.SoftDeleteItem(ctx, userID, strings.TrimSpace(publicID))
}

func DecorateItem(item Item) Item {
	item.TypeLabel = typeLabel(item.MemoryType, item.Polarity)
	if strings.TrimSpace(item.DisplayText) == "" {
		item.DisplayText = displayText(item.MemoryValue)
	}
	if strings.TrimSpace(item.SourceLabel) == "" {
		item.SourceLabel = sourceLabel(item)
	}
	return item
}

func typeLabel(memoryType string, polarity string) string {
	switch strings.TrimSpace(memoryType) {
	case TypeFact:
		return "明确事实"
	case TypePreference:
		if strings.TrimSpace(polarity) == PolarityNegative {
			return "禁忌"
		}
		return "偏好"
	case TypeAvoidance:
		return "禁忌"
	case TypeInference:
		return "AI 推断"
	case TypePending:
		return "待确认"
	default:
		return "其他"
	}
}

func sourceLabel(item Item) string {
	if item.UserCorrectedAt != nil {
		return "用户修正"
	}
	if strings.TrimSpace(item.SourceLabel) != "" {
		return item.SourceLabel
	}
	switch strings.TrimSpace(item.MemoryType) {
	case TypeFact, TypePreference, TypeAvoidance:
		return "用户确认"
	case TypeInference, TypePending:
		return "系统推断"
	default:
		return "长期记忆"
	}
}

func displayText(raw string) string {
	value := strings.TrimSpace(raw)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return strings.Trim(value, `"`)
	}
	return value
}
