package onboarding

import (
	"encoding/json"
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
