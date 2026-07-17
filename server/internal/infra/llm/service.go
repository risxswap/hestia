package llm

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"hestia/server/internal/common/id"
	serverlogger "hestia/server/internal/infra/logger"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Message struct {
	Role    string
	Content string
}

type Request struct {
	UsageKey     string
	RequiredCaps []string
	Messages     []Message
	ImageURLs    []string
	Params       map[string]any
}

type Response struct {
	Text  string
	Usage ResolvedUsage
}

type Generator interface {
	Generate(ctx context.Context, request Request) (Response, error)
}

const defaultRequestTimeout = 60 * time.Second

type Service struct {
	resolver       *ConfigResolver
	client         Client
	logger         *slog.Logger
	requestTimeout time.Duration
}

func NewService(resolver *ConfigResolver, client Client) *Service {
	return NewServiceWithTimeout(resolver, client, defaultRequestTimeout)
}

func NewServiceWithTimeout(resolver *ConfigResolver, client Client, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	return &Service{resolver: resolver, client: client, logger: slog.Default(), requestTimeout: timeout}
}

func (s *Service) SetLogger(logger *slog.Logger) {
	if s == nil {
		return
	}
	if logger == nil {
		s.logger = slog.Default()
		return
	}
	s.logger = logger
}

func (s *Service) Generate(ctx context.Context, request Request) (Response, error) {
	startedAt := time.Now()
	attrs := requestLogAttrs(ctx, id.NewPublicID("llm"), request)
	if s == nil || s.client == nil {
		s.logCallError(ctx, startedAt, attrs, "client_check", ErrClientUnavailable)
		return Response{}, ErrClientUnavailable
	}
	resolved, err := s.resolver.ResolveUsage(ctx, request.UsageKey, request.RequiredCaps)
	if err != nil {
		s.logCallError(ctx, startedAt, attrs, "config_resolve", err)
		return Response{}, err
	}
	attrs = append(attrs, resolvedLogAttrs(resolved)...)
	if s.logger != nil {
		s.logger.InfoContext(ctx, "llm call started", attrs...)
	}
	chatModel, err := s.newChatModel(ctx, resolved, request)
	if err != nil {
		s.logCallError(ctx, startedAt, attrs, "model_init", err)
		return Response{}, err
	}
	if chatModel == nil {
		s.logCallError(ctx, startedAt, attrs, "model_init", ErrClientUnavailable)
		return Response{}, ErrClientUnavailable
	}
	message, err := chatModel.Generate(ctx, einoMessages(request))
	if err != nil {
		s.logCallError(ctx, startedAt, attrs, "model_generate", err)
		return Response{}, err
	}
	text := ""
	if message != nil {
		text = message.Content
	}
	if s.logger != nil {
		successAttrs := append([]any{}, attrs...)
		successAttrs = append(successAttrs,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"output_chars", len([]rune(text)),
			"output_summary", serverlogger.SanitizeSummary(text),
		)
		s.logger.InfoContext(ctx, "llm call completed", successAttrs...)
	}
	return Response{Text: text, Usage: resolved}, nil
}

func (s *Service) NewToolCallingChatModel(ctx context.Context, request Request) (einomodel.ToolCallingChatModel, error) {
	chatModel, _, err := s.NewToolCallingChatModelWithUsage(ctx, request)
	return chatModel, err
}

