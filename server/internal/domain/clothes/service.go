package clothes

import (
	"context"
	"errors"
	"sort"
	"strings"

	"hestia/server/internal/common/id"
)

const (
	StatusActive  = "active"
	StatusDeleted = "deleted"

	RecommendationStatusPreferred = "preferred"
	RecommendationStatusNormal    = "normal"
	RecommendationStatusPaused    = "paused"

	RecognitionStatusPending   = "pending"
	RecognitionStatusSucceeded = "succeeded"
	RecognitionStatusFailed    = "failed"
)

var (
	ErrInvalidRecommendationStatus = errors.New("invalid recommendation status")
	ErrInvalidItemName             = errors.New("invalid clothes item name")
	ErrInvalidPrimaryAsset         = errors.New("invalid clothes primary asset")
	ErrItemNotFound                = errors.New("clothes item not found")
	ErrRepositoryUnsupported       = errors.New("clothes repository unsupported")
	ErrRecognitionPending          = errors.New("clothes item recognition pending")
)

type Repository interface {
	CreateCoreItems(ctx context.Context, items []Item) ([]Item, error)
}

type itemRepository interface {
	ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error)
	FindItemForUser(ctx context.Context, userID int64, publicID string) (Item, error)
	CreateItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error)
	UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error)
	SoftDeleteItem(ctx context.Context, userID int64, publicID string) error
}

type multiAssetItemRepository interface {
	itemRepository
	CreateItemWithAssets(ctx context.Context, item Item, assetPublicIDs []string) (Item, error)
}

type assetReplacingItemRepository interface {
	itemRepository
	ReplaceItemAssets(ctx context.Context, userID int64, publicID string, assetPublicIDs []string) (Item, error)
}

type recognizerAssetRepository interface {
	FindRecognizableAsset(ctx context.Context, userID int64, assetPublicID string) (Image, error)
}

type optionsRepository interface {
	ListClothesOptions(ctx context.Context) (ClothesOptions, error)
}

type ImageURLSigner interface {
	PrivateDownloadURL(ctx context.Context, objectKey string) (string, error)
}

type SignedImageURLs = struct {
	PreviewURL  string
	OriginalURL string
}

type BatchImageURLSigner interface {
	ImageURLSigner
	PrivateImageURLs(ctx context.Context, objectKeys []string) (map[string]SignedImageURLs, error)
}

type ImageRecognizer interface {
	RecognizeClothesItemImage(ctx context.Context, userID int64, input RecognizeImageInput) (RecognizedItemFields, error)
}

type RecognitionJobCreator interface {
	CreateClothesRecognitionJob(ctx context.Context, userID int64, clothItemID int64, clothItemPublicID string, assetPublicIDs []string, overwrite bool) (string, error)
}

type ImageURLSignErrorHandler func(ctx context.Context, objectKey string, err error)

type Service struct {
	repo                     Repository
	imageURLSigner           ImageURLSigner
	imageURLSignErrorHandler ImageURLSignErrorHandler
	imageRecognizer          ImageRecognizer
	recognitionJobCreator    RecognitionJobCreator
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SetImageURLSigner(signer ImageURLSigner) {
	if s == nil {
		return
	}
	s.imageURLSigner = signer
}

func (s *Service) SetImageURLSignErrorHandler(handler ImageURLSignErrorHandler) {
	if s == nil {
		return
	}
	s.imageURLSignErrorHandler = handler
}

func (s *Service) SetImageRecognizer(recognizer ImageRecognizer) {
	if s == nil {
		return
	}
	s.imageRecognizer = recognizer
}

func (s *Service) SetRecognitionJobCreator(creator RecognitionJobCreator) {
	if s == nil {
		return
	}
	s.recognitionJobCreator = creator
}

func (s *Service) CreateCoreItems(ctx context.Context, userID int64, inputs []Input) ([]Item, error) {
	if len(inputs) == 0 {
		return []Item{}, nil
	}
	items := make([]Item, 0, len(inputs))
	for _, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			continue
		}
		category := strings.TrimSpace(input.Category)
		if category == "" {
			category = "其他"
		}
		items = append(items, Item{
			PublicID:             id.NewPublicID("wdi"),
			UserID:               userID,
			Name:                 name,
			Category:             category,
			Color:                input.Color,
			Silhouette:           input.Silhouette,
			Material:             input.Material,
			Season:               input.Season,
			UserNotes:            input.Notes,
			IsCore:               true,
			RecommendationStatus: RecommendationStatusNormal,
			Status:               StatusActive,
		})
	}
	if len(items) == 0 {
		return []Item{}, nil
	}
	return s.repo.CreateCoreItems(ctx, items)
}

