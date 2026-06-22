package onboarding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"hestia/server/internal/common/id"
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
	repo DraftRepository
}

func NewService(repo DraftRepository) *Service {
	return &Service{repo: repo}
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

	existing.CurrentStep = step
	existing.DraftData = data
	existing.ContentHash = contentHash
	existing.Version++
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return DraftResponse{}, err
	}
	return responseFromDraft(updated), nil
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
	var normalized map[string]any
	if err := json.Unmarshal(raw, &normalized); err != nil {
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
