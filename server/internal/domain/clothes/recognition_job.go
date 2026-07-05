package clothes

import (
	"context"

	"hestia/server/internal/domain/job"
)

type JobRecognitionCreator struct {
	service *job.Service
	clothes *Service
}

func NewJobRecognitionCreator(service *job.Service) *JobRecognitionCreator {
	return &JobRecognitionCreator{service: service}
}

func NewAsyncJobRecognitionCreator(service *job.Service, clothes *Service) *JobRecognitionCreator {
	return &JobRecognitionCreator{service: service, clothes: clothes}
}

func (c *JobRecognitionCreator) CreateClothesRecognitionJob(ctx context.Context, userID int64, clothesID int64, clothesPublicID string, assetPublicIDs []string, overwrite bool) (string, error) {
	if c == nil || c.service == nil {
		return "", nil
	}
	item, err := c.service.CreateClothesRecognitionJob(ctx, userID, clothesID, clothesPublicID, assetPublicIDs, overwrite)
	if err != nil {
		return "", err
	}
	if c.clothes != nil {
		go c.run(item, assetPublicIDs, overwrite)
	}
	return item.PublicID, nil
}

func (c *JobRecognitionCreator) run(item job.Job, assetPublicIDs []string, overwrite bool) {
	ctx := context.Background()
	outputItem, err := c.clothes.RecognizeAndApplyItemImage(ctx, item.UserID, publicIDFromJobInput(item), assetPublicIDs, overwrite)
	if err != nil {
		_ = c.service.MarkFailed(ctx, item, err.Error())
		return
	}
	_ = c.service.MarkSucceeded(ctx, item, map[string]any{
		"clothes_public_id":  outputItem.PublicID,
		"recognition_status": outputItem.RecognitionStatus,
	})
}

func publicIDFromJobInput(item job.Job) string {
	if item.InputSummary == nil {
		return ""
	}
	value, _ := item.InputSummary["clothes_public_id"].(string)
	return value
}
