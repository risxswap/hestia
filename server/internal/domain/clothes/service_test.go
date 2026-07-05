package clothes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"hestia/server/internal/infra/llm"
)

type recognizeImageFunc func(ctx context.Context, userID int64, input RecognizeImageInput) (RecognizedItemFields, error)

func (f recognizeImageFunc) RecognizeClothesItemImage(ctx context.Context, userID int64, input RecognizeImageInput) (RecognizedItemFields, error) {
	return f(ctx, userID, input)
}

type generateFunc func(ctx context.Context, request llm.Request) (llm.Response, error)

func (f generateFunc) Generate(ctx context.Context, request llm.Request) (llm.Response, error) {
	return f(ctx, request)
}

type captureClothesRepo struct {
	created                  []Item
	items                    []Item
	options                  ClothesOptions
	recognitionAsset         Image
	primaryAssetPublicID     string
	assetPublicIDs           []string
	lastUpdatePublicID       string
	lastUpdateRecommendation *string
	lastUpdateInput          UpdateInput
}

type captureRecognitionJobCreator struct {
	userID            int64
	clothItemID       int64
	clothItemPublicID string
	assetPublicIDs    []string
	overwrite         bool
}

func (c *captureRecognitionJobCreator) CreateClothesRecognitionJob(_ context.Context, userID int64, clothItemID int64, clothItemPublicID string, assetPublicIDs []string, overwrite bool) (string, error) {
	c.userID = userID
	c.clothItemID = clothItemID
	c.clothItemPublicID = clothItemPublicID
	c.assetPublicIDs = append([]string{}, assetPublicIDs...)
	c.overwrite = overwrite
	return "job_recognition", nil
}

type captureImageURLSigner struct {
	objectKey string
	url       string
}

func (s *captureImageURLSigner) PrivateDownloadURL(_ context.Context, objectKey string) (string, error) {
	s.objectKey = objectKey
	if s.url != "" {
		return s.url, nil
	}
	return "https://download.example.test/" + objectKey, nil
}

func TestRecognizeItemImageTrimsAndNormalizesFields(t *testing.T) {
	repo := &captureClothesRepo{
		recognitionAsset: Image{
			AssetPublicID: "ast_primary",
			ObjectKey:     "users/12/clothes/ast_primary.jpg",
		},
	}
	signer := &captureImageURLSigner{url: "https://download.example.test/private.jpg"}
	service := NewService(repo)
	service.SetImageURLSigner(signer)
	var capturedUserID int64
	var capturedInput RecognizeImageInput
	service.SetImageRecognizer(recognizeImageFunc(func(_ context.Context, userID int64, input RecognizeImageInput) (RecognizedItemFields, error) {
		capturedUserID = userID
		capturedInput = input
		return RecognizedItemFields{
			Name:       " 米白针织开衫 ",
			Category:   " 外套 ",
			Color:      " 米白 ",
			Silhouette: " 微宽松 ",
			Material:   " 针织 ",
			Season:     " 春秋 ",
			SceneTags:  []string{" 通勤 ", "", " 周末 "},
			UserNotes:  " 建议内搭简洁上衣 ",
			Confidence: 0.78,
		}, nil
	}))

	result, err := service.RecognizeItemImage(context.Background(), 12, RecognizeImageInput{
		AssetPublicID: " ast_primary ",
	})
	if err != nil {
		t.Fatalf("recognize item image: %v", err)
	}

	if capturedUserID != 12 || capturedInput.AssetPublicID != "ast_primary" || capturedInput.ImageURL != "https://download.example.test/private.jpg" {
		t.Fatalf("expected recognizer to receive trimmed input, user=%d input=%#v", capturedUserID, capturedInput)
	}
	if signer.objectKey != "users/12/clothes/ast_primary.jpg" {
		t.Fatalf("expected signer to receive clothes asset object key, got %q", signer.objectKey)
	}
	if result.Name != "米白针织开衫" ||
		result.Category != "外套" ||
		result.Color != "米白" ||
		result.Silhouette != "微宽松" ||
		result.Material != "针织" ||
		result.Season != "春秋" ||
		result.UserNotes != "建议内搭简洁上衣" {
		t.Fatalf("expected trimmed recognized fields, got %#v", result)
	}
	if len(result.SceneTags) != 2 || result.SceneTags[0] != "通勤" || result.SceneTags[1] != "周末" {
		t.Fatalf("expected normalized scene tags, got %#v", result.SceneTags)
	}
	if result.Confidence != 0.78 {
		t.Fatalf("expected confidence preserved, got %v", result.Confidence)
	}
}

