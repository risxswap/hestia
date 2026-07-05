package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"hestia/server/internal/common/id"
	"hestia/server/internal/domain/generator"
)

const (
	StatusActive               = "active"
	SourceOnboarding           = "onboarding"
	SourceUser                 = "user"
	PolarityPositive           = "positive"
	PolarityNegative           = "negative"
	PrefTypeStyleGoal          = "style_goal"
	PrefTypeAvoidance          = "avoidance"
	PrefTypeScenarioPreference = "scenario_preference"
)

const (
	maxProfileTextLength = 220
	maxScenarioCount     = 8
	maxScenarioLength    = 40
	maxPreferenceCount   = 12
	maxPreferenceLength  = 60
	minHeightCM          = 80
	maxHeightCM          = 250
	minWeightKG          = 20.0
	maxWeightKG          = 300.0
	maxFaceShapeLength   = 80
	maxProfilePhotoCount = 18
	maxPhotoGroupCount   = 6
	maxPhotoNoteLength   = 220
)

type Repository interface {
	Summary(ctx context.Context, userID int64) (Summary, error)
	UpdateExplicitProfile(ctx context.Context, userID int64, input UpdateProfileInput) (Summary, error)
	UpdateExplicitPreferences(ctx context.Context, userID int64, input UpdatePreferencesInput) (Summary, error)
	CreateProfilePhoto(ctx context.Context, userID int64, input CreateProfilePhotoInput) (ProfilePhoto, error)
	UpdateProfilePhoto(ctx context.Context, userID int64, publicID string, input UpdateProfilePhotoInput) (ProfilePhoto, error)
	DeleteProfilePhoto(ctx context.Context, userID int64, publicID string) error
	Upsert(ctx context.Context, item Profile) (Profile, error)
	ReplaceFacts(ctx context.Context, userID int64, profileID int64, facts []Fact) error
	ReplacePrefs(ctx context.Context, userID int64, profileID int64, prefs []Pref) error
	CreateInferences(ctx context.Context, inferences []Inference) error
	MarkUserOnboardingCompleted(ctx context.Context, userID int64) error
}

type SignedImageURLs = struct {
	PreviewURL  string
	OriginalURL string
}

type ImageURLSigner interface {
	PrivateImageURLs(ctx context.Context, objectKeys []string) (map[string]SignedImageURLs, error)
}

type Service struct {
	repo           Repository
	imageURLSigner ImageURLSigner
}

var ErrValidation = errors.New("profile validation failed")
var ErrUserNotFound = errors.New("profile user not found")
var ErrProfilePhotoNotFound = errors.New("profile photo not found")
var ErrProfilePhotoLimit = errors.New("profile photo limit reached")
var ErrProfileAssetNotFound = errors.New("profile photo asset not found")

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return ErrValidation.Error()
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (e ValidationError) Is(target error) bool {
	return target == ErrValidation
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

func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	summary, err := s.repo.Summary(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.ProfilePhotos = s.enrichProfilePhotos(ctx, summary.ProfilePhotos)
	summary.QuickEntries = buildQuickEntries(summary)
	return summary, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, request UpdateProfileRequest) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	input, err := validateUpdateProfile(request)
	if err != nil {
		return Summary{}, err
	}
	summary, err := s.repo.UpdateExplicitProfile(ctx, userID, input)
	if err != nil {
		return Summary{}, err
	}
	summary.ProfilePhotos = s.enrichProfilePhotos(ctx, summary.ProfilePhotos)
	summary.QuickEntries = buildQuickEntries(summary)
	return summary, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, userID int64, request UpdatePreferencesRequest) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	input, err := validateUpdatePreferences(request)
	if err != nil {
		return Summary{}, err
	}
	summary, err := s.repo.UpdateExplicitPreferences(ctx, userID, input)
	if err != nil {
		return Summary{}, err
	}
	summary.ProfilePhotos = s.enrichProfilePhotos(ctx, summary.ProfilePhotos)
	summary.QuickEntries = buildQuickEntries(summary)
	return summary, nil
}

func (s *Service) CreateProfilePhoto(ctx context.Context, userID int64, request CreateProfilePhotoRequest) (ProfilePhoto, error) {
	if s == nil || s.repo == nil {
		return ProfilePhoto{}, errors.New("profile service dependencies are nil")
	}
	input, err := validateCreateProfilePhoto(request)
	if err != nil {
		return ProfilePhoto{}, err
	}
	photo, err := s.repo.CreateProfilePhoto(ctx, userID, input)
	if err != nil {
		return ProfilePhoto{}, err
	}
	return s.enrichProfilePhoto(ctx, photo), nil
}

