package asset

import (
	"context"
	"strings"

	"hestia/server/internal/common/id"
)

const (
	DefaultBucket       = "local-onboarding"
	SourceOnboarding    = "onboarding"
	StatusActive        = "active"
	ReviewStatusPending = "pending"
)

type Repository interface {
	CreateMany(ctx context.Context, items []Asset) ([]Asset, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RegisterOnboardingAssets(ctx context.Context, userID int64, inputs []Input) ([]Asset, error) {
	if len(inputs) == 0 {
		return []Asset{}, nil
	}
	items := make([]Asset, 0, len(inputs))
	for _, input := range inputs {
		objectKey := strings.TrimSpace(input.ObjectKey)
		if objectKey == "" {
			continue
		}
		mimeType := strings.TrimSpace(input.MimeType)
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		assetType := strings.TrimSpace(input.AssetType)
		if assetType == "" {
			assetType = "onboarding_photo"
		}
		items = append(items, Asset{
			PublicID:     id.NewPublicID("ast"),
			OwnerUserID:  userID,
			Bucket:       DefaultBucket,
			ObjectKey:    objectKey,
			MimeType:     mimeType,
			FileSize:     input.FileSize,
			Width:        input.Width,
			Height:       input.Height,
			AssetType:    assetType,
			Source:       SourceOnboarding,
			Status:       StatusActive,
			ReviewStatus: ReviewStatusPending,
		})
	}
	if len(items) == 0 {
		return []Asset{}, nil
	}
	return s.repo.CreateMany(ctx, items)
}