func TestRecognizeAndApplyItemImageUpdatesFieldsAndStatus(t *testing.T) {
	repo := &captureClothesRepo{
		items: []Item{{
			ID:                   34,
			PublicID:             "wdi_pending",
			UserID:               12,
			Name:                 "识别中",
			Category:             "其他",
			RecognitionStatus:    RecognitionStatusPending,
			RecommendationStatus: RecommendationStatusNormal,
			Status:               StatusActive,
			IsCore:               true,
		}},
		recognitionAsset: Image{
			AssetPublicID: "ast_primary",
			ObjectKey:     "users/12/clothes/ast_primary.jpg",
		},
	}
	service := NewService(repo)
	service.SetImageURLSigner(&captureImageURLSigner{url: "https://download.example.test/private.jpg"})
	service.SetImageRecognizer(recognizeImageFunc(func(_ context.Context, _ int64, _ RecognizeImageInput) (RecognizedItemFields, error) {
		return RecognizedItemFields{
			Name:       "米白针织开衫",
			Category:   "外套",
			Color:      "米白",
			Silhouette: "微宽松",
			Material:   "针织",
			Season:     "春秋",
			SceneTags:  []string{"通勤"},
			UserNotes:  "建议内搭简洁上衣",
		}, nil
	}))

	item, err := service.RecognizeAndApplyItemImage(context.Background(), 12, "wdi_pending", []string{"ast_primary"}, false)
	if err != nil {
		t.Fatalf("recognize and apply: %v", err)
	}

	if item.Name != "米白针织开衫" || item.Category != "外套" || item.RecognitionStatus != RecognitionStatusSucceeded {
		t.Fatalf("expected recognized item fields applied, got %#v", item)
	}
	if repo.lastUpdateInput.Name == nil || *repo.lastUpdateInput.Name != "米白针织开衫" {
		t.Fatalf("expected update input with recognized name, got %#v", repo.lastUpdateInput)
	}
	if repo.lastUpdateInput.RecognitionStatus == nil || *repo.lastUpdateInput.RecognitionStatus != RecognitionStatusSucceeded {
		t.Fatalf("expected recognition status succeeded update, got %#v", repo.lastUpdateInput)
	}
}

func TestRecognizeAndApplyItemImageMarksFailedOnRecognitionError(t *testing.T) {
	repo := &captureClothesRepo{
		items: []Item{{
			PublicID:             "wdi_pending",
			UserID:               12,
			Name:                 "识别中",
			Category:             "其他",
			RecognitionStatus:    RecognitionStatusPending,
			RecommendationStatus: RecommendationStatusNormal,
			Status:               StatusActive,
			IsCore:               true,
		}},
		recognitionAsset: Image{
			AssetPublicID: "ast_primary",
			ObjectKey:     "users/12/clothes/ast_primary.jpg",
		},
	}
	service := NewService(repo)
	service.SetImageURLSigner(&captureImageURLSigner{url: "https://download.example.test/private.jpg"})
	service.SetImageRecognizer(recognizeImageFunc(func(context.Context, int64, RecognizeImageInput) (RecognizedItemFields, error) {
		return RecognizedItemFields{}, errors.New("llm failed")
	}))

	_, err := service.RecognizeAndApplyItemImage(context.Background(), 12, "wdi_pending", []string{"ast_primary"}, false)
	if err == nil {
		t.Fatal("expected recognition error")
	}
	if repo.lastUpdateInput.RecognitionStatus == nil || *repo.lastUpdateInput.RecognitionStatus != RecognitionStatusFailed {
		t.Fatalf("expected recognition status failed update, got %#v", repo.lastUpdateInput)
	}
}