func (s *Service) NewToolCallingChatModelWithUsage(ctx context.Context, request Request) (einomodel.ToolCallingChatModel, ResolvedUsage, error) {
	startedAt := time.Now()
	attrs := requestLogAttrs(ctx, id.NewPublicID("llm"), request)
	if s == nil || s.client == nil {
		s.logCallError(ctx, startedAt, attrs, "client_check", ErrClientUnavailable)
		return nil, ResolvedUsage{}, ErrClientUnavailable
	}
	resolved, err := s.resolver.ResolveUsage(ctx, request.UsageKey, request.RequiredCaps)
	if err != nil {
		s.logCallError(ctx, startedAt, attrs, "config_resolve", err)
		return nil, ResolvedUsage{}, err
	}
	attrs = append(attrs, resolvedLogAttrs(resolved)...)
	if s.logger != nil {
		s.logger.InfoContext(ctx, "llm model init started", attrs...)
	}
	var chatModel einomodel.ToolCallingChatModel
	if toolClient, ok := s.client.(ToolCallingClient); ok {
		switch strings.ToLower(strings.TrimSpace(resolved.Provider.Code)) {
		case "qwen":
			chatModel, err = toolClient.NewQwenToolCallingChatModel(ctx, qwenChatModelConfig(resolved, request, s.requestTimeout))
		case "siliconflow", "openai":
			chatModel, err = toolClient.NewOpenAIToolCallingChatModel(ctx, openAIChatModelConfig(resolved, request, s.requestTimeout))
		default:
			err = fmt.Errorf("unsupported llm provider %q", resolved.Provider.Code)
		}
	} else {
		var baseModel EinoChatModel
		baseModel, err = s.newChatModel(ctx, resolved, request)
		if err == nil {
			chatModel, _ = baseModel.(einomodel.ToolCallingChatModel)
			if chatModel == nil {
				err = ErrClientUnavailable
			}
		}
	}
	if err != nil {
		s.logCallError(ctx, startedAt, attrs, "model_init", err)
		return nil, resolved, err
	}
	if chatModel == nil {
		s.logCallError(ctx, startedAt, attrs, "model_init", ErrClientUnavailable)
		return nil, resolved, ErrClientUnavailable
	}
	if s.logger != nil {
		successAttrs := append([]any{}, attrs...)
		successAttrs = append(successAttrs, "duration_ms", time.Since(startedAt).Milliseconds())
		s.logger.InfoContext(ctx, "llm model init completed", successAttrs...)
	}
	return chatModel, resolved, nil
}

func requestLogAttrs(ctx context.Context, callID string, request Request) []any {
	roleCounts := map[string]int{}
	inputChars := 0
	contents := make([]string, 0, len(request.Messages))
	for _, message := range request.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		roleCounts[role]++
		inputChars += len([]rune(message.Content))
		if content := strings.TrimSpace(message.Content); content != "" {
			contents = append(contents, content)
		}
	}
	paramKeys := make([]string, 0, len(request.Params))
	for key := range request.Params {
		if key = strings.TrimSpace(key); key != "" {
			paramKeys = append(paramKeys, key)
		}
	}
	sort.Strings(paramKeys)
	return []any{
		"request_id", serverlogger.RequestID(ctx),
		"llm_call_id", callID,
		"usage_key", request.UsageKey,
		"required_caps", strings.Join(trimStringValues(request.RequiredCaps), ","),
		"message_count", len(request.Messages),
		"system_message_count", roleCounts["system"],
		"user_message_count", roleCounts["user"],
		"assistant_message_count", roleCounts["assistant"],
		"tool_message_count", roleCounts["tool"],
		"input_chars", inputChars,
		"image_count", len(trimStringValues(request.ImageURLs)),
		"param_keys", strings.Join(paramKeys, ","),
		"input_summary", serverlogger.SanitizeSummary(strings.Join(contents, " | ")),
	}
}

func resolvedLogAttrs(resolved ResolvedUsage) []any {
	return []any{
		"provider_code", resolved.Provider.Code,
		"model_code", resolved.Model.ModelCode,
		"prompt_version", resolved.Usage.PromptVersion,
	}
}

func (s *Service) logCallError(ctx context.Context, startedAt time.Time, attrs []any, stage string, err error) {
	if s == nil || s.logger == nil {
		slog.Default().ErrorContext(ctx, "llm call failed", append(append([]any{}, attrs...),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error_stage", stage,
			"error", serverlogger.ErrorSummary(err),
		)...)
		return
	}
	errorAttrs := append([]any{}, attrs...)
	errorAttrs = append(errorAttrs,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"error_stage", stage,
		"error", serverlogger.ErrorSummary(err),
	)
	s.logger.ErrorContext(ctx, "llm call failed", errorAttrs...)
}

func (s *Service) newChatModel(ctx context.Context, resolved ResolvedUsage, request Request) (EinoChatModel, error) {
	switch strings.ToLower(strings.TrimSpace(resolved.Provider.Code)) {
	case "qwen":
		return s.client.NewQwenChatModel(ctx, qwenChatModelConfig(resolved, request, s.requestTimeout))
	case "siliconflow", "openai":
		return s.client.NewOpenAIChatModel(ctx, openAIChatModelConfig(resolved, request, s.requestTimeout))
	default:
		return nil, fmt.Errorf("unsupported llm provider %q", resolved.Provider.Code)
	}
}

