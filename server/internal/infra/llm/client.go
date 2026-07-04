package llm

import (
	"context"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type EinoChatModel interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

type Client interface {
	NewQwenChatModel(ctx context.Context, config *qwen.ChatModelConfig) (EinoChatModel, error)
	NewOpenAIChatModel(ctx context.Context, config *einoopenai.ChatModelConfig) (EinoChatModel, error)
}

type EinoQwenChatModelFactoryFunc func(ctx context.Context, config *qwen.ChatModelConfig) (EinoChatModel, error)

func (f EinoQwenChatModelFactoryFunc) NewQwenChatModel(ctx context.Context, config *qwen.ChatModelConfig) (EinoChatModel, error) {
	return f(ctx, config)
}

func (f EinoQwenChatModelFactoryFunc) NewOpenAIChatModel(ctx context.Context, config *einoopenai.ChatModelConfig) (EinoChatModel, error) {
	return nil, ErrClientUnavailable
}

type EinoOpenAIChatModelFactoryFunc func(ctx context.Context, config *einoopenai.ChatModelConfig) (EinoChatModel, error)

func (f EinoOpenAIChatModelFactoryFunc) NewQwenChatModel(ctx context.Context, config *qwen.ChatModelConfig) (EinoChatModel, error) {
	return nil, ErrClientUnavailable
}

func (f EinoOpenAIChatModelFactoryFunc) NewOpenAIChatModel(ctx context.Context, config *einoopenai.ChatModelConfig) (EinoChatModel, error) {
	return f(ctx, config)
}

type EinoClient struct{}

func NewEinoClient() *EinoClient {
	return &EinoClient{}
}

func (c *EinoClient) NewQwenChatModel(ctx context.Context, config *qwen.ChatModelConfig) (EinoChatModel, error) {
	return qwen.NewChatModel(ctx, config)
}

func (c *EinoClient) NewOpenAIChatModel(ctx context.Context, config *einoopenai.ChatModelConfig) (EinoChatModel, error) {
	return einoopenai.NewChatModel(ctx, config)
}