func TestRecognizeItemImageRejectsMissingAssetPublicID(t *testing.T) {
	service := NewService(&captureClothesRepo{})

	_, err := service.RecognizeItemImage(context.Background(), 12, RecognizeImageInput{
		AssetPublicID: "   ",
	})

	if err != ErrInvalidPrimaryAsset {
		t.Fatalf("expected ErrInvalidPrimaryAsset, got %v", err)
	}
}

func TestRecognizeItemImageRejectsUnownedOrInvalidClothesAsset(t *testing.T) {
	service := NewService(&captureClothesRepo{})
	service.SetImageURLSigner(&captureImageURLSigner{})
	service.SetImageRecognizer(recognizeImageFunc(func(_ context.Context, _ int64, _ RecognizeImageInput) (RecognizedItemFields, error) {
		t.Fatal("recognizer should not be called for invalid asset")
		return RecognizedItemFields{}, nil
	}))

	_, err := service.RecognizeItemImage(context.Background(), 12, RecognizeImageInput{
		AssetPublicID: "ast_missing",
	})

	if err != ErrInvalidPrimaryAsset {
		t.Fatalf("expected ErrInvalidPrimaryAsset, got %v", err)
	}
}

func TestLLMImageRecognizerParsesJSONFields(t *testing.T) {
	var prompt string
	var request llm.Request
	recognizer := NewLLMImageRecognizer(generateFunc(func(_ context.Context, input llm.Request) (llm.Response, error) {
		request = input
		if len(input.Messages) > 0 {
			prompt = input.Messages[0].Content
		}
		return llm.Response{Text: "```json\n{\"name\":\"米白衬衫\",\"category\":\"上装\",\"scene_tags\":[\"通勤\"],\"confidence\":0.66}\n```"}, nil
	}))

	result, err := recognizer.RecognizeClothesItemImage(context.Background(), 12, RecognizeImageInput{
		AssetPublicID: "ast_primary",
		ImageURL:      "https://example.test/private.jpg",
	})
	if err != nil {
		t.Fatalf("recognize with llm: %v", err)
	}

	if request.UsageKey != "clothes_image_recognition" || len(request.RequiredCaps) != 2 {
		t.Fatalf("expected clothes image usage request, got %#v", request)
	}
	if len(request.ImageURLs) != 1 || request.ImageURLs[0] != "https://example.test/private.jpg" {
		t.Fatalf("expected image url in request, got %#v", request.ImageURLs)
	}
	if !strings.Contains(prompt, "ast_primary") || !strings.Contains(prompt, "不要输出身材") {
		t.Fatalf("expected safe prompt with asset id, got %q", prompt)
	}
	if result.Name != "米白衬衫" || result.Category != "上装" || len(result.SceneTags) != 1 || result.Confidence != 0.66 {
		t.Fatalf("expected parsed recognized fields, got %#v", result)
	}
}

func TestLLMImageRecognizerReturnsUnavailableWithoutGenerator(t *testing.T) {
	recognizer := NewLLMImageRecognizer(nil)

	_, err := recognizer.RecognizeClothesItemImage(context.Background(), 12, RecognizeImageInput{
		AssetPublicID: "ast_primary",
	})

	if !errors.Is(err, ErrImageRecognizerUnavailable) {
		t.Fatalf("expected ErrImageRecognizerUnavailable, got %v", err)
	}
}

type coreOnlyClothesRepo struct{}

func (r coreOnlyClothesRepo) CreateCoreItems(_ context.Context, items []Item) ([]Item, error) {
	return items, nil
}

func (r *captureClothesRepo) CreateCoreItems(_ context.Context, items []Item) ([]Item, error) {
	r.created = append(r.created, items...)
	return items, nil
}

