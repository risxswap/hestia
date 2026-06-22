package onboarding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"hestia/server/internal/common/id"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/generator"
	"hestia/server/internal/domain/imageroute"
	"hestia/server/internal/domain/job"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/domain/report"
	"hestia/server/internal/domain/wardrobe"
)

var ErrValidation = errors.New("onboarding validation failed")

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func (e ValidationError) Is(target error) bool {
	return target == ErrValidation
}

type Service struct {
	repo   DraftRepository
	submit SubmitDependencies
}

func NewService(repo DraftRepository) *Service {
	return &Service{repo: repo}
}

type SubmitDependencies struct {
	Drafts      DraftRepository
	Profiles    *profile.Service
	Assets      *asset.Service
	Wardrobe    *wardrobe.Service
	Jobs        *job.Service
	Reports     *report.Service
	ImageRoutes *imageroute.Service
	Generator   generator.ReportGenerator
	Transactor  Transactor
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, deps SubmitDependencies) error) error
}

func NewSubmitService(repo DraftRepository, deps SubmitDependencies) *Service {
	return &Service{repo: repo, submit: deps}
}

func (s *Service) GetDraft(ctx context.Context, userID int64) (DraftResponse, error) {
	if s == nil || s.repo == nil {
		return DraftResponse{}, errors.New("onboarding service dependencies are nil")
	}
	draft, err := s.repo.FindActiveByUserID(ctx, userID)
	if errors.Is(err, ErrDraftNotFound) {
		return DraftResponse{
			Status:      DraftStatusNotStarted,
			CurrentStep: "",
			Version:     0,
			DraftData:   DraftData{},
		}, nil
	}
	if err != nil {
		return DraftResponse{}, err
	}
	return responseFromDraft(draft), nil
}

func (s *Service) SaveDraft(ctx context.Context, userID int64, input SaveDraftInput) (DraftResponse, error) {
	step := strings.TrimSpace(input.Step)
	if step == "" {
		return DraftResponse{}, ValidationError{Field: "step", Message: "required"}
	}
	if s == nil || s.repo == nil {
		return DraftResponse{}, errors.New("onboarding service dependencies are nil")
	}

	existing, err := s.repo.FindActiveByUserID(ctx, userID)
	notFound := errors.Is(err, ErrDraftNotFound)
	if err != nil && !notFound {
		return DraftResponse{}, err
	}

	data := DraftData{}
	if !notFound {
		data = existing.DraftData.Clone()
	}
	for key, value := range input.Data {
		data[key] = append(json.RawMessage(nil), value...)
	}
	contentHash, err := hashDraftData(data)
	if err != nil {
		return DraftResponse{}, err
	}

	if notFound {
		draft := Draft{
			PublicID:    id.NewPublicID("odf"),
			UserID:      userID,
			Status:      DraftStatusDraft,
			CurrentStep: step,
			DraftData:   data,
			ContentHash: contentHash,
			Version:     1,
		}
		created, err := s.repo.Create(ctx, draft)
		if err != nil {
			return DraftResponse{}, err
		}
		return responseFromDraft(created), nil
	}

	expectedVersion := existing.Version
	expectedContentHash := existing.ContentHash
	updatedDraft := existing
	updatedDraft.CurrentStep = step
	updatedDraft.DraftData = data
	updatedDraft.ContentHash = contentHash
	updatedDraft.Version++
	updated, err := s.repo.Update(ctx, updatedDraft, expectedVersion, expectedContentHash)
	if err != nil {
		return DraftResponse{}, err
	}
	return responseFromDraft(updated), nil
}

