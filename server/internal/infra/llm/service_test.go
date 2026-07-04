package llm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type completeFunc func(ctx context.Context, input string) (string, error)

func (f completeFunc) Complete(ctx context.Context, input string) (string, error) {
	return f(ctx, input)
}

func TestServiceGenerateResolvesUsageAndCallsClient(t *testing.T) {
	var capturedInput string
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
			modelKey(7, "qwen-plus"): {ID: 9, ProviderID: 7, ModelCode: "qwen-plus", Caps: []string{"text", "json"}, Status: StatusActive},
		},
	})
	service := NewService(resolver, completeFunc(func(_ context.Context, input string) (string, error) {
		capturedInput = input
		return `{"answer":"ok"}`, nil
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

	if response.Text != `{"answer":"ok"}` {
		t.Fatalf("unexpected response text: %s", response.Text)
	}
	if response.Usage.Model.ModelCode != "qwen-plus" {
		t.Fatalf("expected resolved model in response, got %#v", response.Usage)
	}
	for _, want := range []string{"usage=agent_chat", "model=qwen-plus", "temperature=0.5", "max_tokens=2000", "你是形象顾问", "今天怎么穿"} {
		if !strings.Contains(capturedInput, want) {
			t.Fatalf("expected input to contain %q, got:\n%s", want, capturedInput)
		}
	}
}

func TestServiceGenerateReturnsResolverAndClientErrors(t *testing.T) {
	t.Run("resolver error", func(t *testing.T) {
		service := NewService(NewConfigResolver(memoryConfigRepo{}), completeFunc(func(context.Context, string) (string, error) {
			t.Fatal("client should not be called")
			return "", nil
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