func (r *captureClothesRepo) ListItems(_ context.Context, userID int64, filter ListFilter) ([]Item, error) {
	var result []Item
	for _, item := range r.items {
		if item.UserID != userID {
			continue
		}
		if filter.Category != "" && item.Category != filter.Category {
			continue
		}
		if filter.RecommendationStatus != "" && item.RecommendationStatus != filter.RecommendationStatus {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *captureClothesRepo) FindItemForUser(_ context.Context, userID int64, publicID string) (Item, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != StatusDeleted {
			return item, nil
		}
	}
	return Item{}, ErrItemNotFound
}

func (r *captureClothesRepo) ListClothesOptions(context.Context) (ClothesOptions, error) {
	if len(r.options.Categories) == 0 &&
		len(r.options.Materials) == 0 &&
		len(r.options.Seasons) == 0 &&
		len(r.options.Silhouettes) == 0 {
		return DefaultClothesOptions(), nil
	}
	return r.options, nil
}

func (r *captureClothesRepo) FindRecognizableAsset(_ context.Context, userID int64, assetPublicID string) (Image, error) {
	if r.recognitionAsset.AssetPublicID == assetPublicID &&
		strings.HasPrefix(r.recognitionAsset.ObjectKey, "users/") &&
		strings.Contains(r.recognitionAsset.ObjectKey, "/clothes/") &&
		userID == 12 {
		return r.recognitionAsset, nil
	}
	return Image{}, ErrInvalidPrimaryAsset
}

func (r *captureClothesRepo) CreateItem(_ context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	item.ID = int64(len(r.created) + 1)
	r.created = append(r.created, item)
	r.primaryAssetPublicID = primaryAssetPublicID
	return item, nil
}

func (r *captureClothesRepo) CreateItemWithAssets(_ context.Context, item Item, assetPublicIDs []string) (Item, error) {
	item.ID = int64(len(r.created) + 1)
	r.created = append(r.created, item)
	r.assetPublicIDs = append([]string{}, assetPublicIDs...)
	if len(assetPublicIDs) > 0 {
		r.primaryAssetPublicID = assetPublicIDs[0]
		item.PrimaryImage = &Image{AssetPublicID: assetPublicIDs[0]}
	}
	return item, nil
}

func (r *captureClothesRepo) UpdateItem(_ context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	r.lastUpdatePublicID = publicID
	r.lastUpdateInput = input
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != StatusDeleted {
			if input.RecommendationStatus != nil {
				item.RecommendationStatus = *input.RecommendationStatus
				r.lastUpdateRecommendation = input.RecommendationStatus
			}
			if input.Name != nil {
				item.Name = *input.Name
			}
			if input.Category != nil {
				item.Category = *input.Category
			}
			if input.Color != nil {
				item.Color = *input.Color
			}
			if input.Silhouette != nil {
				item.Silhouette = *input.Silhouette
			}
			if input.Material != nil {
				item.Material = *input.Material
			}
			if input.Season != nil {
				item.Season = *input.Season
			}
			if input.SceneTags != nil {
				item.SceneTags = *input.SceneTags
			}
			if input.UserNotes != nil {
				item.UserNotes = *input.UserNotes
			}
			if input.RecognitionStatus != nil {
				item.RecognitionStatus = *input.RecognitionStatus
			}
			return item, nil
		}
	}
	return Item{}, ErrItemNotFound
}

func (r *captureClothesRepo) ReplaceItemAssets(_ context.Context, userID int64, publicID string, assetPublicIDs []string) (Item, error) {
	r.assetPublicIDs = append([]string{}, assetPublicIDs...)
	if len(assetPublicIDs) > 0 {
		r.primaryAssetPublicID = assetPublicIDs[0]
	}
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != StatusDeleted {
			if len(assetPublicIDs) > 0 {
				item.PrimaryImage = &Image{AssetPublicID: assetPublicIDs[0]}
			} else {
				item.PrimaryImage = nil
			}
			return item, nil
		}
	}
	return Item{}, ErrItemNotFound
}

func (r *captureClothesRepo) SoftDeleteItem(_ context.Context, userID int64, publicID string) error {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID {
			return nil
		}
	}
	return ErrItemNotFound
}

func TestCreateCoreItemsDefaultsRecommendationStatusNormal(t *testing.T) {
	repo := &captureClothesRepo{}
	service := NewService(repo)

	_, err := service.CreateCoreItems(context.Background(), 12, []Input{{Name: " 米白衬衫 ", Category: "上装"}})
	if err != nil {
		t.Fatalf("create core items: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected one created item, got %#v", repo.created)
	}
	if repo.created[0].RecommendationStatus != RecommendationStatusNormal {
		t.Fatalf("expected normal recommendation status, got %q", repo.created[0].RecommendationStatus)
	}
	if !repo.created[0].IsCore {
		t.Fatal("expected onboarding core item to stay core")
	}
}