func (s *Service) UpdateProfilePhoto(ctx context.Context, userID int64, publicID string, request UpdateProfilePhotoRequest) (ProfilePhoto, error) {
	if s == nil || s.repo == nil {
		return ProfilePhoto{}, errors.New("profile service dependencies are nil")
	}
	input, err := validateUpdateProfilePhoto(request)
	if err != nil {
		return ProfilePhoto{}, err
	}
	photo, err := s.repo.UpdateProfilePhoto(ctx, userID, strings.TrimSpace(publicID), input)
	if err != nil {
		return ProfilePhoto{}, err
	}
	return s.enrichProfilePhoto(ctx, photo), nil
}

func (s *Service) DeleteProfilePhoto(ctx context.Context, userID int64, publicID string) error {
	if s == nil || s.repo == nil {
		return errors.New("profile service dependencies are nil")
	}
	if strings.TrimSpace(publicID) == "" {
		return ValidationError{Field: "public_id", Message: "required"}
	}
	return s.repo.DeleteProfilePhoto(ctx, userID, strings.TrimSpace(publicID))
}

func (s *Service) UpsertFromOnboarding(ctx context.Context, userID int64, input OnboardingInput) (Profile, error) {
	profile := Profile{
		PublicID:           id.NewPublicID("prf"),
		UserID:             userID,
		Status:             StatusActive,
		Gender:             input.Gender,
		HeightCM:           input.HeightCM,
		BodyNotes:          input.BodyNotes,
		SkinNotes:          input.SkinNotes,
		HairNotes:          input.HairNotes,
		LifestyleScenarios: input.LifestyleScenarios,
		StyleGoalSummary:   strings.Join(input.StyleGoals, "、"),
	}
	created, err := s.repo.Upsert(ctx, profile)
	if err != nil {
		return Profile{}, err
	}

	facts := []Fact{
		{Key: "gender", Value: input.Gender, Source: SourceOnboarding},
		{Key: "height_cm", Value: input.HeightCM, Source: SourceOnboarding},
		{Key: "lifestyle_scenarios", Value: input.LifestyleScenarios, Source: SourceOnboarding},
	}
	if err := s.repo.ReplaceFacts(ctx, userID, created.ID, facts); err != nil {
		return Profile{}, err
	}

	prefs := make([]Pref, 0, len(input.StyleGoals)+len(input.Avoidances))
	for _, goal := range input.StyleGoals {
		if strings.TrimSpace(goal) == "" {
			continue
		}
		prefs = append(prefs, Pref{Type: PrefTypeStyleGoal, Key: goal, Value: goal, Polarity: PolarityPositive, Source: SourceOnboarding})
	}
	for _, avoidance := range input.Avoidances {
		if strings.TrimSpace(avoidance) == "" {
			continue
		}
		prefs = append(prefs, Pref{Type: PrefTypeAvoidance, Key: avoidance, Value: avoidance, Polarity: PolarityNegative, Source: SourceOnboarding})
	}
	if err := s.repo.ReplacePrefs(ctx, userID, created.ID, prefs); err != nil {
		return Profile{}, err
	}
	return created, nil
}

func (s *Service) SaveGeneratorInferences(ctx context.Context, userID int64, profileID int64, sourceJobID int64, inputs []generator.ProfileInferenceInput) error {
	if len(inputs) == 0 {
		return nil
	}
	inferences := make([]Inference, 0, len(inputs))
	for _, input := range inputs {
		confidence := input.Confidence
		inferences = append(inferences, Inference{
			UserID:      userID,
			ProfileID:   profileID,
			Key:         input.Key,
			Value:       input.Value,
			Confidence:  &confidence,
			SourceJobID: sourceJobID,
		})
	}
	return s.repo.CreateInferences(ctx, inferences)
}

func (s *Service) CompleteOnboarding(ctx context.Context, userID int64) error {
	return s.repo.MarkUserOnboardingCompleted(ctx, userID)
}

