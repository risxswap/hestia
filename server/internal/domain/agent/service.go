package agent

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"hestia/server/internal/common/id"
	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/memory"
	"hestia/server/internal/domain/profile"
	serverlogger "hestia/server/internal/infra/logger"
)

const (
	defaultStreamStatusText    = "agent stream ready"
	defaultAdviceRunnerTimeout = 75 * time.Second
	runnerFallbackText         = "我暂时无法生成建议，请补充具体场景后再试。"
)

var (
	ErrRepositoryUnsupported = errors.New("agent repository unsupported")
	ErrDraftNotFound         = errors.New("advice draft not found")
	ErrDraftNotActive        = errors.New("advice draft is not active")
	ErrDraftIncomplete       = errors.New("advice draft incomplete")
	ErrDraftInvalid          = errors.New("advice draft invalid")
)

type ClothesAdviceService interface {
	AdviceContextItems(ctx context.Context, userID int64, filter clothes.AdviceContextFilter) ([]clothes.Item, error)
}

type ProfileContextService interface {
	Summary(ctx context.Context, userID int64) (profile.Summary, error)
}

type MemoryContextService interface {
	AgentMemoryContext(ctx context.Context, userID int64, query string, limit int) ([]memory.Item, error)
}

type AdviceToolDependencies struct {
	Clothes ClothesAdviceService
	Profile ProfileContextService
	Memory  MemoryContextService
}

type Repository interface {
	CreateChatMessage(ctx context.Context, input CreateChatMessageInput) (ChatMessage, error)
	UpdateChatMessage(ctx context.Context, input UpdateChatMessageInput) (ChatMessage, error)
	ListRecentChatMessages(ctx context.Context, userID int64, limit int) ([]ChatMessage, error)
	ListChatMessages(ctx context.Context, userID int64, limit int) ([]ChatMessage, error)
	ListAgentRunSteps(ctx context.Context, userID int64, assistantMessageIDs []int64) ([]AgentRunStep, error)
	CreateAgentRunStep(ctx context.Context, input AgentRunStepInput) error
	CreateDraft(ctx context.Context, input CreateDraftInput) (Draft, error)
	GetCurrentDraft(ctx context.Context, userID int64) (Draft, error)
	UpdateDraftSections(ctx context.Context, input UpdateDraftInput) (Draft, error)
	ListDraftVersions(ctx context.Context, userID int64, publicID string) ([]DraftRevision, error)
	ConfirmDraft(ctx context.Context, userID int64, publicID string) (Advice, error)
	DiscardDraft(ctx context.Context, userID int64, publicID string) error
}

