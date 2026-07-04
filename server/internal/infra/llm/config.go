package llm

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

const StatusActive = "active"

var (
	ErrUsageNotFound          = errors.New("llm usage not found")
	ErrProviderNotFound       = errors.New("llm provider not found")
	ErrModelNotFound          = errors.New("llm model not found")
	ErrCapabilityNotSupported = errors.New("llm model capability not supported")
	ErrInvalidUsageKey        = errors.New("invalid llm usage key")
	ErrClientUnavailable      = errors.New("llm client unavailable")
)

type Provider struct {
	ID         int64  `db:"id"`
	Code       string `db:"code"`
	Name       string `db:"name"`
	APIBaseURL string `db:"api_base_url"`
	Token      string `db:"token"`
	AuthType   string `db:"auth_type"`
	Status     string `db:"status"`
}

type Model struct {
	ID              int64    `db:"id"`
	ProviderID      int64    `db:"provider_id"`
	ModelCode       string   `db:"model_code"`
	Name            string   `db:"name"`
	Caps            []string `db:"-"`
	MaxInputTokens  *int     `db:"-"`
	MaxOutputTokens *int     `db:"-"`
	Status          string   `db:"status"`
}

type Usage struct {
	Key           string
	ProviderCode  string
	ModelCode     string
	PromptVersion string
	Params        map[string]any
}

type ResolvedUsage struct {
	Usage    Usage
	Provider Provider
	Model    Model
}

type ConfigRepository interface {
	FindUsage(ctx context.Context, key string) (Usage, error)
	FindProviderByCode(ctx context.Context, code string) (Provider, error)
	FindModel(ctx context.Context, providerID int64, modelCode string) (Model, error)
}

type ConfigResolver struct {
	repo ConfigRepository
}

func NewConfigResolver(repo ConfigRepository) *ConfigResolver {
	return &ConfigResolver{repo: repo}
}

func (r *ConfigResolver) ResolveUsage(ctx context.Context, key string, requiredCaps []string) (ResolvedUsage, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return ResolvedUsage{}, ErrInvalidUsageKey
	}
	if r == nil || r.repo == nil {
		return ResolvedUsage{}, ErrUsageNotFound
	}
	usage, err := r.repo.FindUsage(ctx, key)
	if err != nil {
		return ResolvedUsage{}, err
	}
	provider, err := r.repo.FindProviderByCode(ctx, usage.ProviderCode)
	if err != nil {
		return ResolvedUsage{}, err
	}
	model, err := r.repo.FindModel(ctx, provider.ID, usage.ModelCode)
	if err != nil {
		return ResolvedUsage{}, err
	}
	if !hasRequiredCaps(model.Caps, requiredCaps) {
		return ResolvedUsage{}, ErrCapabilityNotSupported
	}
	return ResolvedUsage{
		Usage:    usage,
		Provider: provider,
		Model:    model,
	}, nil
}

func hasRequiredCaps(caps []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	seen := map[string]bool{}
	for _, cap := range caps {
		cap = strings.ToLower(strings.TrimSpace(cap))
		if cap != "" {
			seen[cap] = true
		}
	}
	for _, cap := range required {
		cap = strings.ToLower(strings.TrimSpace(cap))
		if cap != "" && !seen[cap] {
			return false
		}
	}
	return true
}

func modelKey(providerID int64, modelCode string) string {
	return strings.TrimSpace(modelCode) + "#" + strconvFormatInt(providerID)
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
