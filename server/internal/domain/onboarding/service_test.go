package onboarding

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestHashDraftDataPreservesLargeNumberPrecision(t *testing.T) {
	first, err := hashDraftData(DraftData{
		"basic": json.RawMessage(`{"exact_id":9007199254740992}`),
	})
	if err != nil {
		t.Fatalf("hash first draft: %v", err)
	}
	second, err := hashDraftData(DraftData{
		"basic": json.RawMessage(`{"exact_id":9007199254740993}`),
	})
	if err != nil {
		t.Fatalf("hash second draft: %v", err)
	}

	if first == second {
		t.Fatalf("expected different hashes for different large integers, got %q", first)
	}
}

func TestHashDraftDataCanonicalizesObjectKeyOrder(t *testing.T) {
	first, err := hashDraftData(DraftData{
		"style_goal": json.RawMessage(`{"goals":["干净"],"note":"松弛感"}`),
		"basic":      json.RawMessage(`{"height_cm":168}`),
	})
	if err != nil {
		t.Fatalf("hash first draft: %v", err)
	}
	second, err := hashDraftData(DraftData{
		"basic":      json.RawMessage(`{"height_cm":168}`),
		"style_goal": json.RawMessage(`{"note":"松弛感","goals":["干净"]}`),
	})
	if err != nil {
		t.Fatalf("hash second draft: %v", err)
	}

	if first != second {
		t.Fatalf("expected same hash for same data with different key order, got %q and %q", first, second)
	}
}

func TestSaveDraftDoesNotOverwriteDraftClaimedAfterRead(t *testing.T) {
	initialHash, err := hashDraftData(DraftData{
		"basic": json.RawMessage(`{"height_cm":168}`),
	})
	if err != nil {
		t.Fatalf("hash draft: %v", err)
	}
	repo := &staleUpdateRepo{
		draft: Draft{
			ID:          1,
			PublicID:    "odf_test",
			UserID:      12,
			Status:      DraftStatusDraft,
			CurrentStep: "basic",
			DraftData: DraftData{
				"basic": json.RawMessage(`{"height_cm":168}`),
			},
			ContentHash: initialHash,
			Version:     1,
		},
	}

	_, err = NewService(repo).SaveDraft(context.Background(), 12, SaveDraftInput{
		Step: "style_goal",
		Data: DraftData{
			"style_goal": json.RawMessage(`{"goals":["干净利落"]}`),
		},
	})

	if !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("expected stale update to fail with ErrDraftNotFound, got %v", err)
	}
	if repo.draft.Status != DraftStatusSubmitted {
		t.Fatalf("expected draft to remain submitted, got %q", repo.draft.Status)
	}
	if _, ok := repo.draft.DraftData["style_goal"]; ok {
		t.Fatalf("stale save overwrote submitted draft: %#v", repo.draft.DraftData)
	}
}

type staleUpdateRepo struct {
	draft Draft
}

func (r *staleUpdateRepo) FindActiveByUserID(context.Context, int64) (Draft, error) {
	draft := r.draft
	r.draft.Status = DraftStatusSubmitted
	return draft, nil
}

func (r *staleUpdateRepo) Create(context.Context, Draft) (Draft, error) {
	return Draft{}, errors.New("unexpected create")
}

func (r *staleUpdateRepo) Update(_ context.Context, draft Draft, expectedVersion int, expectedContentHash string) (Draft, error) {
	if r.draft.Status != DraftStatusDraft || r.draft.Version != expectedVersion || r.draft.ContentHash != expectedContentHash {
		return Draft{}, ErrDraftNotFound
	}
	r.draft = draft
	return draft, nil
}

func (r *staleUpdateRepo) ClaimDraft(context.Context, int64, int64, string) error {
	return errors.New("unexpected claim")
}

func (r *staleUpdateRepo) MarkSubmitted(context.Context, int64, int64) error {
	return errors.New("unexpected mark submitted")
}
