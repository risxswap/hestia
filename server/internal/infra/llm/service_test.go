package llm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	serverlogger "hestia/server/internal/infra/logger"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestServiceGenerateResolvesUsageAndCallsEinoModel(t *testing.T) {
	capturedModel := &captureEinoChatModel{}
	var capturedConfig *qwen.ChatModelConfig
	resolver := NewConfigResolver(memoryConfigRepo{
		usages: map[string]Usage{
			"agent_chat": {
				Key:           "agent_chat",
				ProviderCode:  "qwen",
				ModelCode:     "qwen-plus",
				PromptVersion: "v1",
				Params: map[string]any{
					"temperature": float64(0.7),
					"max_tokens":  float64(2000),
				},
			},
		},
		providers: map[string]Provider{
			"qwen": {ID: 7, Code: "qwen", APIBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Token: "token", Status: StatusActive},
		},
		models: map[string]Model{
			modelKey("qwen", "qwen-plus"): {ID: 9, ProviderCode: "qwen", ModelCode: "qwen-plus", Caps: []string{"text", "json"}, Status: StatusActive},
		},
	})
	service := NewService(resolver, EinoQwenChatModelFactoryFunc(func(_ context.Context, config *qwen.ChatModelConfig) (EinoChatModel, error) {
		capturedConfig = config
		return capturedModel, nil
	}))

	response, err := service.Generate(context.Background(), Request{
		UsageKey:     "agent_chat",
		RequiredCaps: []string{"text"},
		Messages: []Message{
			{Role: "system", Content: "你是形象顾问"},
			{Role: "user", Content: "今天怎么穿"},
		},
		Params: map[string]any{
			"temperature": float64(0.5),
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if response.Text != `{"name":"米白衬衫"}` {
		t.Fatalf("unexpected response text: %s", response.Text)
	}
	if response.Usage.Model.ModelCode != "qwen-plus" {
		t.Fatalf("expected resolved model in response, got %#v", response.Usage)
	}
	if capturedConfig == nil || capturedConfig.Model != "qwen-plus" || capturedConfig.Temperature == nil || *capturedConfig.Temperature != float32(0.5) {
		t.Fatalf("expected qwen config with override params, got %#v", capturedConfig)
	}
	if len(capturedModel.messages) != 2 ||
		capturedModel.messages[0].Role != schema.System ||
		capturedModel.messages[0].Content != "你是形象顾问" ||
		capturedModel.messages[1].Role != schema.User ||
		capturedModel.messages[1].Content != "今天怎么穿" {
		t.Fatalf("expected system and user messages, got %#v", capturedModel.messages)
	}
}

type captureEinoChatModel struct {
	messages []*schema.Message
	err      error
}

func (m *captureEinoChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.messages = input
	if m.err != nil {
		return nil, m.err
	}
	return schema.AssistantMessage(`{"name":"米白衬衫"}`, nil), nil
}

type captureToolCallingChatModel struct{}

func (m *captureToolCallingChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("ok", nil), nil
}

func (m *captureToolCallingChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *captureToolCallingChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type captureToolCallingClient struct {
	qwenModel      model.ToolCallingChatModel
	openAIModel    model.ToolCallingChatModel
	onQwenConfig   func(*qwen.ChatModelConfig)
	onOpenAIConfig func(*einoopenai.ChatModelConfig)
	qwenErr        error
	openAIErr      error
}

func (c *captureToolCallingClient) NewQwenChatModel(context.Context, *qwen.ChatModelConfig) (EinoChatModel, error) {
	return nil, ErrClientUnavailable
}

func (c *captureToolCallingClient) NewOpenAIChatModel(context.Context, *einoopenai.ChatModelConfig) (EinoChatModel, error) {
	return nil, ErrClientUnavailable
}

func (c *captureToolCallingClient) NewQwenToolCallingChatModel(_ context.Context, config *qwen.ChatModelConfig) (model.ToolCallingChatModel, error) {
	if c.onQwenConfig != nil {
		c.onQwenConfig(config)
	}
	if c.qwenErr != nil {
		return nil, c.qwenErr
	}
	return c.qwenModel, nil
}

func (c *captureToolCallingClient) NewOpenAIToolCallingChatModel(_ context.Context, config *einoopenai.ChatModelConfig) (model.ToolCallingChatModel, error) {
	if c.onOpenAIConfig != nil {
		c.onOpenAIConfig(config)
	}
	if c.openAIErr != nil {
		return nil, c.openAIErr
	}
	return c.openAIModel, nil
}

func TestServiceGenerateUsesEinoChatModelWithResolvedProviderAndImage(t *testing.T) {
	temperature := float64(0.2)
	maxTokens := float64(1200)
	capturedModel := &captureEinoChatModel{}
	var capturedConfig *qwen.ChatModelConfig
	service := NewService(NewConfigResolver(memoryConfigRepo{
		usages: map[string]Usage{
			"wardrobe_image_recognition": {
				Key:          "wardrobe_image_recognition",
				ProviderCode: "qwen",
				ModelCode:    "qwen-vl-plus",
				Params: map[string]any{
					"temperature":     temperature,
					"max_tokens":      maxTokens,
					"response_format": "json_object",
				},
			},
		},
		providers: map[string]Provider{
			"qwen": {ID: 7, Code: "qwen", APIBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Token: "token", Status: StatusActive},
		},
		models: map[string]Model{
			modelKey("qwen", "qwen-vl-plus"): {ID: 9, ProviderCode: "qwen", ModelCode: "qwen-vl-plus", Caps: []string{"vision", "json"}, Status: StatusActive},
		},
	}), EinoQwenChatModelFactoryFunc(func(_ context.Context, config *qwen.ChatModelConfig) (EinoChatModel, error) {
		capturedConfig = config
		return capturedModel, nil
	}))

	response, err := service.Generate(context.Background(), Request{
		UsageKey:     "wardrobe_image_recognition",
		RequiredCaps: []string{"vision", "json"},
		Messages: []Message{
			{Role: "user", Content: "识别这件衣服"},
		},
		ImageURLs: []string{"https://download.example.test/private.jpg"},
	})
	if err != nil {
		t.Fatalf("generate with eino: %v", err)
	}

	if response.Text != `{"name":"米白衬衫"}` {
		t.Fatalf("unexpected response text: %s", response.Text)
	}
	if capturedConfig == nil || capturedConfig.BaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" ||
		capturedConfig.APIKey != "token" || capturedConfig.Model != "qwen-vl-plus" {
		t.Fatalf("expected qwen config from resolved provider/model, got %#v", capturedConfig)
	}
	if capturedConfig.Temperature == nil || *capturedConfig.Temperature != float32(0.2) {
		t.Fatalf("expected temperature 0.2, got %#v", capturedConfig.Temperature)
	}
	if capturedConfig.MaxTokens == nil || *capturedConfig.MaxTokens != 1200 {
		t.Fatalf("expected max tokens 1200, got %#v", capturedConfig.MaxTokens)
	}
	if capturedConfig.ResponseFormat == nil || capturedConfig.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONObject {
		t.Fatalf("expected json_object response format, got %#v", capturedConfig.ResponseFormat)
	}
	if len(capturedModel.messages) != 1 || capturedModel.messages[0].Role != schema.User {
		t.Fatalf("expected one user message, got %#v", capturedModel.messages)
	}
	parts := capturedModel.messages[0].UserInputMultiContent
	if len(parts) != 2 || parts[0].Text != "识别这件衣服" || parts[1].Image == nil ||
		parts[1].Image.URL == nil || *parts[1].Image.URL != "https://download.example.test/private.jpg" {
		t.Fatalf("expected text and image multi content, got %#v", parts)
	}
}

func TestServiceNewToolCallingChatModelWithUsageReturnsResolvedMetadata(t *testing.T) {
	toolModel := &captureToolCallingChatModel{}
	var capturedConfig *qwen.ChatModelConfig
	service := NewService(NewConfigResolver(memoryConfigRepo{
		usages: map[string]Usage{
			"agent_chat": {
				Key:           "agent_chat",
				ProviderCode:  "qwen",
				ModelCode:     "qwen-plus",
				PromptVersion: "v2",
			},
		},
		providers: map[string]Provider{
			"qwen": {ID: 7, Code: "qwen", APIBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Token: "token", Status: StatusActive},
		},
		models: map[string]Model{
			modelKey("qwen", "qwen-plus"): {ID: 9, ProviderCode: "qwen", ModelCode: "qwen-plus", Caps: []string{"text"}, Status: StatusActive},
		},
	}), &captureToolCallingClient{
		qwenModel: toolModel,
		onQwenConfig: func(config *qwen.ChatModelConfig) {
			capturedConfig = config
		},
	})

	chatModel, resolved, err := service.NewToolCallingChatModelWithUsage(context.Background(), Request{
		UsageKey:     "agent_chat",
		RequiredCaps: []string{"text"},
	})
	if err != nil {
		t.Fatalf("new tool calling chat model: %v", err)
	}
	if chatModel != toolModel {
		t.Fatalf("expected returned tool model")
	}
	if resolved.Usage.Key != "agent_chat" || resolved.Provider.Code != "qwen" || resolved.Model.ModelCode != "qwen-plus" || resolved.Usage.PromptVersion != "v2" {
		t.Fatalf("unexpected resolved usage: %#v", resolved)
	}
	if capturedConfig == nil || capturedConfig.Model != "qwen-plus" {
		t.Fatalf("expected qwen model config, got %#v", capturedConfig)
	}
}

func TestServiceGenerateUsesSiliconFlowProviderFromDatabase(t *testing.T) {
	capturedModel := &captureEinoChatModel{}
	var capturedConfig *einoopenai.ChatModelConfig
	service := NewService(NewConfigResolver(memoryConfigRepo{
		usages: map[string]Usage{
			"wardrobe_image_recognition": {
				Key:          "wardrobe_image_recognition",
				ProviderCode: "siliconflow",
				ModelCode:    "Qwen/Qwen2.5-VL-72B-Instruct",
				Params: map[string]any{
					"temperature":     float64(0.2),
					"max_tokens":      float64(1200),
					"response_format": "json_object",
				},
			},
		},
		providers: map[string]Provider{
			"siliconflow": {ID: 17, Code: "siliconflow", APIBaseURL: "https://api.siliconflow.cn/v1", Token: "sf-token", Status: StatusActive},
		},
		models: map[string]Model{
			modelKey("siliconflow", "Qwen/Qwen2.5-VL-72B-Instruct"): {
				ID:           19,
				ProviderCode: "siliconflow",
				ModelCode:    "Qwen/Qwen2.5-VL-72B-Instruct",
				Caps:         []string{"vision", "json"},
				Status:       StatusActive,
			},
		},
	}), EinoOpenAIChatModelFactoryFunc(func(_ context.Context, config *einoopenai.ChatModelConfig) (EinoChatModel, error) {
		capturedConfig = config
		return capturedModel, nil
	}))

	_, err := service.Generate(context.Background(), Request{
		UsageKey:     "wardrobe_image_recognition",
		RequiredCaps: []string{"vision", "json"},
		Messages: []Message{
			{Role: "user", Content: "识别这件衣服"},
		},
		ImageURLs: []string{"https://download.example.test/private.jpg"},
	})
	if err != nil {
		t.Fatalf("generate with siliconflow: %v", err)
	}

	if capturedConfig == nil ||
		capturedConfig.BaseURL != "https://api.siliconflow.cn/v1" ||
		capturedConfig.APIKey != "sf-token" ||
		capturedConfig.Model != "Qwen/Qwen2.5-VL-72B-Instruct" {
		t.Fatalf("expected siliconflow config from database, got %#v", capturedConfig)
	}
	if capturedConfig.ResponseFormat == nil || capturedConfig.ResponseFormat.Type != einoopenai.ChatCompletionResponseFormatTypeJSONObject {
		t.Fatalf("expected json_object response format, got %#v", capturedConfig.ResponseFormat)
	}
	if len(capturedModel.messages) != 1 || len(capturedModel.messages[0].UserInputMultiContent) != 2 {
		t.Fatalf("expected image request sent through openai-compatible model, got %#v", capturedModel.messages)
	}
}

func TestServiceGenerateLogsLLMCallWithoutSensitiveValues(t *testing.T) {
	capturedModel := &captureEinoChatModel{}
	var logs bytes.Buffer
	service := NewService(NewConfigResolver(memoryConfigRepo{
		usages: map[string]Usage{
			"wardrobe_image_recognition": {
				Key:           "wardrobe_image_recognition",
				ProviderCode:  "siliconflow",
				ModelCode:     "Qwen/Qwen2.5-VL-72B-Instruct",
				PromptVersion: "v3",
			},
		},
		providers: map[string]Provider{
			"siliconflow": {ID: 17, Code: "siliconflow", APIBaseURL: "https://api.siliconflow.cn/v1", Token: "sf-token", Status: StatusActive},
		},
		models: map[string]Model{
			modelKey("siliconflow", "Qwen/Qwen2.5-VL-72B-Instruct"): {
				ID:           19,
				ProviderCode: "siliconflow",
				ModelCode:    "Qwen/Qwen2.5-VL-72B-Instruct",
				Caps:         []string{"vision", "json"},
				Status:       StatusActive,
			},
		},
	}), EinoOpenAIChatModelFactoryFunc(func(_ context.Context, _ *einoopenai.ChatModelConfig) (EinoChatModel, error) {
		return capturedModel, nil
	}))
	service.SetLogger(slog.New(slog.NewTextHandler(&logs, nil)))

	ctx := serverlogger.WithRequestID(context.Background(), "req_llm_test")
	_, err := service.Generate(ctx, Request{
		UsageKey:     "wardrobe_image_recognition",
		RequiredCaps: []string{"vision", "json"},
		Messages: []Message{
			{Role: "system", Content: "分析服装"},
			{Role: "user", Content: "联系 test@example.com 后识别这件衣服"},
		},
		ImageURLs: []string{"https://download.example.test/private.jpg?token=secret"},
		Params:    map[string]any{"temperature": 0.2, "max_tokens": 800},
	})
	if err != nil {
		t.Fatalf("generate with logging: %v", err)
	}

	output := logs.String()
	for _, expected := range []string{
		"llm call started",
		"llm call completed",
		"request_id=req_llm_test",
		"llm_call_id=llm_",
		"usage_key=wardrobe_image_recognition",
		"provider_code=siliconflow",
		"model_code=Qwen/Qwen2.5-VL-72B-Instruct",
		"prompt_version=v3",
		"message_count=2",
		"system_message_count=1",
		"user_message_count=1",
		"input_chars=",
		"param_keys=max_tokens,temperature",
		"image_count=1",
		"duration_ms=",
		"output_chars=",
		"input_summary=",
		"output_summary=",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log output to contain %q, got %s", expected, output)
		}
	}
	for _, leaked := range []string{"sf-token", "download.example.test", "token=secret", "test@example.com"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("expected log output not to contain sensitive value %q, got %s", leaked, output)
		}
	}
}

func TestServiceGenerateLogsEveryFailureStage(t *testing.T) {
	tests := []struct {
		name       string
		stage      string
		newService func(*bytes.Buffer) *Service
		request    Request
	}{
		{
			name: "client check", stage: "client_check",
			newService: func(logs *bytes.Buffer) *Service {
				service := NewService(NewConfigResolver(memoryConfigRepo{}), nil)
				service.SetLogger(slog.New(slog.NewTextHandler(logs, nil)))
				return service
			},
			request: Request{UsageKey: "agent_chat"},
		},
		{
			name: "config resolve", stage: "config_resolve",
			newService: func(logs *bytes.Buffer) *Service {
				service := NewService(NewConfigResolver(memoryConfigRepo{}), EinoQwenChatModelFactoryFunc(func(context.Context, *qwen.ChatModelConfig) (EinoChatModel, error) { return nil, nil }))
				service.SetLogger(slog.New(slog.NewTextHandler(logs, nil)))
				return service
			},
			request: Request{UsageKey: "missing"},
		},
		{
			name: "model init", stage: "model_init",
			newService: func(logs *bytes.Buffer) *Service {
				service := NewService(testResolver(), EinoQwenChatModelFactoryFunc(func(context.Context, *qwen.ChatModelConfig) (EinoChatModel, error) {
					return nil, errors.New("init failed token=secret")
				}))
				service.SetLogger(slog.New(slog.NewTextHandler(logs, nil)))
				return service
			},
			request: Request{UsageKey: "agent_chat"},
		},
		{
			name: "model generate", stage: "model_generate",
			newService: func(logs *bytes.Buffer) *Service {
				service := NewService(testResolver(), EinoQwenChatModelFactoryFunc(func(context.Context, *qwen.ChatModelConfig) (EinoChatModel, error) {
					return &captureEinoChatModel{err: errors.New("generate failed Bearer secret")}, nil
				}))
				service.SetLogger(slog.New(slog.NewTextHandler(logs, nil)))
				return service
			},
			request: Request{UsageKey: "agent_chat"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			_, err := test.newService(&logs).Generate(serverlogger.WithRequestID(context.Background(), "req_failure"), test.request)
			if err == nil {
				t.Fatal("expected generate error")
			}
			output := logs.String()
			for _, expected := range []string{"llm call failed", "level=ERROR", "request_id=req_failure", "llm_call_id=llm_", "error_stage=" + test.stage, "error="} {
				if !strings.Contains(output, expected) {
					t.Fatalf("expected %q in %s", expected, output)
				}
			}
			for _, leaked := range []string{"token=secret", "Bearer secret"} {
				if strings.Contains(output, leaked) {
					t.Fatalf("leaked %q in %s", leaked, output)
				}
			}
		})
	}
}

func TestServiceToolCallingLogsInitializationFailure(t *testing.T) {
	var logs bytes.Buffer
	service := NewService(testResolver(), &captureToolCallingClient{qwenErr: errors.New("tool init failed api_key=secret")})
	service.SetLogger(slog.New(slog.NewTextHandler(&logs, nil)))
	_, _, err := service.NewToolCallingChatModelWithUsage(serverlogger.WithRequestID(context.Background(), "req_tool"), Request{UsageKey: "agent_chat"})
	if err == nil {
		t.Fatal("expected tool model init error")
	}
	output := logs.String()
	for _, expected := range []string{"llm call failed", "request_id=req_tool", "error_stage=model_init"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in %s", expected, output)
		}
	}
	if strings.Contains(output, "api_key=secret") {
		t.Fatalf("expected secret redacted in %s", output)
	}
}

func testResolver() *ConfigResolver {
	return NewConfigResolver(memoryConfigRepo{
		usages:    map[string]Usage{"agent_chat": {Key: "agent_chat", ProviderCode: "qwen", ModelCode: "qwen-plus", PromptVersion: "v2"}},
		providers: map[string]Provider{"qwen": {Code: "qwen", Token: "provider-secret", Status: StatusActive}},
		models:    map[string]Model{modelKey("qwen", "qwen-plus"): {ProviderCode: "qwen", ModelCode: "qwen-plus", Status: StatusActive}},
	})
}

func TestServiceGenerateReturnsResolverAndClientErrors(t *testing.T) {
	t.Run("resolver error", func(t *testing.T) {
		service := NewService(NewConfigResolver(memoryConfigRepo{}), EinoQwenChatModelFactoryFunc(func(context.Context, *qwen.ChatModelConfig) (EinoChatModel, error) {
			t.Fatal("client should not be called")
			return nil, nil
		}))

		_, err := service.Generate(context.Background(), Request{UsageKey: "missing"})
		if !errors.Is(err, ErrUsageNotFound) {
			t.Fatalf("expected ErrUsageNotFound, got %v", err)
		}
	})

	t.Run("client unavailable", func(t *testing.T) {
		service := NewService(NewConfigResolver(memoryConfigRepo{}), nil)

		_, err := service.Generate(context.Background(), Request{UsageKey: "agent_chat"})
		if !errors.Is(err, ErrClientUnavailable) {
			t.Fatalf("expected ErrClientUnavailable, got %v", err)
		}
	})
}
