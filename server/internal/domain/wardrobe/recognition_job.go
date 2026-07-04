package wardrobe

import (
	"context"

	"hestia/server/internal/domain/job"
)

type JobRecognitionCreator struct {
	service  *job.Service
	wardrobe *Service
}

func NewJobRecognitionCreator(service *job.Service) *JobRecognitionCreator {
	return &JobRecognitionCreator{service: service}
}

func NewAsyncJobRecognitionCreator(service *job.Service, wardrobe *Service) *JobRecognitionCreator {
	return &JobRecognitionCreator{service: service, wardrobe: wardrobe}
}

func (c *JobRecognitionCreator) CreateWardrobeRecognitionJob(ctx context.Context, userID int64, wardrobeItemID int64, wardrobeItemPublicID string, assetPublicIDs []string, overwrite bool) (string, error) {
	if c == nil || c.service == nil {
		return "", nil
	}
	item, err := c.service.CreateWardrobeRecognitionJob(ctx, userID, wardrobeItemID, wardrobeItemPublicID, assetPublicIDs, overwrite)
	if err != nil {
		return "", err
	}
	if c.wardrobe != nil {
		go c.run(item, assetPublicIDs, overwrite)
	}
	return item.PublicID, nil
}

func (c *JobRecognitionCreator) run(item job.Job, assetPublicIDs []string, overwrite bool) {
	ctx := context.Background()
	outputItem, err := c.wardrobe.RecognizeAndApplyItemImage(ctx, item.UserID, publicIDFromJobInput(item), assetPublicIDs, overwrite)
	if err != nil {
		_ = c.service.MarkFailed(ctx, item, err.Error())
		return
	}
	_ = c.service.MarkSucceeded(ctx, item, map[string]any{
		"wardrobe_item_public_id": outputItem.PublicID,
		"recognition_status":      outputItem.RecognitionStatus,
	})
}

func publicIDFromJobInput(item job.Job) string {
	if item.InputSummary == nil {
		return ""
	}
	value, _ := item.InputSummary["wardrobe_item_public_id"].(string)
	return value
}
