package asset

import (
	"context"
	"testing"
)

type captureRepo struct {
	items []Asset
}

func (r *captureRepo) CreateMany(_ context.Context, items []Asset) ([]Asset, error) {
	r.items = append(r.items, items...)
	return items, nil
}

func TestRegisterOnboardingAssetsPersistsReferenceMetadata(t *testing.T) {
	repo := &captureRepo{}
	_, err := NewService(repo).RegisterOnboardingAssets(context.Background(), 12, []Input{
		{
			AssetPublicID: "ast_existing",
			ClientRef:     "tmp-1",
			AssetType:     "selfie",
			Note:          "自然光自拍",
		},
	})
	if err != nil {
		t.Fatalf("register assets: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected one asset, got %#v", repo.items)
	}

	metadata := repo.items[0].Metadata
	if metadata["client_ref"] != "tmp-1" {
		t.Fatalf("expected client_ref metadata, got %#v", metadata)
	}
	if metadata["note"] != "自然光自拍" {
		t.Fatalf("expected note metadata, got %#v", metadata)
	}
	if metadata["original_asset_public_id"] != "ast_existing" {
		t.Fatalf("expected original asset public id metadata, got %#v", metadata)
	}
}
