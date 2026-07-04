package wardrobe

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
)

var (
	ErrInvalidRecommendationStatus = errors.New("invalid recommendation status")
	ErrInvalidItemName             = errors.New("invalid wardrobe item name")
	ErrInvalidPrimaryAsset         = errors.New("invalid wardrobe primary asset")
	ErrInvalidWardrobeOption       = errors.New("invalid wardrobe configured option")
	ErrItemNotFound                = errors.New("wardrobe item not found")
	ErrRepositoryUnsupported       = errors.New("wardrobe repository unsupported")
)

type Repository interface {
	CreateCoreItems(ctx context.Context, items []Item) ([]Item, error)
}

type itemRepository interface {
	ListItems(ctx context.Context, userID int64, filter ListFilter) ([]Item, error)
	CreateItem(ctx context.Context, item Item, primaryAssetPublicID string) (Item, error)
	UpdateItem(ctx context.Context, userID int64, publicID string, input UpdateInput) (Item, error)
	SoftDeleteItem(ctx context.Context, userID int64, publicID string) error
}

type optionsRepository interface {
	ListWardrobeOptions(ctx context.Context) (WardrobeOptions, error)
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
	RecognizeWardrobeItemImage(ctx context.Context, userID int64, input RecognizeImageInput) (RecognizedItemFields, error)
}

type ImageURLSignErrorHandler func(ctx context.Context, objectKey string, err error)

type Service struct {
	repo                     Repository
	imageURLSigner           ImageURLSigner
	imageURLSignErrorHandler ImageURLSignErrorHandler
	imageRecognizer          ImageRecognizer
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
			category = "unknown"
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

func (s *Service) CreateItem(ctx context.Context, userID int64, input CreateInput) (Item, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Item{}, ErrInvalidItemName
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = "unknown"
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
	if err := s.validateConfiguredOptions(ctx, category, material, season, silhouette); err != nil {
		return Item{}, err
	}
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
		Status:               StatusActive,
	}
	created, err := repo.CreateItem(ctx, item, strings.TrimSpace(input.PrimaryAssetPublicID))
	if err != nil {
		return Item{}, err
	}
	return s.enrichPrimaryImageURL(ctx, created), nil
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
	if err := s.validateUpdateConfiguredOptions(ctx, input); err != nil {
		return Item{}, err
	}
	repo, err := s.itemRepo()
	if err != nil {
		return Item{}, err
	}
	item, err := repo.UpdateItem(ctx, userID, strings.TrimSpace(publicID), input)
	if err != nil {
		return Item{}, err
	}
	return s.enrichPrimaryImageURL(ctx, item), nil
}

func (s *Service) ListWardrobeOptions(ctx context.Context) (WardrobeOptions, error) {
	return s.wardrobeOptions(ctx), nil
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
	result, err := s.imageRecognizer.RecognizeWardrobeItemImage(ctx, userID, input)
	if err != nil {
		return RecognizedItemFields{}, err
	}
	return normalizeRecognizedItemFields(result), nil
}

func (s *Service) itemRepo() (itemRepository, error) {
	repo, ok := s.repo.(itemRepository)
	if !ok {
		return nil, ErrRepositoryUnsupported
	}
	return repo, nil
}

func (s *Service) wardrobeOptions(ctx context.Context) WardrobeOptions {
	repo, ok := s.repo.(optionsRepository)
	if !ok {
		return DefaultWardrobeOptions()
	}
	options, err := repo.ListWardrobeOptions(ctx)
	if err != nil {
		return DefaultWardrobeOptions()
	}
	return mergeWithDefaultWardrobeOptions(options)
}

func (s *Service) validateConfiguredOptions(ctx context.Context, category string, material string, season string, silhouette string) error {
	options := s.wardrobeOptions(ctx)
	if !optionValueExists(options.Categories, category) {
		return ErrInvalidWardrobeOption
	}
	if material != "" && !optionValueExists(options.Materials, material) {
		return ErrInvalidWardrobeOption
	}
	if season != "" && !optionValueExists(options.Seasons, season) {
		return ErrInvalidWardrobeOption
	}
	if silhouette != "" && !optionValueExists(options.Silhouettes, silhouette) {
		return ErrInvalidWardrobeOption
	}
	return nil
}

func (s *Service) validateUpdateConfiguredOptions(ctx context.Context, input UpdateInput) error {
	options := s.wardrobeOptions(ctx)
	if input.Category != nil && !optionValueExists(options.Categories, *input.Category) {
		return ErrInvalidWardrobeOption
	}
	if input.Material != nil && *input.Material != "" && !optionValueExists(options.Materials, *input.Material) {
		return ErrInvalidWardrobeOption
	}
	if input.Season != nil && *input.Season != "" && !optionValueExists(options.Seasons, *input.Season) {
		return ErrInvalidWardrobeOption
	}
	if input.Silhouette != nil && *input.Silhouette != "" && !optionValueExists(options.Silhouettes, *input.Silhouette) {
		return ErrInvalidWardrobeOption
	}
	return nil
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

func DefaultWardrobeOptions() WardrobeOptions {
	return WardrobeOptions{
		Categories: []OptionItem{
			{Label: "上装", Value: "top"},
			{Label: "下装", Value: "bottom"},
			{Label: "外套", Value: "outerwear"},
			{Label: "鞋", Value: "shoes"},
			{Label: "包", Value: "bag"},
			{Label: "配饰", Value: "accessory"},
			{Label: "运动", Value: "sport"},
			{Label: "家居", Value: "home"},
			{Label: "其他", Value: "other"},
		},
		Materials: []OptionItem{
			{Label: "棉", Value: "cotton"},
			{Label: "亚麻", Value: "linen"},
			{Label: "羊毛", Value: "wool"},
			{Label: "针织", Value: "knit"},
			{Label: "牛仔", Value: "denim"},
			{Label: "真丝", Value: "silk"},
			{Label: "皮革", Value: "leather"},
			{Label: "聚酯纤维", Value: "polyester"},
			{Label: "混纺", Value: "blend"},
			{Label: "其他", Value: "other"},
		},
		Seasons: []OptionItem{
			{Label: "春夏", Value: "spring_summer"},
			{Label: "春秋", Value: "spring_autumn"},
			{Label: "秋冬", Value: "autumn_winter"},
			{Label: "夏季", Value: "summer"},
			{Label: "冬季", Value: "winter"},
			{Label: "四季", Value: "all_season"},
		},
		Silhouettes: []OptionItem{
			{Label: "修身", Value: "fitted"},
			{Label: "合身", Value: "regular"},
			{Label: "微宽松", Value: "slightly_relaxed"},
			{Label: "宽松", Value: "relaxed"},
			{Label: "直筒", Value: "straight"},
			{Label: "A 字", Value: "a_line"},
			{Label: "短款", Value: "cropped"},
			{Label: "长款", Value: "longline"},
			{Label: "高腰", Value: "high_waist"},
			{Label: "其他", Value: "other"},
		},
	}
}

func mergeWithDefaultWardrobeOptions(options WardrobeOptions) WardrobeOptions {
	defaults := DefaultWardrobeOptions()
	if len(options.Categories) == 0 {
		options.Categories = defaults.Categories
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

func optionValueExists(options []OptionItem, value string) bool {
	target := strings.TrimSpace(value)
	if target == "" {
		return false
	}
	for _, option := range options {
		if strings.TrimSpace(option.Value) == target {
			return true
		}
	}
	return false
}
