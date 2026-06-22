package imageroute_test

import (
	"context"
	"testing"
	"time"

	"hestia/server/internal/domain/imageroute"
)

func TestFeedbackServiceRepeatedLikeWritesEvent(t *testing.T) {
	activatedAt := time.Now().UTC().Add(-time.Hour)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_service_active", UserID: 12, Status: imageroute.StatusActive, ActivatedAt: &activatedAt})
	service := imageroute.NewFeedbackService(repo)

	if _, err := service.ApplyFeedback(context.Background(), 12, "irt_service_active", imageroute.FeedbackInput{Action: "like"}); err != nil {
		t.Fatalf("first like failed: %v", err)
	}
	if _, err := service.ApplyFeedback(context.Background(), 12, "irt_service_active", imageroute.FeedbackInput{Action: "like"}); err != nil {
		t.Fatalf("second like failed: %v", err)
	}

	if len(repo.events) != 2 {
		t.Fatalf("expected two events, got %#v", repo.events)
	}
	if repo.routes["irt_service_active"].ActivatedAt == nil || !repo.routes["irt_service_active"].ActivatedAt.Equal(activatedAt) {
		t.Fatalf("expected existing activated_at to be preserved, got %#v", repo.routes["irt_service_active"].ActivatedAt)
	}
}

func TestFeedbackServiceKeepsRouteStatusWhenEventWriteFails(t *testing.T) {
	repo := newMemoryRouteRepo()
	repo.failEventInsert = true
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_service_event_fail", UserID: 12, Status: imageroute.StatusCandidate})
	service := imageroute.NewFeedbackService(repo)

	_, err := service.ApplyFeedback(context.Background(), 12, "irt_service_event_fail", imageroute.FeedbackInput{Action: "like"})

	if err == nil {
		t.Fatal("expected event write failure")
	}
	if repo.routes["irt_service_event_fail"].Status != imageroute.StatusCandidate {
		t.Fatalf("expected route status to roll back, got %#v", repo.routes["irt_service_event_fail"])
	}
	if len(repo.events) != 0 {
		t.Fatalf("expected no event after failed insert, got %#v", repo.events)
	}
}