func qwenChatModelConfig(resolved ResolvedUsage, request Request, timeout time.Duration) *qwen.ChatModelConfig {
	params := mergedParams(resolved.Usage.Params, request.Params)
	config := &qwen.ChatModelConfig{
		BaseURL: strings.TrimSpace(resolved.Provider.APIBaseURL),
		APIKey:  strings.TrimSpace(resolved.Provider.Token),
		Model:   strings.TrimSpace(resolved.Model.ModelCode),
		Timeout: timeout,
	}
	if maxTokens, ok := intParam(params, "max_tokens"); ok {
		config.MaxTokens = &maxTokens
	}
	if temperature, ok := float32Param(params, "temperature"); ok {
		config.Temperature = &temperature
	}
	if topP, ok := float32Param(params, "top_p"); ok {
		config.TopP = &topP
	}
	if responseFormat := strings.TrimSpace(fmt.Sprint(params["response_format"])); responseFormat != "" && responseFormat != "<nil>" {
		config.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatType(responseFormat),
		}
	}
	return config
}

func openAIChatModelConfig(resolved ResolvedUsage, request Request, timeout time.Duration) *einoopenai.ChatModelConfig {
	params := mergedParams(resolved.Usage.Params, request.Params)
	config := &einoopenai.ChatModelConfig{
		BaseURL: strings.TrimSpace(resolved.Provider.APIBaseURL),
		APIKey:  strings.TrimSpace(resolved.Provider.Token),
		Model:   strings.TrimSpace(resolved.Model.ModelCode),
		Timeout: timeout,
	}
	if maxTokens, ok := intParam(params, "max_tokens"); ok {
		config.MaxTokens = &maxTokens
	}
	if temperature, ok := float32Param(params, "temperature"); ok {
		config.Temperature = &temperature
	}
	if topP, ok := float32Param(params, "top_p"); ok {
		config.TopP = &topP
	}
	if responseFormat := strings.TrimSpace(fmt.Sprint(params["response_format"])); responseFormat != "" && responseFormat != "<nil>" {
		config.ResponseFormat = &einoopenai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatType(responseFormat),
		}
	}
	return config
}

func einoMessages(request Request) []*schema.Message {
	messages := make([]*schema.Message, 0, len(request.Messages))
	imageURLs := trimStringValues(request.ImageURLs)
	imageAttached := false
	for _, message := range request.Messages {
		role := einoRole(message.Role)
		content := strings.TrimSpace(message.Content)
		if role == schema.User && len(imageURLs) > 0 && !imageAttached {
			messages = append(messages, userMultiContentMessage(content, imageURLs))
			imageAttached = true
			continue
		}
		messages = append(messages, &schema.Message{
			Role:    role,
			Content: content,
		})
	}
	if len(messages) == 0 && len(imageURLs) > 0 {
		messages = append(messages, userMultiContentMessage("", imageURLs))
	}
	return messages
}

func userMultiContentMessage(content string, imageURLs []string) *schema.Message {
	parts := make([]schema.MessageInputPart, 0, 1+len(imageURLs))
	if content != "" {
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: content,
		})
	}
	for _, imageURL := range imageURLs {
		url := imageURL
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL: &url,
				},
				Detail: schema.ImageURLDetailAuto,
			},
		})
	}
	return &schema.Message{
		Role:                  schema.User,
		UserInputMultiContent: parts,
	}
}

func einoRole(role string) schema.RoleType {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system":
		return schema.System
	case "assistant":
		return schema.Assistant
	case "tool":
		return schema.Tool
	default:
		return schema.User
	}
}

func mergedParams(base map[string]any, override map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range base {
		key = strings.TrimSpace(key)
		if key != "" {
			result[key] = value
		}
	}
	for key, value := range override {
		key = strings.TrimSpace(key)
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func trimStringValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func intParam(params map[string]any, key string) (int, bool) {
	value, ok := params[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	default:
		return 0, false
	}
}

func float32Param(params map[string]any, key string) (float32, bool) {
	value, ok := params[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float32:
		return typed, true
	case float64:
		return float32(typed), true
	case int:
		return float32(typed), true
	case int64:
		return float32(typed), true
	default:
		return 0, false
	}
}