func (s *Service) Submit(ctx context.Context, userID int64) (SubmitResponse, error) {
	if s == nil || s.repo == nil {
		return SubmitResponse{}, errors.New("onboarding service dependencies are nil")
	}
	if err := s.submit.validate(); err != nil {
		return SubmitResponse{}, err
	}
	draft, err := s.repo.FindActiveByUserID(ctx, userID)
	if errors.Is(err, ErrDraftNotFound) {
		return SubmitResponse{}, ValidationError{Field: "draft", Message: "required"}
	}
	if err != nil {
		return SubmitResponse{}, err
	}
	parsed, err := parseSubmitDraft(draft.DraftData)
	if err != nil {
		return SubmitResponse{}, err
	}
	result, err := s.submit.Generator.GenerateInitialReport(ctx, generator.InitialReportInput{
		UserID:          userID,
		StyleGoals:      parsed.profile.StyleGoals,
		Avoidances:      parsed.profile.Avoidances,
		Scenarios:       parsed.profile.LifestyleScenarios,
		Basic:           parsed.basic,
		BodyNotes:       parsed.profile.BodyNotes,
		SkinNotes:       parsed.profile.SkinNotes,
		HairNotes:       parsed.profile.HairNotes,
		WardrobeItems:   parsed.generatorWardrobe,
		ReferenceStyles: parsed.referenceStyles,
	})
	if err != nil {
		s.recordFailedJob(ctx, userID, draft)
		return SubmitResponse{}, fmt.Errorf("generate initial report: %w", err)
	}
	var response SubmitResponse
	err = s.submit.Transactor.WithinTx(ctx, func(txCtx context.Context, txDeps SubmitDependencies) error {
		if txDeps.Generator == nil {
			txDeps.Generator = s.submit.Generator
		}
		return s.submitInTx(txCtx, userID, draft, parsed, result, txDeps, &response)
	})
	if err != nil {
		if !errors.Is(err, ErrValidation) {
			s.recordFailedJob(ctx, userID, draft)
		}
		return SubmitResponse{}, err
	}
	return response, nil
}

func (s *Service) submitInTx(ctx context.Context, userID int64, draft Draft, parsed parsedSubmitDraft, result *generator.InitialReportResult, deps SubmitDependencies, response *SubmitResponse) error {
	if err := deps.validateWork(); err != nil {
		return err
	}
	if err := deps.Drafts.ClaimDraft(ctx, draft.ID, userID, draft.ContentHash); err != nil {
		if errors.Is(err, ErrDraftNotFound) {
			return ValidationError{Field: "draft", Message: "already submitted or changed"}
		}
		return fmt.Errorf("draft claim: %w", err)
	}
	profileItem, err := deps.Profiles.UpsertFromOnboarding(ctx, userID, parsed.profile)
	if err != nil {
		return fmt.Errorf("profile upsert: %w", err)
	}
	if _, err := deps.Assets.RegisterOnboardingAssets(ctx, userID, parsed.assets); err != nil {
		return fmt.Errorf("assets register: %w", err)
	}
	if _, err := deps.Wardrobe.CreateCoreItems(ctx, userID, parsed.wardrobe); err != nil {
		return fmt.Errorf("wardrobe create: %w", err)
	}

	generationJob, err := deps.Jobs.CreateInitialReportJob(ctx, userID, map[string]any{
		"draft_public_id": draft.PublicID,
		"profile_id":      profileItem.ID,
	})
	if err != nil {
		return fmt.Errorf("job create: %w", err)
	}
	if err := deps.Profiles.SaveGeneratorInferences(ctx, userID, profileItem.ID, generationJob.ID, result.Inferences); err != nil {
		return fmt.Errorf("profile inferences save: %w", err)
	}
	routes, err := deps.ImageRoutes.CreateFromGenerator(ctx, userID, profileItem.ID, generationJob.ID, result.Routes)
	if err != nil {
		return fmt.Errorf("image routes create: %w", err)
	}
	content := map[string]any{
		"summary":               result.Summary,
		"hair_strategy":         result.HairStrategy,
		"makeup_strategy":       result.MakeupStrategy,
		"outfit_strategy":       result.OutfitStrategy,
		"avoidances":            result.Avoidances,
		"wardrobe_combinations": result.WardrobeCombinations,
		"wardrobe_gaps":         result.WardrobeGaps,
		"action_items":          result.ActionItems,
		"reference_style_logic": result.ReferenceStyleLogic,
		"privacy_note":          result.PrivacyNote,
	}
	reportItem, err := deps.Reports.CreateInitialReport(ctx, report.CreateInitialInput{
		UserID:      userID,
		ProfileID:   profileItem.ID,
		JobID:       generationJob.ID,
		Title:       "初版个人形象报告",
		Summary:     result.Summary,
		ContentJSON: content,
		ContextSnapshot: map[string]any{
			"draft_public_id": draft.PublicID,
			"style_goals":     parsed.profile.StyleGoals,
			"avoidances":      parsed.profile.Avoidances,
			"scenarios":       parsed.profile.LifestyleScenarios,
		},
		StyleRefsJSON: map[string]any{
			"reference_styles": parsed.referenceStyles,
			"logic":            result.ReferenceStyleLogic,
		},
	})
	if err != nil {
		return fmt.Errorf("report create: %w", err)
	}
	reportRoutes := make([]report.ReportRoute, 0, len(routes))
	for i, route := range routes {
		reportRoutes = append(reportRoutes, report.ReportRoute{
			ImageRouteID: route.ID,
			RouteRole:    route.RouteRole,
			SortOrder:    i,
			RouteSnapshot: map[string]any{
				"public_id":         route.PublicID,
				"name":              route.Name,
				"route_role":        route.RouteRole,
				"target_impression": route.TargetImpression,
				"suitable_scenes":   route.SuitableScenes,
				"hair_strategy":     route.HairStrategy,
				"makeup_strategy":   route.MakeupStrategy,
				"outfit_strategy":   route.OutfitStrategy,
				"avoid_points":      route.AvoidPoints,
				"reason":            route.Reason,
			},
		})
	}
	if err := deps.Reports.AttachRoutes(ctx, reportItem.ID, reportRoutes); err != nil {
		return fmt.Errorf("report routes attach: %w", err)
	}
	if err := deps.Profiles.CompleteOnboarding(ctx, userID); err != nil {
		return fmt.Errorf("onboarding complete: %w", err)
	}
	if err := deps.Reports.MarkReady(ctx, reportItem.ID); err != nil {
		return fmt.Errorf("report mark ready: %w", err)
	}
	if err := deps.Jobs.MarkSucceeded(ctx, generationJob, map[string]any{"report_public_id": reportItem.PublicID}); err != nil {
		return fmt.Errorf("job mark succeeded: %w", err)
	}
	*response = SubmitResponse{JobPublicID: generationJob.PublicID, ReportPublicID: reportItem.PublicID}
	return nil
}