func (s *Service) ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error) {
	filter.Category = strings.TrimSpace(filter.Category)
	filter.RecommendationStatus = strings.TrimSpace(filter.RecommendationStatus)
	if filter.RecommendationStatus != "" && !isValidRecommendationStatus(filter.RecommendationStatus) {
		return nil, ErrInvalidRecommendationStatus
	}
	repo, err := s.itemRepo()
	if err != nil {
		return nil, err
	}
	items, err := repo.ListItems(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	s.enrichPrimaryImageURLs(ctx, items)
	return items, nil
}

func (s *Service) GetItem(ctx context.Context, userID int64, publicID string) (Item, error) {
	repo, err := s.itemRepo()
	if err != nil {
		return Item{}, err
	}
	item, err := repo.FindItemForUser(ctx, userID, strings.TrimSpace(publicID))
	if err != nil {
		return Item{}, err
	}
	return s.enrichPrimaryImageURL(ctx, item), nil
}

func (s *Service) CreateItem(ctx context.Context, userID int64, input CreateInput) (Item, error) {
	assetPublicIDs := trimStringSlice(input.AssetPublicIDs)
	primaryAssetPublicID := strings.TrimSpace(input.PrimaryAssetPublicID)
	if len(assetPublicIDs) == 0 && primaryAssetPublicID != "" {
		assetPublicIDs = []string{primaryAssetPublicID}
	}
	name := strings.TrimSpace(input.Name)
	if name == "" && len(assetPublicIDs) == 0 {
		return Item{}, ErrInvalidItemName
	}
	if name == "" {
		name = "识别中"
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = "其他"
	}
	recommendationStatus := strings.TrimSpace(input.RecommendationStatus)
	if recommendationStatus == "" {
		recommendationStatus = RecommendationStatusNormal
	}
	if !isValidRecommendationStatus(recommendationStatus) {
		return Item{}, ErrInvalidRecommendationStatus
	}
	material := strings.TrimSpace(input.Material)
	silhouette := strings.TrimSpace(input.Silhouette)
	season := strings.TrimSpace(input.Season)
	repo, err := s.itemRepo()
	if err != nil {
		return Item{}, err
	}
	isCore := true
	if input.IsCore != nil {
		isCore = *input.IsCore
	}
	item := Item{
		PublicID:             id.NewPublicID("wdi"),
		UserID:               userID,
		Name:                 name,
		Category:             category,
		Color:                strings.TrimSpace(input.Color),
		Silhouette:           silhouette,
		Material:             material,
		Season:               season,
		SceneTags:            trimStringSlice(input.SceneTags),
		UserNotes:            strings.TrimSpace(input.UserNotes),
		IsCore:               isCore,
		RecommendationStatus: recommendationStatus,
		RecognitionStatus:    recognitionStatusForCreate(input, assetPublicIDs),
		Status:               StatusActive,
	}
	var created Item
	if len(assetPublicIDs) > 0 {
		if multiRepo, ok := repo.(multiAssetItemRepository); ok {
			created, err = multiRepo.CreateItemWithAssets(ctx, item, assetPublicIDs)
		} else {
			created, err = repo.CreateItem(ctx, item, assetPublicIDs[0])
		}
	} else {
		created, err = repo.CreateItem(ctx, item, primaryAssetPublicID)
	}
	if err != nil {
		return Item{}, err
	}
	if len(assetPublicIDs) > 0 && s.recognitionJobCreator != nil {
		jobPublicID, err := s.recognitionJobCreator.CreateClothesRecognitionJob(ctx, userID, created.ID, created.PublicID, assetPublicIDs, false)
		if err != nil {
			return Item{}, err
		}
		created.RecognitionJobPublicID = jobPublicID
	}
	return s.enrichPrimaryImageURL(ctx, created), nil
}

func recognitionStatusForCreate(input CreateInput, assetPublicIDs []string) string {
	status := strings.TrimSpace(input.RecognitionStatus)
	if status != "" {
		return status
	}
	if len(assetPublicIDs) > 0 {
		return RecognitionStatusPending
	}
	return RecognitionStatusSucceeded
}

func (s *Service) UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Item{}, ErrInvalidItemName
		}
		input.Name = &name
	}
	if input.RecommendationStatus != nil {
		status := strings.TrimSpace(*input.RecommendationStatus)
		if status == "" || !isValidRecommendationStatus(status) {
			return Item{}, ErrInvalidRecommendationStatus
		}
		input.RecommendationStatus = &status
	}
	if input.SceneTags != nil {
		sceneTags := trimStringSlice(*input.SceneTags)
		input.SceneTags = &sceneTags
	}
	trimStringPtr(input.Category)
	trimStringPtr(input.Color)
	trimStringPtr(input.Silhouette)
	trimStringPtr(input.Material)
	trimStringPtr(input.Season)
	trimStringPtr(input.UserNotes)
	trimStringPtr(input.PrimaryAssetPublicID)
	if input.AssetPublicIDs != nil {
		assetPublicIDs := trimStringSlice(*input.AssetPublicIDs)
		input.AssetPublicIDs = &assetPublicIDs
		if len(assetPublicIDs) > 0 {
			input.PrimaryAssetPublicID = nil
		}
	}
	repo, err := s.itemRepo()
	if err != nil {
		return Item{}, err
	}
	publicID = strings.TrimSpace(publicID)
	current, err := repo.FindItemForUser(ctx, userID, publicID)
	if err != nil {
		return Item{}, err
	}
	if current.RecognitionStatus == RecognitionStatusPending {
		return Item{}, ErrRecognitionPending
	}
	if input.AssetPublicIDs != nil {
		assetPublicIDs := *input.AssetPublicIDs
		if len(assetPublicIDs) == 0 {
			empty := ""
			input.PrimaryAssetPublicID = &empty
		} else {
			assetRepo, ok := repo.(assetReplacingItemRepository)
			if !ok {
				return Item{}, ErrRepositoryUnsupported
			}
			if _, err := repo.UpdateItem(ctx, userID, publicID, input); err != nil {
				return Item{}, err
			}
			updated, err := assetRepo.ReplaceItemAssets(ctx, userID, publicID, assetPublicIDs)
			if err != nil {
				return Item{}, err
			}
			return s.enrichPrimaryImageURL(ctx, updated), nil
		}
	}
	item, err := repo.UpdateItem(ctx, userID, publicID, input)
	if err != nil {
		return Item{}, err
	}
	return s.enrichPrimaryImageURL(ctx, item), nil
}

