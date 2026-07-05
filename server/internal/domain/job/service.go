package job

import (
	"context"
	"errors"
	"time"

	"hestia/server/internal/common/id"
)

var ErrJobNotFound = errors.New("job not found")

type Repository interface {
	Create(ctx context.Context, item Job) (Job, error)
	UpdateStatus(ctx context.Context, id int64, status string, output map[string]any, errorMessage string) error
	FindByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Job, error)
}

type Service struct {
	repo Repository
}

func NewService(repo ...Repository) *Service {
	var selected Repository
	if len(repo) > 0 {
		selected = repo[0]
	}
	return &Service{repo: selected}
}

func (s *Service) CreateInitialReportJob(ctx context.Context, userID int64, input map[string]any) (Job, error) {
	startedAt := time.Now().UTC()
	item := Job{
		PublicID:     id.NewPublicID("job"),
		JobType:      TypeInitialReportGeneration,
		Status:       StatusRunning,
		QueueName:    QueueInline,
		RelatedType:  "onboarding",
		UserID:       userID,
		InputSummary: input,
		StartedAt:    &startedAt,
	}
	return s.repo.Create(ctx, item)
}

func (s *Service) CreateWardrobeRecognitionJob(ctx context.Context, userID int64, wardrobeItemID int64, wardrobeItemPublicID string, assetPublicIDs []string, overwrite bool) (Job, error) {
	relatedID := wardrobeItemID
	item := Job{
		PublicID:    id.NewPublicID("job"),
		JobType:     TypeWardrobeItemImageRecognition,
		Status:      StatusPending,
		QueueName:   QueueInline,
		RelatedType: "wardrobe_item",
		RelatedID:   &relatedID,
		UserID:      userID,
		InputSummary: map[string]any{
			"wardrobe_item_public_id": wardrobeItemPublicID,
			"asset_public_ids":        assetPublicIDs,
			"overwrite":               overwrite,
		},
	}
	return s.repo.Create(ctx, item)
}

func (s *Service) CreateClothesRecognitionJob(ctx context.Context, userID int64, clothesID int64, clothesPublicID string, assetPublicIDs []string, overwrite bool) (Job, error) {
	relatedID := clothesID
	item := Job{
		PublicID:    id.NewPublicID("job"),
		JobType:     TypeClothesItemImageRecognition,
		Status:      StatusPending,
		QueueName:   QueueInline,
		RelatedType: "clothes",
		RelatedID:   &relatedID,
		UserID:      userID,
		InputSummary: map[string]any{
			"clothes_public_id": clothesPublicID,
			"asset_public_ids":  assetPublicIDs,
			"overwrite":         overwrite,
		},
	}
	return s.repo.Create(ctx, item)
}

func (s *Service) MarkSucceeded(ctx context.Context, item Job, output map[string]any) error {
	return s.repo.UpdateStatus(ctx, item.ID, StatusSucceeded, output, "")
}

func (s *Service) MarkFailed(ctx context.Context, item Job, message string) error {
	return s.repo.UpdateStatus(ctx, item.ID, StatusFailed, nil, message)
}

func (s *Service) GetForUser(ctx context.Context, userID int64, publicID string) (JobSummary, error) {
	item, err := s.repo.FindByPublicIDForUser(ctx, userID, publicID)
	if err != nil {
		return JobSummary{}, err
	}
	return summaryFromJob(item), nil
}

func summaryFromJob(item Job) JobSummary {
	return JobSummary{
		PublicID:      item.PublicID,
		Type:          item.JobType,
		Status:        item.Status,
		OutputSummary: item.OutputSummary,
		ErrorMessage:  item.ErrorMessage,
	}
}
