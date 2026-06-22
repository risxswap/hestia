package wardrobe

import (
	"context"
	"strings"

	"hestia/server/internal/common/id"
)

const StatusActive = "active"

type Repository interface {
	CreateCoreItems(ctx context.Context, items []Item) ([]Item, error)
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
			PublicID:   id.NewPublicID("wdi"),
			UserID:     userID,
			Name:       name,
			Category:   category,
			Color:      input.Color,
			Silhouette: input.Silhouette,
			Material:   input.Material,
			Season:     input.Season,
			UserNotes:  input.Notes,
			IsCore:     true,
			Status:     StatusActive,
		})
	}
	if len(items) == 0 {
		return []Item{}, nil
	}
	return s.repo.CreateCoreItems(ctx, items)
}