func TestCreateItemRejectsInvalidRecommendationStatus(t *testing.T) {
	service := NewService(&captureClothesRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{
		Name:                 "黑色西装",
		Category:             "外套",
		RecommendationStatus: "hidden",
	})
	if err == nil {
		t.Fatal("expected invalid recommendation status error")
	}
	if err != ErrInvalidRecommendationStatus {
		t.Fatalf("expected ErrInvalidRecommendationStatus, got %v", err)
	}
}

func TestCreateItemRejectsEmptyNameWithSentinelError(t *testing.T) {
	service := NewService(&captureClothesRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{Name: "   "})
	if err != ErrInvalidItemName {
		t.Fatalf("expected ErrInvalidItemName, got %v", err)
	}
}

func TestCreateItemTrimsAndDefaultsFields(t *testing.T) {
	repo := &captureClothesRepo{}
	service := NewService(repo)

	item, err := service.CreateItem(context.Background(), 12, CreateInput{
		Name:                 " 黑色西装 ",
		Category:             " 上装 ",
		Color:                " 黑色 ",
		SceneTags:            []string{" 通勤 ", "", " 晚宴 "},
		UserNotes:            " 可配白衬衫 ",
		PrimaryAssetPublicID: " ast_primary ",
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	if item.Name != "黑色西装" {
		t.Fatalf("expected trimmed name, got %q", item.Name)
	}
	if item.Category != "上装" {
		t.Fatalf("expected trimmed category 上装, got %q", item.Category)
	}
	if item.RecommendationStatus != RecommendationStatusNormal {
		t.Fatalf("expected default recommendation status normal, got %q", item.RecommendationStatus)
	}
	if !item.IsCore {
		t.Fatal("expected create item default is_core true")
	}
	if item.Color != "黑色" {
		t.Fatalf("expected trimmed color, got %q", item.Color)
	}
	if len(item.SceneTags) != 2 || item.SceneTags[0] != "通勤" || item.SceneTags[1] != "晚宴" {
		t.Fatalf("expected trimmed scene tags, got %#v", item.SceneTags)
	}
	if repo.primaryAssetPublicID != "ast_primary" {
		t.Fatalf("expected trimmed primary asset public id, got %q", repo.primaryAssetPublicID)
	}
}

func TestCreateItemSupportsMultipleUploadedAssetsAndStartsRecognitionPending(t *testing.T) {
	repo := &captureClothesRepo{}
	service := NewService(repo)
	jobCreator := &captureRecognitionJobCreator{}
	service.SetRecognitionJobCreator(jobCreator)

	item, err := service.CreateItem(context.Background(), 12, CreateInput{
		AssetPublicIDs: []string{" ast_primary ", "ast_side", "", " ast_detail "},
	})
	if err != nil {
		t.Fatalf("create item from assets: %v", err)
	}

	if item.Name != "识别中" || item.Category != "其他" {
		t.Fatalf("expected placeholder item while recognizing, got name=%q category=%q", item.Name, item.Category)
	}
	if item.RecognitionStatus != RecognitionStatusPending {
		t.Fatalf("expected pending recognition status, got %q", item.RecognitionStatus)
	}
	if len(repo.assetPublicIDs) != 3 || repo.assetPublicIDs[0] != "ast_primary" || repo.assetPublicIDs[1] != "ast_side" || repo.assetPublicIDs[2] != "ast_detail" {
		t.Fatalf("expected trimmed uploaded assets preserved, got %#v", repo.assetPublicIDs)
	}
	if repo.primaryAssetPublicID != "ast_primary" {
		t.Fatalf("expected first uploaded asset as primary, got %q", repo.primaryAssetPublicID)
	}
	if item.RecognitionJobPublicID != "job_recognition" {
		t.Fatalf("expected recognition job public id, got %q", item.RecognitionJobPublicID)
	}
	if jobCreator.userID != 12 || jobCreator.clothItemID != item.ID || jobCreator.overwrite {
		t.Fatalf("unexpected job creator context: %#v", jobCreator)
	}
}

func TestCreateItemAllowsCustomSuggestedFields(t *testing.T) {
	service := NewService(&captureClothesRepo{
		options: ClothesOptions{
			Categories:  []string{"上装"},
			Materials:   []string{"棉"},
			Seasons:     []string{"春秋"},
			Silhouettes: []string{"微宽松"},
		},
	})

	item, err := service.CreateItem(context.Background(), 12, CreateInput{
		Name:       "黑色西装",
		Category:   "定制分类",
		Color:      "雾霾蓝",
		Material:   "丝绒",
		Season:     "梅雨季",
		Silhouette: "茧型",
	})
	if err != nil {
		t.Fatalf("expected custom suggested fields to pass, got %v", err)
	}
	if item.Category != "定制分类" || item.Material != "丝绒" || item.Season != "梅雨季" || item.Silhouette != "茧型" {
		t.Fatalf("expected custom fields saved, got %#v", item)
	}
}

func TestCreateItemAllowsEmptyOptionalConfiguredOptions(t *testing.T) {
	repo := &captureClothesRepo{
		options: ClothesOptions{
			Categories:  []string{"上装"},
			Materials:   []string{"棉"},
			Seasons:     []string{"春秋"},
			Silhouettes: []string{"微宽松"},
		},
	}
	service := NewService(repo)

	_, err := service.CreateItem(context.Background(), 12, CreateInput{
		Name:     "米白衬衫",
		Category: "上装",
	})
	if err != nil {
		t.Fatalf("expected empty optional options to pass, got %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected item created, got %#v", repo.created)
	}
}

func TestClothesAssetRowEligibilityRequiresClothesUploadScope(t *testing.T) {
	valid := clothAssetRow{
		PublicID:  "ast_abcdefghijklmnopqrstuvwxyz",
		Bucket:    "private-assets",
		ObjectKey: "users/12/clothes/ast_abcdefghijklmnopqrstuvwxyz.jpg",
		AssetType: clothPrimaryAssetType,
		Source:    clothPrimaryAssetSource,
	}
	if !valid.eligibleClothesPrimaryAsset(12) {
		t.Fatalf("expected valid cloth uploaded asset to be eligible")
	}

	tests := []struct {
		name string
		row  clothAssetRow
	}{
		{
			name: "onboarding asset type",
			row: clothAssetRow{
				PublicID:  valid.PublicID,
				Bucket:    valid.Bucket,
				ObjectKey: valid.ObjectKey,
				AssetType: "onboarding_photo",
				Source:    valid.Source,
			},
		},
		{
			name: "onboarding source",
			row: clothAssetRow{
				PublicID:  valid.PublicID,
				Bucket:    valid.Bucket,
				ObjectKey: valid.ObjectKey,
				AssetType: valid.AssetType,
				Source:    "onboarding",
			},
		},
		{
			name: "local bucket",
			row: clothAssetRow{
				PublicID:  valid.PublicID,
				Bucket:    localOnboardingBucket,
				ObjectKey: valid.ObjectKey,
				AssetType: valid.AssetType,
				Source:    valid.Source,
			},
		},
		{
			name: "other user path",
			row: clothAssetRow{
				PublicID:  valid.PublicID,
				Bucket:    valid.Bucket,
				ObjectKey: "users/99/clothes/ast_abcdefghijklmnopqrstuvwxyz.jpg",
				AssetType: valid.AssetType,
				Source:    valid.Source,
			},
		},
		{
			name: "other scope path",
			row: clothAssetRow{
				PublicID:  valid.PublicID,
				Bucket:    valid.Bucket,
				ObjectKey: "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.jpg",
				AssetType: valid.AssetType,
				Source:    valid.Source,
			},
		},
		{
			name: "mismatched filename",
			row: clothAssetRow{
				PublicID:  valid.PublicID,
				Bucket:    valid.Bucket,
				ObjectKey: "users/12/clothes/ast_bcdefghijklmnopqrstuvwxyza.jpg",
				AssetType: valid.AssetType,
				Source:    valid.Source,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.row.eligibleClothesPrimaryAsset(12) {
				t.Fatalf("expected ineligible clothes primary asset, got %#v", tt.row)
			}
		})
	}
}

func TestCreateItemReturnsUnsupportedWhenRepoDoesNotSupportItems(t *testing.T) {
	service := NewService(coreOnlyClothesRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{Name: "黑色西装", Category: "上装"})
	if err != ErrRepositoryUnsupported {
		t.Fatalf("expected ErrRepositoryUnsupported, got %v", err)
	}
}

func TestUpdateItemTrimsNameRecommendationStatusAndSceneTags(t *testing.T) {
	repo := &captureClothesRepo{items: []Item{
		{PublicID: "wdi_blazer", UserID: 12, Name: "黑色西装", Category: "外套", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true},
	}}
	service := NewService(repo)

	name := " 黑色短西装 "
	recommendationStatus := " preferred "
	sceneTags := []string{" 通勤 ", "", " 约会 "}
	_, err := service.UpdateItem(context.Background(), 12, " wdi_blazer ", UpdateInput{
		Name:                 &name,
		RecommendationStatus: &recommendationStatus,
		SceneTags:            &sceneTags,
	})
	if err != nil {
		t.Fatalf("update item: %v", err)
	}

	if repo.lastUpdatePublicID != "wdi_blazer" {
		t.Fatalf("expected trimmed public id, got %q", repo.lastUpdatePublicID)
	}
	if repo.lastUpdateInput.Name == nil || *repo.lastUpdateInput.Name != "黑色短西装" {
		t.Fatalf("expected trimmed update name, got %#v", repo.lastUpdateInput.Name)
	}
	if repo.lastUpdateInput.RecommendationStatus == nil || *repo.lastUpdateInput.RecommendationStatus != RecommendationStatusPreferred {
		t.Fatalf("expected trimmed recommendation status, got %#v", repo.lastUpdateInput.RecommendationStatus)
	}
	if repo.lastUpdateInput.SceneTags == nil || len(*repo.lastUpdateInput.SceneTags) != 2 || (*repo.lastUpdateInput.SceneTags)[0] != "通勤" || (*repo.lastUpdateInput.SceneTags)[1] != "约会" {
		t.Fatalf("expected trimmed scene tags, got %#v", repo.lastUpdateInput.SceneTags)
	}
}

func TestUpdateItemRejectsEmptyNameWithSentinelError(t *testing.T) {
	service := NewService(&captureClothesRepo{items: []Item{
		{PublicID: "wdi_blazer", UserID: 12, Name: "黑色西装", Category: "外套", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true},
	}})

	name := "   "
	_, err := service.UpdateItem(context.Background(), 12, "wdi_blazer", UpdateInput{Name: &name})
	if err != ErrInvalidItemName {
		t.Fatalf("expected ErrInvalidItemName, got %v", err)
	}
}

func TestUpdateItemRejectsWhenRecognitionPending(t *testing.T) {
	service := NewService(&captureClothesRepo{items: []Item{
		{PublicID: "wdi_pending", UserID: 12, Name: "识别中", Category: "其他", RecognitionStatus: RecognitionStatusPending, RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true},
	}})

	name := "米白衬衫"
	_, err := service.UpdateItem(context.Background(), 12, "wdi_pending", UpdateInput{Name: &name})
	if err != ErrRecognitionPending {
		t.Fatalf("expected ErrRecognitionPending, got %v", err)
	}
}

func TestUpdateItemReplacesAssetsAndUsesFirstAsPrimary(t *testing.T) {
	repo := &captureClothesRepo{items: []Item{
		{PublicID: "wdi_blazer", UserID: 12, Name: "黑色西装", Category: "外套", RecognitionStatus: RecognitionStatusSucceeded, RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true},
	}}
	service := NewService(repo)

	assetPublicIDs := []string{" ast_first ", "", "ast_side", " ast_detail "}
	item, err := service.UpdateItem(context.Background(), 12, "wdi_blazer", UpdateInput{
		AssetPublicIDs: &assetPublicIDs,
	})
	if err != nil {
		t.Fatalf("update item assets: %v", err)
	}

	if len(repo.assetPublicIDs) != 3 || repo.assetPublicIDs[0] != "ast_first" || repo.assetPublicIDs[1] != "ast_side" || repo.assetPublicIDs[2] != "ast_detail" {
		t.Fatalf("expected trimmed asset ids preserved, got %#v", repo.assetPublicIDs)
	}
	if repo.primaryAssetPublicID != "ast_first" {
		t.Fatalf("expected first asset as primary, got %q", repo.primaryAssetPublicID)
	}
	if repo.lastUpdateInput.PrimaryAssetPublicID != nil {
		t.Fatalf("expected primary asset field suppressed when full asset list is present, got %#v", repo.lastUpdateInput.PrimaryAssetPublicID)
	}
	if item.PrimaryImage == nil || item.PrimaryImage.AssetPublicID != "ast_first" {
		t.Fatalf("expected returned item primary image from first asset, got %#v", item.PrimaryImage)
	}
}

func TestAdviceContextExcludesPausedAndSortsPreferredFirst(t *testing.T) {
	now := time.Now()
	repo := &captureClothesRepo{items: []Item{
		{PublicID: "wdi_normal", UserID: 12, Name: "蓝色牛仔裤", Category: "下装", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true, UpdatedAt: now.Add(-time.Hour)},
		{PublicID: "wdi_paused", UserID: 12, Name: "红色长裙", Category: "dress", RecommendationStatus: RecommendationStatusPaused, Status: StatusActive, IsCore: true, UpdatedAt: now},
		{PublicID: "wdi_inactive", UserID: 12, Name: "旧外套", Category: "外套", RecommendationStatus: RecommendationStatusPreferred, Status: "inactive", IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(2 * time.Hour)},
		{PublicID: "wdi_deleted", UserID: 12, Name: "灰色短外套", Category: "外套", RecommendationStatus: RecommendationStatusPreferred, Status: StatusDeleted, IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(time.Hour)},
		{PublicID: "wdi_preferred", UserID: 12, Name: "米白衬衫", Category: "上装", RecommendationStatus: RecommendationStatusPreferred, Status: StatusActive, IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(-2 * time.Hour)},
	}}
	service := NewService(repo)

	items, err := service.AdviceContextItems(context.Background(), 12, AdviceContextFilter{Scene: "通勤", Limit: 10})
	if err != nil {
		t.Fatalf("advice context: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected paused and inactive items excluded, got %#v", items)
	}
	for _, item := range items {
		if item.PublicID == "wdi_deleted" || item.PublicID == "wdi_inactive" {
			t.Fatalf("expected inactive item excluded, got %#v", items)
		}
	}
	if items[0].PublicID != "wdi_preferred" {
		t.Fatalf("expected preferred scene match first, got %#v", items)
	}
}

func TestAdviceContextAppliesLimitAndSortsCoreBeforeScene(t *testing.T) {
	now := time.Now()
	repo := &captureClothesRepo{items: []Item{
		{PublicID: "wdi_noncore_scene", UserID: 12, Name: "浅色围巾", Category: "配饰", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: false, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(2 * time.Hour)},
		{PublicID: "wdi_core_no_scene", UserID: 12, Name: "直筒牛仔裤", Category: "下装", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true, UpdatedAt: now},
		{PublicID: "wdi_other", UserID: 12, Name: "黑色乐福鞋", Category: "鞋", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: false, UpdatedAt: now.Add(time.Hour)},
	}}
	service := NewService(repo)

	items, err := service.AdviceContextItems(context.Background(), 12, AdviceContextFilter{Scene: "通勤", Limit: 2})
	if err != nil {
		t.Fatalf("advice context: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected limit applied, got %#v", items)
	}
	if items[0].PublicID != "wdi_core_no_scene" {
		t.Fatalf("expected core item before scene-only item, got %#v", items)
	}
	if items[1].PublicID != "wdi_noncore_scene" {
		t.Fatalf("expected scene item before other normal item, got %#v", items)
	}
}