func (s *Service) ChatHistory(ctx context.Context, userID int64, limit int) ([]ChatHistoryMessage, error) {
	if s == nil || s.repo == nil {
		return nil, ErrRepositoryUnsupported
	}
	messages, err := s.repo.ListChatMessages(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	assistantIDs := make([]int64, 0, len(messages))
	for _, message := range messages {
		if message.Role == ChatRoleAssistant {
			assistantIDs = append(assistantIDs, message.ID)
		}
	}
	steps, err := s.repo.ListAgentRunSteps(ctx, userID, assistantIDs)
	if err != nil {
		return nil, err
	}
	stepsByMessage := make(map[int64][]ChatProcessStep, len(assistantIDs))
	for _, step := range steps {
		stepsByMessage[step.AssistantMsgID] = append(stepsByMessage[step.AssistantMsgID], safeChatProcessStep(step))
	}
	history := make([]ChatHistoryMessage, 0, len(messages))
	for _, message := range messages {
		item := ChatHistoryMessage{
			PublicID:  message.PublicID,
			Role:      message.Role,
			MsgType:   message.MsgType,
			Content:   message.ContentText,
			Status:    message.Status,
			AssetRefs: message.AssetRefs,
			CreatedAt: message.CreatedAt,
		}
		if message.Role == ChatRoleAssistant {
			item.Process = chatProcessForMessage(message, stepsByMessage[message.ID])
		}
		history = append(history, item)
	}
	return history, nil
}

func chatProcessForMessage(message ChatMessage, steps []ChatProcessStep) *ChatProcess {
	process := &ChatProcess{
		Status:            AgentStepStatusSucceeded,
		Summary:           "查看处理过程",
		ResponseStartedAt: message.CreatedAt,
		Steps:             steps,
	}
	if message.Status == ChatStatusGenerating {
		process.Status = "running"
		process.Summary = "理解你的需求"
		return process
	}
	if message.Status == ChatStatusFailed {
		process.Status = AgentStepStatusFailed
		process.Summary = "本轮处理未完成"
	}
	for _, step := range steps {
		if step.FinishedAt != nil {
			process.FinishedAt = step.FinishedAt
		}
	}
	return process
}

func safeChatProcessStep(step AgentRunStep) ChatProcessStep {
	summary, detail := safeProcessCopy(step.StepType, step.ToolName, step.Status)
	return ChatProcessStep{
		StepNo:     step.StepNo,
		Status:     step.Status,
		Summary:    summary,
		Detail:     detail,
		StartedAt:  step.StartedAt,
		FinishedAt: optionalTime(step.FinishedAt),
	}
}

func safeProcessCopy(stepType, toolName, status string) (string, string) {
	if status == AgentStepStatusFailed {
		return "本轮处理未完成", "可以补充具体场景后再试"
	}
	switch toolName {
	case AdviceToolCreateDraft:
		return "生成建议草稿", "已创建一版可继续调整的建议草稿"
	case AdviceToolUpdateDraft:
		return "调整建议草稿", "已更新当前建议草稿"
	}
	switch stepType {
	case AgentStepTypeModelDecision:
		return "理解你的需求", "已结合本轮场景与已有信息"
	case AgentStepTypeToolCall, AgentStepTypeToolResult:
		return "整理建议内容", "正在更新可执行建议"
	case AgentStepTypeFinalResponse:
		return "完成回复", "已整理本轮建议"
	default:
		return "整理本轮建议", "已处理本轮请求"
	}
}

type Service struct {
	clothes       ClothesAdviceService
	repo          Repository
	runner        AdviceRunner
	logger        *slog.Logger
	runnerTimeout time.Duration
}

func NewService() *Service {
	return &Service{runnerTimeout: defaultAdviceRunnerTimeout}
}

func NewServiceWithClothes(clothesService ClothesAdviceService) *Service {
	service := NewService()
	service.clothes = clothesService
	return service
}

func NewServiceWithRepository(repo Repository) *Service {
	service := NewService()
	service.repo = repo
	return service
}

func NewServiceWithDependencies(repo Repository, clothesService ClothesAdviceService) *Service {
	service := NewService()
	service.repo = repo
	service.clothes = clothesService
	return service
}

func NewServiceWithRunner(repo Repository, clothesService ClothesAdviceService, runner AdviceRunner) *Service {
	service := NewServiceWithDependencies(repo, clothesService)
	service.runner = runner
	return service
}

func NewServiceWithRunnerTimeout(repo Repository, clothesService ClothesAdviceService, runner AdviceRunner, timeout time.Duration) *Service {
	service := NewServiceWithRunner(repo, clothesService, runner)
	service.SetRunnerTimeout(timeout)
	return service
}

func (s *Service) SetAdviceRunner(runner AdviceRunner) {
	if s == nil {
		return
	}
	s.runner = runner
}

func (s *Service) SetRunnerTimeout(timeout time.Duration) {
	if s != nil && timeout > 0 {
		s.runnerTimeout = timeout
	}
}

func (s *Service) SetLogger(logger *slog.Logger) {
	if s == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	s.logger = logger
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

func (s *Service) Chat(ctx context.Context, userID int64, text string, assetRefs ...ChatAssetRef) (ChatResult, error) {
	return s.chat(ctx, userID, text, nil, assetRefs...)
}

func (s *Service) ChatWithProcessEvents(ctx context.Context, userID int64, text string, emit func(ChatProcessEvent), assetRefs ...ChatAssetRef) (ChatResult, error) {
	return s.chat(ctx, userID, text, emit, assetRefs...)
}

func (s *Service) chat(ctx context.Context, userID int64, text string, emit func(ChatProcessEvent), assetRefs ...ChatAssetRef) (ChatResult, error) {
	message := StreamMessage{Text: strings.TrimSpace(text)}
	if message.Text == "" {
		message.Text = "请告诉我想咨询的场景或想解决的问题。"
	}
	assetRefs = normalizeChatAssetRefs(assetRefs)
	if s == nil || s.repo == nil {
		return ChatResult{Message: message}, nil
	}
	userMessage, err := s.repo.CreateChatMessage(ctx, CreateChatMessageInput{
		UserID:      userID,
		Role:        ChatRoleUser,
		MsgType:     ChatMsgTypeText,
		ContentText: message.Text,
		AssetRefs:   assetRefs,
		Status:      ChatStatusSent,
	})
	if err != nil {
		return ChatResult{}, err
	}
	responseStartedAt := time.Now().UTC()
	assistantMessage, err := s.repo.CreateChatMessage(ctx, CreateChatMessageInput{
		UserID:      userID,
		SourceMsgID: userMessage.ID,
		Role:        ChatRoleAssistant,
		MsgType:     ChatMsgTypeText,
		Status:      ChatStatusGenerating,
		CreatedAt:   responseStartedAt,
	})
	if err != nil {
		return ChatResult{}, err
	}
	result := ChatResult{
		UserMessageID:            userMessage.ID,
		UserMessagePublicID:      userMessage.PublicID,
		AssistantMessageID:       assistantMessage.ID,
		AssistantMessagePublicID: assistantMessage.PublicID,
		ResponseStartedAt:        assistantMessage.CreatedAt,
	}
	if result.ResponseStartedAt.IsZero() {
		result.ResponseStartedAt = responseStartedAt
	}
	emitChatProcess(emit, ChatProcessEvent{
		StepNo:            1,
		Status:            "running",
		Summary:           "理解你的需求",
		Detail:            "正在结合本轮场景与已有信息",
		ResponseStartedAt: result.ResponseStartedAt,
	})
	currentDraft, err := s.repo.GetCurrentDraft(ctx, userID)
	var currentDraftPtr *Draft
	if errors.Is(err, ErrDraftNotFound) {
		err = nil
	} else if err == nil {
		currentDraftPtr = &currentDraft
	}
	if err != nil {
		emitChatProcess(emit, failedChatProcessEvent(1, result.ResponseStartedAt))
		_, _ = s.repo.UpdateChatMessage(ctx, UpdateChatMessageInput{ID: assistantMessage.ID, Status: ChatStatusFailed, MsgType: ChatMsgTypeError, ContentText: "智能体请求失败"})
		return result, err
	}
	recentMessages, err := s.repo.ListRecentChatMessages(ctx, userID, 12)
	if err != nil {
		now := time.Now().UTC()
		_ = s.recordFailedStep(ctx, AgentRunStepInput{
			UserID:         userID,
			SourceMsgID:    userMessage.ID,
			AssistantMsgID: assistantMessage.ID,
			StepNo:         1,
			StepType:       AgentStepTypeModelDecision,
			DecisionLabel:  "chat_history_failed",
			InputSummary:   message.Text,
			StartedAt:      now,
			FinishedAt:     now,
			DurationMS:     1,
		}, err)
		_, _ = s.repo.UpdateChatMessage(ctx, UpdateChatMessageInput{ID: assistantMessage.ID, Status: ChatStatusFailed, MsgType: ChatMsgTypeError, ContentText: "智能体请求失败"})
		emitChatProcess(emit, failedChatProcessEvent(1, result.ResponseStartedAt))
		return result, err
	}
	runnerStartedAt := time.Now().UTC()
	runnerLog := s.logger
	if runnerLog == nil {
		runnerLog = slog.Default()
	}
	llmCallID := id.NewPublicID("llm")
	runnerAttrs := []any{
		"request_id", serverlogger.RequestID(ctx),
		"llm_call_id", llmCallID,
		"user_id", userID,
		"input_chars", len([]rune(message.Text)),
		"image_count", len(assetRefs),
		"input_summary", serverlogger.SanitizeSummary(message.Text),
	}
	runnerLog.InfoContext(ctx, "agent llm call started", runnerAttrs...)
	runnerInput := AdviceRunInput{
		UserID:         userID,
		Text:           message.Text,
		SourceMsgID:    userMessage.ID,
		AssetRefs:      assetRefs,
		RecentMessages: recentMessages,
		CurrentDraft:   currentDraftPtr,
	}
	output := AdviceRunOutput{}
	var runnerErr error
	runnerMetadata := AdviceRunMetadata{AgentTimeoutMS: s.runnerTimeout.Milliseconds()}
	if provider, ok := s.runner.(interface{ Metadata() AdviceRunMetadata }); ok {
		runnerMetadata = provider.Metadata()
		if runnerMetadata.AgentTimeoutMS <= 0 {
			runnerMetadata.AgentTimeoutMS = s.runnerTimeout.Milliseconds()
		}
	}
	var errorDiagnostics agentErrorDiagnostics
	if s.runner == nil {
		runnerErr = ErrAdviceRunnerUnavailable
		errorDiagnostics = diagnoseAgentError(ctx, runnerErr, runnerMetadata)
	} else {
		runnerCtx, cancelRunner := context.WithTimeout(ctx, s.runnerTimeout)
		output, runnerErr = s.runner.Run(runnerCtx, runnerInput)
		if runnerErr != nil {
			errorDiagnostics = diagnoseAgentError(runnerCtx, runnerErr, runnerMetadata)
		}
		cancelRunner()
	}
	runnerFinishedAt := time.Now().UTC()
	runnerDuration := durationMS(runnerStartedAt, runnerFinishedAt)
	stepOffset := 0
	if runnerErr != nil {
		errorAttrs := append([]any{}, runnerAttrs...)
		errorAttrs = append(errorAttrs,
			"duration_ms", runnerDuration,
			"error_stage", "agent_run",
			"error", safeAgentErrorSummary(runnerErr),
		)
		errorAttrs = append(errorAttrs, errorDiagnostics.logAttrs()...)
		runnerLog.ErrorContext(ctx, "agent llm call failed", errorAttrs...)
		_ = s.recordFailedStep(ctx, AgentRunStepInput{
			UserID:         userID,
			SourceMsgID:    userMessage.ID,
			AssistantMsgID: assistantMessage.ID,
			StepNo:         1,
			StepType:       AgentStepTypeModelDecision,
			DecisionLabel:  "runner_failed",
			InputSummary:   message.Text,
			StartedAt:      runnerStartedAt,
			FinishedAt:     runnerFinishedAt,
			DurationMS:     durationMS(runnerStartedAt, runnerFinishedAt),
		}, runnerErr)
		output = AdviceRunOutput{
			AssistantText: runnerFallbackText,
			DecisionLabel: "runner_fallback_text",
		}
		stepOffset = 1
		emitChatProcess(emit, failedChatProcessEvent(1, result.ResponseStartedAt))
	} else {
		successAttrs := append([]any{}, runnerAttrs...)
		successAttrs = append(successAttrs,
			"usage_key", output.Metadata.UsageKey,
			"provider_code", output.Metadata.ProviderCode,
			"model_code", output.Metadata.ModelCode,
			"prompt_version", output.Metadata.PromptVersion,
			"duration_ms", runnerDuration,
			"output_chars", len([]rune(output.AssistantText)),
			"output_summary", serverlogger.SanitizeSummary(output.AssistantText),
		)
		runnerLog.InfoContext(ctx, "agent llm call completed", successAttrs...)
		emitChatProcess(emit, ChatProcessEvent{
			StepNo:            1,
			Status:            AgentStepStatusSucceeded,
			Summary:           "理解你的需求",
			Detail:            "已结合本轮场景与已有信息",
			ResponseStartedAt: result.ResponseStartedAt,
			FinishedAt:        optionalTime(runnerFinishedAt),
		})
	}
	if strings.TrimSpace(output.AssistantText) == "" {
		output.AssistantText = "请告诉我具体场景、已有单品或想调整的方向。"
	}
	result.Message = StreamMessage{Text: output.AssistantText}
	stepNo := 1 + stepOffset
	if err := s.repo.CreateAgentRunStep(ctx, AgentRunStepInput{
		UserID:         userID,
		SourceMsgID:    userMessage.ID,
		AssistantMsgID: assistantMessage.ID,
		StepNo:         stepNo,
		StepType:       AgentStepTypeModelDecision,
		Status:         AgentStepStatusSucceeded,
		UsageKey:       output.Metadata.UsageKey,
		ProviderCode:   output.Metadata.ProviderCode,
		ModelCode:      output.Metadata.ModelCode,
		PromptVersion:  output.Metadata.PromptVersion,
		MaxIterations:  output.Metadata.MaxIterations,
		DecisionLabel:  firstNonEmpty(output.DecisionLabel, "chat_response"),
		InputSummary:   message.Text,
		OutputSummary:  output.AssistantText,
		StartedAt:      runnerStartedAt,
		FinishedAt:     runnerFinishedAt,
		DurationMS:     durationMS(runnerStartedAt, runnerFinishedAt),
	}); err != nil {
		return result, err
	}
	var draft Draft
	draftUpdated := false
	if hasCompletedAdviceDraftAudit(output.AuditSteps) {
		refreshedDraft, refreshErr := s.repo.GetCurrentDraft(ctx, userID)
		if refreshErr != nil {
			now := time.Now().UTC()
			_ = s.recordFailedStep(ctx, AgentRunStepInput{
				UserID:         userID,
				SourceMsgID:    userMessage.ID,
				AssistantMsgID: assistantMessage.ID,
				StepNo:         stepNo + 1,
				StepType:       AgentStepTypeToolResult,
				DecisionLabel:  "refresh_audited_draft_failed",
				InputSummary:   message.Text,
				StartedAt:      now,
				FinishedAt:     now,
				DurationMS:     1,
			}, refreshErr)
			_, _ = s.repo.UpdateChatMessage(ctx, UpdateChatMessageInput{ID: assistantMessage.ID, Status: ChatStatusFailed, MsgType: ChatMsgTypeError, ContentText: "智能体请求失败"})
			return result, refreshErr
		}
		draft = refreshedDraft
		currentDraftPtr = &draft
		draftUpdated = true
	}
	for _, auditStep := range output.AuditSteps {
		stepNo++
		if err := s.repo.CreateAgentRunStep(ctx, agentRunStepFromAudit(userID, userMessage.ID, assistantMessage.ID, stepNo, auditStep)); err != nil {
			return result, err
		}
		summary, detail := safeProcessCopy(auditStep.StepType, auditStep.ToolName, auditStep.Status)
		emitChatProcess(emit, ChatProcessEvent{
			StepNo:            stepNo,
			Status:            firstNonEmpty(auditStep.Status, AgentStepStatusSucceeded),
			Summary:           summary,
			Detail:            detail,
			ResponseStartedAt: result.ResponseStartedAt,
			FinishedAt:        optionalTime(time.Now().UTC()),
		})
	}
	for _, call := range output.ToolCalls {
		stepNo++
		toolStartedAt := time.Now().UTC()
		summary, detail := safeProcessCopy(AgentStepTypeToolCall, call.Name, "running")
		emitChatProcess(emit, ChatProcessEvent{
			StepNo:            stepNo,
			Status:            "running",
			Summary:           summary,
			Detail:            detail,
			ResponseStartedAt: result.ResponseStartedAt,
		})
		if err := s.repo.CreateAgentRunStep(ctx, AgentRunStepInput{
			UserID:         userID,
			SourceMsgID:    userMessage.ID,
			AssistantMsgID: assistantMessage.ID,
			StepNo:         stepNo,
			StepType:       AgentStepTypeToolCall,
			Status:         AgentStepStatusSucceeded,
			ToolName:       call.Name,
			ToolCallID:     call.ToolCallID,
			DecisionLabel:  call.Name,
			InputSummary:   call.InputSummary,
			StartedAt:      toolStartedAt,
		}); err != nil {
			return result, err
		}
		draft, err = s.executeAdviceToolCall(ctx, userID, userMessage.ID, currentDraftPtr, call)
		toolFinishedAt := time.Now().UTC()
		if err != nil {
			stepNo++
			_ = s.recordFailedStep(ctx, AgentRunStepInput{
				UserID:         userID,
				SourceMsgID:    userMessage.ID,
				AssistantMsgID: assistantMessage.ID,
				StepNo:         stepNo,
				StepType:       AgentStepTypeToolResult,
				ToolName:       call.Name,
				ToolCallID:     call.ToolCallID,
				DecisionLabel:  call.Name,
				InputSummary:   call.InputSummary,
				StartedAt:      toolStartedAt,
				FinishedAt:     toolFinishedAt,
				DurationMS:     durationMS(toolStartedAt, toolFinishedAt),
			}, err)
			emitChatProcess(emit, failedChatProcessEvent(stepNo, result.ResponseStartedAt))
			break
		}
		currentDraft = draft
		currentDraftPtr = &currentDraft
		draftUpdated = true
		stepNo++
		if err := s.repo.CreateAgentRunStep(ctx, AgentRunStepInput{
			UserID:          userID,
			SourceMsgID:     userMessage.ID,
			AssistantMsgID:  assistantMessage.ID,
			StepNo:          stepNo,
			StepType:        AgentStepTypeToolResult,
			Status:          AgentStepStatusSucceeded,
			ToolName:        call.Name,
			ToolCallID:      call.ToolCallID,
			DecisionLabel:   call.Name,
			OutputSummary:   "草稿已更新",
			RelatedType:     "advice_draft",
			RelatedID:       draft.ID,
			RelatedPublicID: draft.PublicID,
			StartedAt:       toolStartedAt,
			FinishedAt:      toolFinishedAt,
			DurationMS:      durationMS(toolStartedAt, toolFinishedAt),
		}); err != nil {
			return result, err
		}
		summary, detail = safeProcessCopy(AgentStepTypeToolResult, call.Name, AgentStepStatusSucceeded)
		emitChatProcess(emit, ChatProcessEvent{
			StepNo:            stepNo,
			Status:            AgentStepStatusSucceeded,
			Summary:           summary,
			Detail:            detail,
			ResponseStartedAt: result.ResponseStartedAt,
			FinishedAt:        optionalTime(toolFinishedAt),
		})
	}
	if err != nil {
		_, _ = s.repo.UpdateChatMessage(ctx, UpdateChatMessageInput{ID: assistantMessage.ID, Status: ChatStatusFailed, MsgType: ChatMsgTypeError, ContentText: "智能体请求失败"})
		return result, err
	}
	messageUpdate := UpdateChatMessageInput{
		ID:          assistantMessage.ID,
		Status:      ChatStatusSent,
		MsgType:     ChatMsgTypeText,
		ContentText: output.AssistantText,
	}
	finishedAt := time.Now().UTC()
	finalStep := AgentRunStepInput{
		UserID:         userID,
		SourceMsgID:    userMessage.ID,
		AssistantMsgID: assistantMessage.ID,
		StepNo:         stepNo + 1,
		StepType:       AgentStepTypeFinalResponse,
		Status:         AgentStepStatusSucceeded,
		DecisionLabel:  "final_response",
		OutputSummary:  output.AssistantText,
		StartedAt:      finishedAt,
		FinishedAt:     finishedAt,
		DurationMS:     1,
	}
	if draftUpdated {
		card := draftCard(draft)
		result.Draft = &card
		messageUpdate.MsgType = ChatMsgTypeDraftCard
		messageUpdate.RelatedType = "advice_draft"
		messageUpdate.RelatedID = draft.ID
		messageUpdate.RelatedPublicID = draft.PublicID
		finalStep.RelatedType = "advice_draft"
		finalStep.RelatedID = draft.ID
		finalStep.RelatedPublicID = draft.PublicID
	}
	_, err = s.repo.UpdateChatMessage(ctx, messageUpdate)
	if err != nil {
		return result, err
	}
	if err := s.repo.CreateAgentRunStep(ctx, finalStep); err != nil {
		return result, err
	}
	result.FinishedAt = finishedAt
	emitChatProcess(emit, ChatProcessEvent{
		StepNo:            finalStep.StepNo,
		Status:            AgentStepStatusSucceeded,
		Summary:           "完成回复",
		Detail:            "已整理本轮建议",
		ResponseStartedAt: result.ResponseStartedAt,
		FinishedAt:        optionalTime(finishedAt),
	})
	return result, nil
}

func emitChatProcess(emit func(ChatProcessEvent), event ChatProcessEvent) {
	if emit != nil {
		emit(event)
	}
}

func failedChatProcessEvent(stepNo int, responseStartedAt time.Time) ChatProcessEvent {
	return ChatProcessEvent{
		StepNo:            stepNo,
		Status:            AgentStepStatusFailed,
		Summary:           "本轮处理未完成",
		Detail:            "可以补充具体场景后再试",
		ResponseStartedAt: responseStartedAt,
		FinishedAt:        optionalTime(time.Now().UTC()),
	}
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value
	return &copy
}

func (s *Service) recordFailedStep(ctx context.Context, input AgentRunStepInput, stepErr error) error {
	if s == nil || s.repo == nil {
		return ErrRepositoryUnsupported
	}
	errorText := ""
	if stepErr != nil {
		errorText = stepErr.Error()
	}
	input.Status = AgentStepStatusFailed
	input.ErrorMessage = errorText
	return s.repo.CreateAgentRunStep(ctx, input)
}

func durationMS(startedAt, finishedAt time.Time) int {
	if startedAt.IsZero() || finishedAt.IsZero() || finishedAt.Before(startedAt) {
		return 1
	}
	elapsed := finishedAt.Sub(startedAt).Milliseconds()
	if elapsed <= 0 {
		return 1
	}
	return int(elapsed)
}

func hasCompletedAdviceDraftAudit(steps []AdviceRunAuditStep) bool {
	for _, step := range steps {
		if step.StepType != AgentStepTypeToolResult || step.Status != AgentStepStatusSucceeded {
			continue
		}
		if step.ToolName == AdviceToolCreateDraft || step.ToolName == AdviceToolUpdateDraft {
			return true
		}
	}
	return false
}

func agentRunStepFromAudit(userID, sourceMsgID, assistantMsgID int64, stepNo int, auditStep AdviceRunAuditStep) AgentRunStepInput {
	status := auditStep.Status
	if strings.TrimSpace(status) == "" {
		status = AgentStepStatusSucceeded
	}
	stepType := auditStep.StepType
	if strings.TrimSpace(stepType) == "" {
		stepType = AgentStepTypeToolCall
	}
	duration := auditStep.DurationMS
	if duration <= 0 {
		duration = 1
	}
	return AgentRunStepInput{
		UserID:          userID,
		SourceMsgID:     sourceMsgID,
		AssistantMsgID:  assistantMsgID,
		StepNo:          stepNo,
		StepType:        stepType,
		Status:          status,
		ToolName:        auditStep.ToolName,
		ToolCallID:      auditStep.ToolCallID,
		DecisionLabel:   firstNonEmpty(auditStep.DecisionLabel, auditStep.ToolName),
		InputSummary:    auditStep.InputSummary,
		OutputSummary:   auditStep.OutputSummary,
		RelatedType:     auditStep.RelatedType,
		RelatedID:       auditStep.RelatedID,
		RelatedPublicID: auditStep.RelatedPublicID,
		DurationMS:      duration,
		ErrorMessage:    auditStep.ErrorMessage,
	}
}

func (s *Service) executeAdviceToolCall(ctx context.Context, userID, sourceMsgID int64, currentDraft *Draft, call AdviceToolCall) (Draft, error) {
	switch call.Name {
	case AdviceToolCreateDraft:
		if call.CreateDraftInput == nil {
			return Draft{}, ErrDraftNotFound
		}
		input := *call.CreateDraftInput
		input.UserID = userID
		input.SourceMsgID = sourceMsgID
		return s.repo.CreateDraft(ctx, input)
	case AdviceToolUpdateDraft:
		if call.UpdateDraftInput == nil {
			return Draft{}, ErrDraftNotFound
		}
		input := *call.UpdateDraftInput
		input.UserID = userID
		input.SourceMsgID = sourceMsgID
		if input.PublicID == "" && currentDraft != nil {
			input.PublicID = currentDraft.PublicID
		}
		return s.repo.UpdateDraftSections(ctx, input)
	default:
		return Draft{}, ErrDraftNotFound
	}
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

func (s *Service) DraftVersions(ctx context.Context, userID int64, publicID string) ([]DraftRevision, error) {
	if s == nil || s.repo == nil {
		return nil, ErrRepositoryUnsupported
	}
	return s.repo.ListDraftVersions(ctx, userID, strings.TrimSpace(publicID))
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

func normalizeChatAssetRefs(refs []ChatAssetRef) []ChatAssetRef {
	seen := map[string]bool{}
	normalized := make([]ChatAssetRef, 0, len(refs))
	for _, ref := range refs {
		publicID := strings.TrimSpace(ref.AssetPublicID)
		if publicID == "" || seen[publicID] {
			continue
		}
		seen[publicID] = true
		normalized = append(normalized, ChatAssetRef{
			AssetPublicID: publicID,
			AssetType:     strings.TrimSpace(ref.AssetType),
			Note:          strings.TrimSpace(ref.Note),
		})
		if len(normalized) >= 6 {
			break
		}
	}
	return normalized
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