func (s *Service) ListClothesOptions(ctx context.Context) (ClothesOptions, error) {
	return s.clothOptions(ctx), nil
}

func (s *Service) SoftDeleteItem(ctx context.Context, userID int64, publicID string) error {
	repo, err := s.itemRepo()
	if err != nil {
		return err
	}
	return repo.SoftDeleteItem(ctx, userID, strings.TrimSpace(publicID))
}

func (s *Service) AdviceContextItems(ctx context.Context, userID int64, filter AdviceContextFilter) ([]Item, error) {
	repo, err := s.itemRepo()
	if err != nil {
		return nil, err
	}
	items, err := repo.ListItems(ctx, userID, ListFilter{})
	if err != nil {
		return nil, err
	}
	scene := strings.TrimSpace(filter.Scene)
	result := make([]Item, 0, len(items))
	for _, item := range items {
		if item.Status != StatusActive || item.RecommendationStatus == RecommendationStatusPaused {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := adviceRank(result[i], scene)
		right := adviceRank(result[j], scene)
		if left != right {
			return left > right
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

func (s *Service) RecognizeItemImage(ctx context.Context, userID int64, input RecognizeImageInput) (RecognizedItemFields, error) {
	input.AssetPublicID = strings.TrimSpace(input.AssetPublicID)
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.AssetPublicID == "" {
		return RecognizedItemFields{}, ErrInvalidPrimaryAsset
	}
	if s == nil || s.imageRecognizer == nil {
		return RecognizedItemFields{}, ErrImageRecognizerUnavailable
	}
	if input.ImageURL == "" {
		if s.imageURLSigner == nil {
			return RecognizedItemFields{}, ErrImageRecognizerUnavailable
		}
		repo, ok := s.repo.(recognizerAssetRepository)
		if !ok {
			return RecognizedItemFields{}, ErrRepositoryUnsupported
		}
		image, err := repo.FindRecognizableAsset(ctx, userID, input.AssetPublicID)
		if err != nil {
			if errors.Is(err, ErrItemNotFound) {
				return RecognizedItemFields{}, ErrInvalidPrimaryAsset
			}
			return RecognizedItemFields{}, err
		}
		imageURL, err := s.imageURLSigner.PrivateDownloadURL(ctx, image.ObjectKey)
		if err != nil {
			return RecognizedItemFields{}, err
		}
		input.ImageURL = strings.TrimSpace(imageURL)
		if input.ImageURL == "" {
			return RecognizedItemFields{}, ErrImageRecognizerUnavailable
		}
	}
	result, err := s.imageRecognizer.RecognizeClothesItemImage(ctx, userID, input)
	if err != nil {
		return RecognizedItemFields{}, err
	}
	return normalizeRecognizedItemFields(result), nil
}

func (s *Service) ScheduleItemImageRecognition(ctx context.Context, userID int64, input RecognizeImageInput) (map[string]any, error) {
	input.AssetPublicID = strings.TrimSpace(input.AssetPublicID)
	input.ItemPublicID = strings.TrimSpace(input.ItemPublicID)
	if input.AssetPublicID == "" || input.ItemPublicID == "" {
		return nil, ErrInvalidPrimaryAsset
	}
	if s == nil || s.recognitionJobCreator == nil {
		return nil, ErrImageRecognizerUnavailable
	}
	repo, err := s.itemRepo()
	if err != nil {
		return nil, err
	}
	item, err := repo.FindItemForUser(ctx, userID, input.ItemPublicID)
	if err != nil {
		return nil, err
	}
	pending := RecognitionStatusPending
	if _, err := repo.UpdateItem(ctx, userID, item.PublicID, UpdateInput{RecognitionStatus: &pending}); err != nil {
		return nil, err
	}
	jobPublicID, err := s.recognitionJobCreator.CreateClothesRecognitionJob(ctx, userID, item.ID, item.PublicID, []string{input.AssetPublicID}, input.Overwrite)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"job_public_id":      jobPublicID,
		"recognition_status": RecognitionStatusPending,
	}, nil
}

func (s *Service) RecognizeAndApplyItemImage(ctx context.Context, userID int64, publicID string, assetPublicIDs []string, overwrite bool) (Item, error) {
	assetPublicIDs = trimStringSlice(assetPublicIDs)
	if len(assetPublicIDs) == 0 {
		return Item{}, ErrInvalidPrimaryAsset
	}
	repo, err := s.itemRepo()
	if err != nil {
		return Item{}, err
	}
	fields, err := s.RecognizeItemImage(ctx, userID, RecognizeImageInput{AssetPublicID: assetPublicIDs[0]})
	if err != nil {
		failed := RecognitionStatusFailed
		_, _ = repo.UpdateItem(ctx, userID, strings.TrimSpace(publicID), UpdateInput{RecognitionStatus: &failed})
		return Item{}, err
	}
	input := updateInputFromRecognizedFields(fields, overwrite)
	succeeded := RecognitionStatusSucceeded
	input.RecognitionStatus = &succeeded
	item, err := repo.UpdateItem(ctx, userID, strings.TrimSpace(publicID), input)
	if err != nil {
		return Item{}, err
	}
	return s.enrichPrimaryImageURL(ctx, item), nil
}

func updateInputFromRecognizedFields(fields RecognizedItemFields, overwrite bool) UpdateInput {
	input := UpdateInput{}
	if fields.Name != "" || overwrite {
		value := fields.Name
		if value == "" {
			value = "识别中"
		}
		input.Name = &value
	}
	if fields.Category != "" || overwrite {
		value := fields.Category
		if value == "" {
			value = "其他"
		}
		input.Category = &value
	}
	if fields.Color != "" || overwrite {
		value := fields.Color
		input.Color = &value
	}
	if fields.Silhouette != "" || overwrite {
		value := fields.Silhouette
		input.Silhouette = &value
	}
	if fields.Material != "" || overwrite {
		value := fields.Material
		input.Material = &value
	}
	if fields.Season != "" || overwrite {
		value := fields.Season
		input.Season = &value
	}
	if len(fields.SceneTags) > 0 || overwrite {
		value := trimStringSlice(fields.SceneTags)
		input.SceneTags = &value
	}
	if fields.UserNotes != "" || overwrite {
		value := fields.UserNotes
		input.UserNotes = &value
	}
	return input
}

func (s *Service) itemRepo() (itemRepository, error) {
	repo, ok := s.repo.(itemRepository)
	if !ok {
		return nil, ErrRepositoryUnsupported
	}
	return repo, nil
}

func (s *Service) clothOptions(ctx context.Context) ClothesOptions {
	repo, ok := s.repo.(optionsRepository)
	if !ok {
		return DefaultClothesOptions()
	}
	options, err := repo.ListClothesOptions(ctx)
	if err != nil {
		return DefaultClothesOptions()
	}
	return mergeWithDefaultClothesOptions(options)
}

func (s *Service) enrichPrimaryImageURLs(ctx context.Context, items []Item) {
	if len(items) == 0 {
		return
	}
	batchSigner, ok := s.imageURLSigner.(BatchImageURLSigner)
	if !ok || batchSigner == nil {
		for i := range items {
			items[i] = s.enrichPrimaryImageURL(ctx, items[i])
		}
		return
	}
	objectKeys := uniquePrimaryImageObjectKeys(items)
	if len(objectKeys) == 0 {
		return
	}
	signedURLs, err := batchSigner.PrivateImageURLs(ctx, objectKeys)
	if err != nil {
		if s.imageURLSignErrorHandler != nil {
			for _, objectKey := range objectKeys {
				s.imageURLSignErrorHandler(ctx, objectKey, err)
			}
		}
		return
	}
	for i := range items {
		if items[i].PrimaryImage == nil {
			continue
		}
		image := *items[i].PrimaryImage
		image.PreviewURL = ""
		image.OriginalURL = ""
		objectKey := strings.TrimSpace(image.ObjectKey)
		if urls, ok := signedURLs[objectKey]; ok {
			image.PreviewURL = strings.TrimSpace(urls.PreviewURL)
			image.OriginalURL = strings.TrimSpace(urls.OriginalURL)
		}
		items[i].PrimaryImage = &image
	}
}

func (s *Service) enrichPrimaryImageURL(ctx context.Context, item Item) Item {
	if item.PrimaryImage == nil {
		return item
	}
	image := *item.PrimaryImage
	image.PreviewURL = ""
	image.OriginalURL = ""
	objectKey := strings.TrimSpace(image.ObjectKey)
	if s != nil && s.imageURLSigner != nil && objectKey != "" {
		if batchSigner, ok := s.imageURLSigner.(BatchImageURLSigner); ok {
			if urls, err := batchSigner.PrivateImageURLs(ctx, []string{objectKey}); err == nil {
				image.PreviewURL = strings.TrimSpace(urls[objectKey].PreviewURL)
				image.OriginalURL = strings.TrimSpace(urls[objectKey].OriginalURL)
			} else if s.imageURLSignErrorHandler != nil {
				s.imageURLSignErrorHandler(ctx, objectKey, err)
			}
		} else if url, err := s.imageURLSigner.PrivateDownloadURL(ctx, objectKey); err == nil {
			image.OriginalURL = strings.TrimSpace(url)
		} else if s.imageURLSignErrorHandler != nil {
			s.imageURLSignErrorHandler(ctx, objectKey, err)
		}
	}
	item.PrimaryImage = &image
	return item
}

func uniquePrimaryImageObjectKeys(items []Item) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if item.PrimaryImage == nil {
			continue
		}
		objectKey := strings.TrimSpace(item.PrimaryImage.ObjectKey)
		if objectKey == "" || seen[objectKey] {
			continue
		}
		seen[objectKey] = true
		keys = append(keys, objectKey)
	}
	return keys
}

func isValidRecommendationStatus(status string) bool {
	switch status {
	case RecommendationStatusPreferred, RecommendationStatusNormal, RecommendationStatusPaused:
		return true
	default:
		return false
	}
}

func adviceRank(item Item, scene string) int {
	rank := 0
	if item.RecommendationStatus == RecommendationStatusPreferred {
		rank += 100
	}
	if item.IsCore {
		rank += 20
	}
	if scene != "" && hasSceneTag(item.SceneTags, scene) {
		rank += 10
	}
	return rank
}

func hasSceneTag(tags []string, scene string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), scene) {
			return true
		}
	}
	return false
}

func trimStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func trimStringPtr(value *string) {
	if value == nil {
		return
	}
	trimmed := strings.TrimSpace(*value)
	*value = trimmed
}

func normalizeRecognizedItemFields(value RecognizedItemFields) RecognizedItemFields {
	return RecognizedItemFields{
		Name:       strings.TrimSpace(value.Name),
		Category:   strings.TrimSpace(value.Category),
		Color:      strings.TrimSpace(value.Color),
		Silhouette: strings.TrimSpace(value.Silhouette),
		Material:   strings.TrimSpace(value.Material),
		Season:     strings.TrimSpace(value.Season),
		SceneTags:  trimStringSlice(value.SceneTags),
		UserNotes:  strings.TrimSpace(value.UserNotes),
		Confidence: value.Confidence,
	}
}

func DefaultClothesOptions() ClothesOptions {
	return ClothesOptions{
		Categories:  []string{"上装", "下装", "外套", "鞋", "包", "配饰", "运动", "家居", "其他"},
		Colors:      []string{"黑色", "白色", "米白", "灰色", "深蓝", "浅蓝", "棕色", "卡其", "红色", "绿色", "其他"},
		Materials:   []string{"棉", "亚麻", "羊毛", "针织", "牛仔", "真丝", "皮革", "聚酯纤维", "混纺", "其他"},
		Seasons:     []string{"春夏", "春秋", "秋冬", "夏季", "冬季", "四季"},
		Silhouettes: []string{"修身", "合身", "微宽松", "宽松", "直筒", "A 字", "短款", "长款", "高腰", "其他"},
	}
}

func mergeWithDefaultClothesOptions(options ClothesOptions) ClothesOptions {
	defaults := DefaultClothesOptions()
	if len(options.Categories) == 0 {
		options.Categories = defaults.Categories
	}
	if len(options.Colors) == 0 {
		options.Colors = defaults.Colors
	}
	if len(options.Materials) == 0 {
		options.Materials = defaults.Materials
	}
	if len(options.Seasons) == 0 {
		options.Seasons = defaults.Seasons
	}
	if len(options.Silhouettes) == 0 {
		options.Silhouettes = defaults.Silhouettes
	}
	return options
}
