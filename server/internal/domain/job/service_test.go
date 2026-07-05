package job

import (
	"context"
	"testing"
)

type captureJobRepo struct {
	created Job
}

func (r *captureJobRepo) Create(_ context.Context, item Job) (Job, error) {
	item.ID = 7
	r.created = item
	return item, nil
}

func (r *captureJobRepo) UpdateStatus(context.Context, int64, string, map[string]any, string) error {
	return nil
}

func (r *captureJobRepo) FindByPublicIDForUser(context.Context, int64, string) (Job, error) {
	return Job{}, ErrJobNotFound
}

func TestCreateWardrobeRecognitionJob(t *testing.T) {
	repo := &captureJobRepo{}
	service := NewService(repo)

	job, err := service.CreateWardrobeRecognitionJob(context.Background(), 12, 34, "wdi_pending", []string{"ast_primary", "ast_side"}, false)
	if err != nil {
		t.Fatalf("create wardrobe recognition job: %v", err)
	}

	if job.ID != 7 || job.JobType != TypeWardrobeItemImageRecognition || job.Status != StatusPending || job.QueueName != QueueInline {
		t.Fatalf("unexpected job fields: %#v", job)
	}
	if job.RelatedType != "wardrobe_item" || job.RelatedID == nil || *job.RelatedID != 34 || job.UserID != 12 {
		t.Fatalf("unexpected relation fields: %#v", job)
	}
	if repo.created.InputSummary["wardrobe_item_public_id"].(string) != "wdi_pending" || repo.created.InputSummary["asset_public_ids"].([]string)[0] != "ast_primary" || repo.created.InputSummary["overwrite"].(bool) {
		t.Fatalf("unexpected input summary: %#v", repo.created.InputSummary)
	}
}

func TestCreateClothesRecognitionJob(t *testing.T) {
	repo := &captureJobRepo{}
	service := NewService(repo)

	job, err := service.CreateClothesRecognitionJob(context.Background(), 12, 34, "wdi_pending", []string{"ast_primary", "ast_side"}, false)
	if err != nil {
		t.Fatalf("create clothes recognition job: %v", err)
	}

	if job.ID != 7 || job.JobType != TypeClothesItemImageRecognition || job.Status != StatusPending || job.QueueName != QueueInline {
		t.Fatalf("unexpected job fields: %#v", job)
	}
	if job.RelatedType != "clothes" || job.RelatedID == nil || *job.RelatedID != 34 || job.UserID != 12 {
		t.Fatalf("unexpected relation fields: %#v", job)
	}
	if repo.created.InputSummary["clothes_public_id"].(string) != "wdi_pending" || repo.created.InputSummary["asset_public_ids"].([]string)[0] != "ast_primary" || repo.created.InputSummary["overwrite"].(bool) {
		t.Fatalf("unexpected input summary: %#v", repo.created.InputSummary)
	}
}
