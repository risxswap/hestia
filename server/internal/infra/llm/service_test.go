package llm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

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
}

func (m *captureEinoChatModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.messages = input
	return schema.AssistantMessage(`{"name":"米白衬衫"}`, nil), nil
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
				Key:          "wardrobe_image_recognition",
				ProviderCode: "siliconflow",
				ModelCode:    "Qwen/Qwen2.5-VL-72B-Instruct",
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

	_, err := service.Generate(context.Background(), Request{
		UsageKey:     "wardrobe_image_recognition",
		RequiredCaps: []string{"vision", "json"},
		Messages: []Message{
			{Role: "user", Content: "识别这件衣服"},
		},
		ImageURLs: []string{"https://download.example.test/private.jpg?token=secret"},
	})
	if err != nil {
		t.Fatalf("generate with logging: %v", err)
	}

	output := logs.String()
	for _, expected := range []string{
		"llm generate started",
		"llm generate completed",
		"usage_key=wardrobe_image_recognition",
		"provider_code=siliconflow",
		"model_code=Qwen/Qwen2.5-VL-72B-Instruct",
		"image_count=1",
		"duration_ms=",
		"response_chars=",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log output to contain %q, got %s", expected, output)
		}
	}
	for _, leaked := range []string{"sf-token", "download.example.test", "token=secret", "识别这件衣服"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("expected log output not to contain sensitive value %q, got %s", leaked, output)
		}
	}
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
