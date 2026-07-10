package agent

import (
	"context"
	"errors"
	"strings"

	"hestia/server/internal/domain/clothes"
)

const defaultStreamStatusText = "agent stream ready"

var (
	ErrRepositoryUnsupported = errors.New("agent repository unsupported")
	ErrDraftNotFound         = errors.New("advice draft not found")
	ErrDraftNotActive        = errors.New("advice draft is not active")
	ErrDraftIncomplete       = errors.New("advice draft incomplete")
)

type ClothesAdviceService interface {
	AdviceContextItems(ctx context.Context, userID int64, filter clothes.AdviceContextFilter) ([]clothes.Item, error)
}

type Repository interface {
	CreateChatMessage(ctx context.Context, input CreateChatMessageInput) (ChatMessage, error)
	UpdateChatMessage(ctx context.Context, input UpdateChatMessageInput) (ChatMessage, error)
	CreateAgentRunStep(ctx context.Context, input AgentRunStepInput) error
	CreateDraft(ctx context.Context, input CreateDraftInput) (Draft, error)
	GetCurrentDraft(ctx context.Context, userID int64) (Draft, error)
	UpdateDraftSections(ctx context.Context, input UpdateDraftInput) (Draft, error)
	ConfirmDraft(ctx context.Context, userID int64, publicID string) (Advice, error)
	DiscardDraft(ctx context.Context, userID int64, publicID string) error
}

type Service struct {
	clothes ClothesAdviceService
	repo    Repository
}

func NewService() *Service {
	return &Service{}
}