func (s *Service) recordFailedJob(ctx context.Context, userID int64, draft Draft) {
	if s == nil || s.submit.Jobs == nil {
		return
	}
	failedJob, err := s.submit.Jobs.CreateInitialReportJob(ctx, userID, map[string]any{
		"draft_public_id": draft.PublicID,
	})
	if err != nil {
		return
	}
	_ = s.submit.Jobs.MarkFailed(ctx, failedJob, "initial report submit failed")
}

func (d SubmitDependencies) validate() error {
	if d.Profiles == nil || d.Assets == nil || d.Wardrobe == nil || d.Jobs == nil || d.Reports == nil || d.ImageRoutes == nil || d.Generator == nil || d.Transactor == nil {
		return errors.New("onboarding submit dependencies are nil")
	}
	return nil
}

func (d SubmitDependencies) validateWork() error {
	if d.Drafts == nil || d.Profiles == nil || d.Assets == nil || d.Wardrobe == nil || d.Jobs == nil || d.Reports == nil || d.ImageRoutes == nil {
		return errors.New("onboarding submit transaction dependencies are nil")
	}
	return nil
}

func hashDraftData(data DraftData) (string, error) {
	canonical, err := canonicalJSON(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(data DraftData) ([]byte, error) {
	if len(data) == 0 {
		return []byte("{}"), nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func responseFromDraft(draft Draft) DraftResponse {
	data := draft.DraftData.Clone()
	if data == nil {
		data = DraftData{}
	}
	return DraftResponse{
		Status:      draft.Status,
		CurrentStep: draft.CurrentStep,
		Version:     draft.Version,
		DraftData:   data,
	}
}
