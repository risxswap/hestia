package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

type Service struct {
	resolver *ConfigResolver
	client   Client
}

func NewService(resolver *ConfigResolver, client Client) *Service {
	return &Service{resolver: resolver, client: client}
}

func (s *Service) Generate(ctx context.Context, request Request) (Response, error) {
	if s == nil || s.client == nil {
		return Response{}, ErrClientUnavailable
	}
	resolved, err := s.resolver.ResolveUsage(ctx, request.UsageKey, request.RequiredCaps)
	if err != nil {
		return Response{}, err
	}
	input := renderCompletionInput(resolved, request)
	text, err := s.client.Complete(ctx, input)
	if err != nil {
		return Response{}, err
	}
	return Response{Text: text, Usage: resolved}, nil
}

func renderCompletionInput(resolved ResolvedUsage, request Request) string {
	var builder strings.Builder
	builder.WriteString("usage=")
	builder.WriteString(resolved.Usage.Key)
	builder.WriteByte('\n')
	builder.WriteString("provider=")
	builder.WriteString(resolved.Provider.Code)
	builder.WriteByte('\n')
	builder.WriteString("model=")
	builder.WriteString(resolved.Model.ModelCode)
	builder.WriteByte('\n')
	if resolved.Usage.PromptVersion != "" {
		builder.WriteString("prompt_version=")
		builder.WriteString(resolved.Usage.PromptVersion)
		builder.WriteByte('\n')
	}
	params := mergedParams(resolved.Usage.Params, request.Params)
	for _, key := range sortedParamKeys(params) {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(fmt.Sprint(params[key]))
		builder.WriteByte('\n')
	}
	for _, imageURL := range request.ImageURLs {
		imageURL = strings.TrimSpace(imageURL)
		if imageURL == "" {
			continue
		}
		builder.WriteString("image_url=")
		builder.WriteString(imageURL)
		builder.WriteByte('\n')
	}
	for _, message := range request.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		builder.WriteString("\n[")
		builder.WriteString(role)
		builder.WriteString("]\n")
		builder.WriteString(strings.TrimSpace(message.Content))
		builder.WriteByte('\n')
	}
	return strings.TrimSpace(builder.String())
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

func sortedParamKeys(params map[string]any) []string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