func NewServiceWithClothes(clothesService ClothesAdviceService) *Service {
	return &Service{clothes: clothesService}
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

func NewServiceWithDependencies(repo Repository, clothesService ClothesAdviceService) *Service {
	return &Service{repo: repo, clothes: clothesService}
}

func (s *Service) StreamStatus(ctx context.Context, userID int64) StreamStatus {
	text := defaultStreamStatusText
	if s == nil || s.clothes == nil || userID == 0 {
		return StreamStatus{Text: text}
	}
	items, err := s.clothes.AdviceContextItems(ctx, userID, clothes.AdviceContextFilter{Limit: 5})
	if err != nil || len(items) == 0 {
		return StreamStatus{Text: text}
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return StreamStatus{Text: text}
	}
	return StreamStatus{Text: text + "；核心衣服：" + strings.Join(names, "、")}
}

func (s *Service) Chat(ctx context.Context, userID int64, text string) (ChatResult, error) {
	message := StreamMessage{Text: strings.TrimSpace(text)}
	if message.Text == "" {
		message.Text = "我会先根据你的场景生成一个可调整的形象建议草稿。"
	}
	if s == nil || s.repo == nil {
		return ChatResult{Message: message}, nil
	}
	userMessage, err := s.repo.CreateChatMessage(ctx, CreateChatMessageInput{
		UserID:      userID,
		Role:        ChatRoleUser,
		MsgType:     ChatMsgTypeText,
		ContentText: message.Text,
		Status:      ChatStatusSent,
	})
	if err != nil {
		return ChatResult{}, err
	}
	assistantMessage, err := s.repo.CreateChatMessage(ctx, CreateChatMessageInput{
		UserID:      userID,
		SourceMsgID: userMessage.ID,
		Role:        ChatRoleAssistant,
		MsgType:     ChatMsgTypeText,
		Status:      ChatStatusGenerating,
	})
	if err != nil {
		return ChatResult{}, err
	}
	result := ChatResult{
		Message:                  message,
		UserMessageID:            userMessage.ID,
		UserMessagePublicID:      userMessage.PublicID,
		AssistantMessageID:       assistantMessage.ID,
		AssistantMessagePublicID: assistantMessage.PublicID,
	}
	if err := s.repo.CreateAgentRunStep(ctx, AgentRunStepInput{
		UserID:         userID,
		SourceMsgID:    userMessage.ID,
		AssistantMsgID: assistantMessage.ID,
		StepNo:         1,
		StepType:       AgentStepTypeModelDecision,
		Status:         AgentStepStatusSucceeded,
		DecisionLabel:  "draft",
		InputSummary:   message.Text,
	}); err != nil {
		return result, err
	}
	draft, err := s.repo.GetCurrentDraft(ctx, userID)
	if errors.Is(err, ErrDraftNotFound) {
		input := defaultCreateDraftInput(userID, message.Text)
		input.SourceMsgID = userMessage.ID
		draft, err = s.repo.CreateDraft(ctx, input)
	} else if err == nil {
		draft, err = s.repo.UpdateDraftSections(ctx, defaultUpdateDraftInput(userID, draft.PublicID, userMessage.ID, message.Text))
	}
	if err != nil {
		_, _ = s.repo.UpdateChatMessage(ctx, UpdateChatMessageInput{ID: assistantMessage.ID, Status: ChatStatusFailed, MsgType: ChatMsgTypeError, ContentText: "智能体请求失败"})
		return result, err
	}
	card := draftCard(draft)
	result.Draft = &card
	_, err = s.repo.UpdateChatMessage(ctx, UpdateChatMessageInput{
		ID:              assistantMessage.ID,
		Status:          ChatStatusSent,
		MsgType:         ChatMsgTypeDraftCard,
		ContentText:     message.Text,
		RelatedType:     "advice_draft",
		RelatedID:       draft.ID,
		RelatedPublicID: draft.PublicID,
	})
	if err != nil {
		return result, err
	}
	if err := s.repo.CreateAgentRunStep(ctx, AgentRunStepInput{
		UserID:          userID,
		SourceMsgID:     userMessage.ID,
		AssistantMsgID:  assistantMessage.ID,
		StepNo:          2,
		StepType:        AgentStepTypeFinalResponse,
		Status:          AgentStepStatusSucceeded,
		DecisionLabel:   "final_response",
		OutputSummary:   message.Text,
		RelatedType:     "advice_draft",
		RelatedID:       draft.ID,
		RelatedPublicID: draft.PublicID,
	}); err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) CurrentDraft(ctx context.Context, userID int64) (DraftCard, error) {
	if s == nil || s.repo == nil {
		return DraftCard{}, ErrRepositoryUnsupported
	}
	draft, err := s.repo.GetCurrentDraft(ctx, userID)
	if err != nil {
		return DraftCard{}, err
	}
	return draftCard(draft), nil
}

func (s *Service) ConfirmDraft(ctx context.Context, userID int64, publicID string) (Advice, error) {
	if s == nil || s.repo == nil {
		return Advice{}, ErrRepositoryUnsupported
	}
	return s.repo.ConfirmDraft(ctx, userID, strings.TrimSpace(publicID))
}

func (s *Service) DiscardDraft(ctx context.Context, userID int64, publicID string) error {
	if s == nil || s.repo == nil {
		return ErrRepositoryUnsupported
	}
	return s.repo.DiscardDraft(ctx, userID, strings.TrimSpace(publicID))
}

func draftCard(draft Draft) DraftCard {
	return DraftCard{
		DraftPublicID: draft.PublicID,
		RevisionNo:    draft.CurrentRevisionNo,
		Status:        draft.Status,
		SceneLabel:    draft.SceneLabel,
		Sections:      draft.Sections,
	}
}

func defaultCreateDraftInput(userID int64, text string) CreateDraftInput {
	return CreateDraftInput{
		UserID:          userID,
		SceneLabel:      text,
		UserIntent:      text,
		RevisionSummary: "创建初版建议草稿",
		Sections: []DraftSectionInput{
			{
				SectionType:          SectionTypeOutfit,
				ContentSchemaVersion: "v1",
				ContentJSON:          defaultAdviceContent("穿搭建议", "先给你一版可继续调整的穿搭方向。"),
				RevisionSummary:      "创建穿搭建议",
			},
			{
				SectionType:          SectionTypeHair,
				ContentSchemaVersion: "v1",
				ContentJSON:          defaultAdviceContent("发型建议", "先保持易执行、适合场景的发型方向。"),
				RevisionSummary:      "创建发型建议",
			},
			{
				SectionType:          SectionTypeMakeup,
				ContentSchemaVersion: "v1",
				ContentJSON:          defaultAdviceContent("妆容建议", "先给出低风险、可调整的妆容方向。"),
				RevisionSummary:      "创建妆容建议",
			},
		},
	}
}

func defaultUpdateDraftInput(userID int64, publicID string, sourceMsgID int64, text string) UpdateDraftInput {
	return UpdateDraftInput{
		UserID:          userID,
		PublicID:        publicID,
		SourceMsgID:     sourceMsgID,
		UserIntent:      text,
		RevisionSummary: "根据用户反馈精修草稿",
		Sections: []DraftSectionInput{{
			SectionType:          SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON:          defaultAdviceContent("穿搭建议调整", text),
			RevisionSummary:      "更新穿搭建议",
		}},
	}
}

func defaultAdviceContent(title string, summary string) map[string]any {
	return map[string]any{
		"title":            title,
		"summary":          summary,
		"why_text":         "这版用于先形成可讨论的草稿，后续可以按你的反馈精修。",
		"avoid_text":       "避免一次性做过多不可逆调整。",
		"alternative_text": "你可以继续说想改哪一项，例如鞋子、刘海、妆感浓淡。",
	}
}
