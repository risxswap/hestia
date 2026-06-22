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
		assetPublicID := strings.TrimSpace(input.AssetPublicID)
		clientRef := strings.TrimSpace(input.ClientRef)
		if objectKey == "" && assetPublicID == "" && clientRef == "" {
			continue
		}
		if objectKey == "" {
			objectKey = simulatedObjectKey(assetPublicID, clientRef)
		}
		publicID := assetPublicID
		if publicID == "" {
			publicID = id.NewPublicID("ast")
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
			PublicID:     publicID,
			ClientRef:    clientRef,
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
			Note:         strings.TrimSpace(input.Note),
			Metadata: map[string]any{
				"client_ref":                clientRef,
				"note":                      strings.TrimSpace(input.Note),
				"original_asset_public_id":  assetPublicID,
				"simulated_local_asset":     input.ObjectKey == "",
				"onboarding_source_version": 1,
			},
		})
	}
	if len(items) == 0 {
		return []Asset{}, nil
	}
	return s.repo.CreateMany(ctx, items)
}

func simulatedObjectKey(assetPublicID string, clientRef string) string {
	if assetPublicID != "" {
		return "simulated/asset-public-id/" + safeObjectKeyPart(assetPublicID)
	}
	return "simulated/client-ref/" + safeObjectKeyPart(clientRef)
}

func safeObjectKeyPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return value
}