func validateUpdateProfile(request UpdateProfileRequest) (UpdateProfileInput, error) {
	input := UpdateProfileInput{
		Nickname:       sanitizePatchString(request.Nickname),
		Gender:         sanitizePatchString(request.Gender),
		HeightCM:       request.HeightCM,
		WeightKG:       request.WeightKG,
		BodyNotes:      sanitizePatchString(request.BodyNotes),
		SkinNotes:      sanitizePatchString(request.SkinNotes),
		HairNotes:      sanitizePatchString(request.HairNotes),
		FaceShape:      sanitizePatchString(request.FaceShape),
		UpperBodyNotes: sanitizePatchString(request.UpperBodyNotes),
		LowerBodyNotes: sanitizePatchString(request.LowerBodyNotes),
		SizeNotes:      sanitizePatchString(request.SizeNotes),
	}
	if err := validatePatchString(input.Nickname, "nickname", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.Gender, "gender", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.BodyNotes, "body_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.SkinNotes, "skin_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.HairNotes, "hair_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.FaceShape, "face_shape", maxFaceShapeLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.UpperBodyNotes, "upper_body_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.LowerBodyNotes, "lower_body_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.SizeNotes, "size_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if input.HeightCM.Present && input.HeightCM.Value != nil && (*input.HeightCM.Value < minHeightCM || *input.HeightCM.Value > maxHeightCM) {
		return UpdateProfileInput{}, ValidationError{Field: "height_cm", Message: "out of range"}
	}
	if input.WeightKG.Present && input.WeightKG.Value != nil && (*input.WeightKG.Value < minWeightKG || *input.WeightKG.Value > maxWeightKG) {
		return UpdateProfileInput{}, ValidationError{Field: "weight_kg", Message: "out of range"}
	}
	if request.LifestyleScenarios.Present {
		scenarios, err := sanitizeStringList(request.LifestyleScenarios.Value, maxScenarioCount, maxScenarioLength, "lifestyle_scenarios")
		if err != nil {
			return UpdateProfileInput{}, err
		}
		input.LifestyleScenarios = PatchStringSlice{Present: true, Value: scenarios}
	}
	return input, nil
}

func validateCreateProfilePhoto(request CreateProfilePhotoRequest) (CreateProfilePhotoInput, error) {
	input := CreateProfilePhotoInput{
		AssetPublicID: strings.TrimSpace(request.AssetPublicID),
		PhotoType:     strings.TrimSpace(request.PhotoType),
		Angle:         strings.TrimSpace(request.Angle),
		Note:          strings.TrimSpace(request.Note),
		SortOrder:     request.SortOrder,
	}
	if input.AssetPublicID == "" {
		return CreateProfilePhotoInput{}, ValidationError{Field: "asset_public_id", Message: "required"}
	}
	if !validPhotoType(input.PhotoType) {
		return CreateProfilePhotoInput{}, ValidationError{Field: "photo_type", Message: "unsupported"}
	}
	if !validPhotoAngle(input.Angle) {
		return CreateProfilePhotoInput{}, ValidationError{Field: "angle", Message: "unsupported"}
	}
	if len([]rune(input.Note)) > maxPhotoNoteLength {
		return CreateProfilePhotoInput{}, ValidationError{Field: "note", Message: "too long"}
	}
	return input, nil
}

func validateUpdateProfilePhoto(request UpdateProfilePhotoRequest) (UpdateProfilePhotoInput, error) {
	input := UpdateProfilePhotoInput{
		PhotoType: sanitizePatchString(request.PhotoType),
		Angle:     sanitizePatchString(request.Angle),
		Note:      sanitizePatchString(request.Note),
		SortOrder: request.SortOrder,
		Status:    sanitizePatchString(request.Status),
	}
	if input.PhotoType.Present && !validPhotoType(input.PhotoType.Value) {
		return UpdateProfilePhotoInput{}, ValidationError{Field: "photo_type", Message: "unsupported"}
	}
	if input.Angle.Present && !validPhotoAngle(input.Angle.Value) {
		return UpdateProfilePhotoInput{}, ValidationError{Field: "angle", Message: "unsupported"}
	}
	if err := validatePatchString(input.Note, "note", maxPhotoNoteLength); err != nil {
		return UpdateProfilePhotoInput{}, err
	}
	if input.Status.Present && input.Status.Value != StatusActive {
		return UpdateProfilePhotoInput{}, ValidationError{Field: "status", Message: "unsupported"}
	}
	return input, nil
}

func validPhotoType(value string) bool {
	switch value {
	case "headshot", "half_body", "full_body":
		return true
	default:
		return false
	}
}

func validPhotoAngle(value string) bool {
	switch value {
	case "front", "left_45", "right_45", "side", "back", "natural", "sitting", "other":
		return true
	default:
		return false
	}
}

func (s *Service) enrichProfilePhoto(ctx context.Context, photo ProfilePhoto) ProfilePhoto {
	photos := s.enrichProfilePhotos(ctx, []ProfilePhoto{photo})
	if len(photos) == 0 {
		return photo
	}
	return photos[0]
}

func (s *Service) enrichProfilePhotos(ctx context.Context, photos []ProfilePhoto) []ProfilePhoto {
	if s == nil || s.imageURLSigner == nil || len(photos) == 0 {
		return photos
	}
	objectKeys := make([]string, 0, len(photos))
	for _, photo := range photos {
		if photo.Image != nil && strings.TrimSpace(photo.Image.ObjectKey) != "" {
			objectKeys = append(objectKeys, strings.TrimSpace(photo.Image.ObjectKey))
		}
	}
	if len(objectKeys) == 0 {
		return photos
	}
	urls, err := s.imageURLSigner.PrivateImageURLs(ctx, objectKeys)
	if err != nil {
		return photos
	}
	for index := range photos {
		if photos[index].Image == nil {
			continue
		}
		signed := urls[photos[index].Image.ObjectKey]
		if signed.PreviewURL != "" {
			photos[index].Image.URL = signed.PreviewURL
		} else if signed.OriginalURL != "" {
			photos[index].Image.URL = signed.OriginalURL
		}
	}
	return photos
}

func sanitizePatchString(value PatchString) PatchString {
	if !value.Present {
		return value
	}
	value.Value = strings.TrimSpace(value.Value)
	return value
}

func validatePatchString(value PatchString, field string, maxLength int) error {
	if !value.Present {
		return nil
	}
	if len([]rune(value.Value)) > maxLength {
		return ValidationError{Field: field, Message: "too long"}
	}
	return nil
}

func validateUpdatePreferences(request UpdatePreferencesRequest) (UpdatePreferencesInput, error) {
	styleGoals, err := sanitizeStringList(request.StyleGoals, maxPreferenceCount, maxPreferenceLength, "style_goals")
	if err != nil {
		return UpdatePreferencesInput{}, err
	}
	avoidances, err := sanitizeStringList(request.Avoidances, maxPreferenceCount, maxPreferenceLength, "avoidances")
	if err != nil {
		return UpdatePreferencesInput{}, err
	}
	scenarios, err := sanitizeStringList(request.ScenarioPreferences, maxPreferenceCount, maxPreferenceLength, "scenario_preferences")
	if err != nil {
		return UpdatePreferencesInput{}, err
	}
	return UpdatePreferencesInput{
		StyleGoals:          styleGoals,
		Avoidances:          avoidances,
		ScenarioPreferences: scenarios,
	}, nil
}

func sanitizeStringList(values []string, maxCount int, maxLength int, field string) ([]string, error) {
	if len(values) > maxCount {
		return nil, ValidationError{Field: field, Message: "too many"}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if len([]rune(trimmed)) > maxLength {
			return nil, ValidationError{Field: field, Message: "too long"}
		}
		result = append(result, trimmed)
	}
	return result, nil
}

func buildQuickEntries(summary Summary) []QuickEntry {
	scenarioCount := 0
	if summary.Profile != nil {
		scenarioCount = len(summary.Profile.LifestyleScenarios)
	}
	profileSummary := "还没有记录常见场景"
	if scenarioCount > 0 {
		profileSummary = fmt.Sprintf("已记录 %d 个常见场景", scenarioCount)
	}

	reportSummary := "暂无初版报告"
	if summary.LatestReport != nil {
		reportSummary = "初版报告已生成"
	}

	return []QuickEntry{
		{
			Key:     "profile",
			Title:   "我的档案",
			Summary: profileSummary,
		},
		{
			Key:     "preferences",
			Title:   "偏好与禁忌",
			Summary: fmt.Sprintf("%d 个偏好、%d 个禁忌", summary.MemorySummary.PreferenceCount, summary.MemorySummary.AvoidanceCount),
		},
		{
			Key:     "report",
			Title:   "报告与路线",
			Summary: reportSummary,
		},
		{
			Key:     "privacy",
			Title:   "隐私与数据",
			Summary: "照片、档案、反馈可管理",
		},
	}
}
