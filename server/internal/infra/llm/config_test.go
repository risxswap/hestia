package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestMySQLConfigRepositoryFindsUsageProviderAndModel(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewMySQLConfigRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))
	mock.ExpectQuery("SELECT `key`, `value`").
		WithArgs("wardrobe_image_recognition", StatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).AddRow(
			"wardrobe_image_recognition",
			[]byte(`{"provider_code":"qwen","model_code":"qwen-vl-plus","prompt_version":"v1","params":{"temperature":0.2,"max_tokens":1200}}`),
		))
	mock.ExpectQuery("SELECT id, code, name, api_base_url, token, auth_type, status").
		WithArgs("qwen", StatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "api_base_url", "token", "auth_type", "status"}).
			AddRow(7, "qwen", "通义千问", "https://dashscope.aliyuncs.com/compatible-mode/v1", "token", "bearer", StatusActive))
	mock.ExpectQuery("SELECT id, provider_id, model_code, name, caps_json, max_input_tokens, max_output_tokens, status").
		WithArgs(int64(7), "qwen-vl-plus", StatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"id", "provider_id", "model_code", "name", "caps_json", "max_input_tokens", "max_output_tokens", "status"}).
			AddRow(9, 7, "qwen-vl-plus", "Qwen VL Plus", []byte(`["text","vision","json"]`), 32000, 2000, StatusActive))

	usage, err := repo.FindUsage(context.Background(), " wardrobe_image_recognition ")
	if err != nil {
		t.Fatalf("find usage: %v", err)
	}
	provider, err := repo.FindProviderByCode(context.Background(), usage.ProviderCode)
	if err != nil {
		t.Fatalf("find provider: %v", err)
	}
	model, err := repo.FindModel(context.Background(), provider.ID, usage.ModelCode)
	if err != nil {
		t.Fatalf("find model: %v", err)
	}

	if provider.APIBaseURL == "" || model.ModelCode != "qwen-vl-plus" || len(model.Caps) != 3 {
		t.Fatalf("unexpected config: usage=%#v provider=%#v model=%#v", usage, provider, model)
	}
	if model.MaxInputTokens == nil || *model.MaxInputTokens != 32000 {
		t.Fatalf("expected max input tokens to be parsed, got %#v", model.MaxInputTokens)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

type memoryConfigRepo struct {
	usages    map[string]Usage
	providers map[string]Provider
	models    map[string]Model
}

func (r memoryConfigRepo) FindUsage(_ context.Context, key string) (Usage, error) {
	usage, ok := r.usages[key]
	if !ok {
		return Usage{}, ErrUsageNotFound
	}
	return usage, nil
}

func (r memoryConfigRepo) FindProviderByCode(_ context.Context, code string) (Provider, error) {
	provider, ok := r.providers[code]
	if !ok {
		return Provider{}, ErrProviderNotFound
	}
	return provider, nil
}

func (r memoryConfigRepo) FindModel(_ context.Context, providerID int64, modelCode string) (Model, error) {
	model, ok := r.models[modelKey(providerID, modelCode)]
	if !ok {
		return Model{}, ErrModelNotFound
	}
	return model, nil
}

func TestConfigResolverResolvesActiveUsageProviderAndModel(t *testing.T) {
	resolver := NewConfigResolver(memoryConfigRepo{
		usages: map[string]Usage{
			"wardrobe_image_recognition": {
				Key:           "wardrobe_image_recognition",
				ProviderCode:  "qwen",
				ModelCode:     "qwen-vl-plus",
				PromptVersion: "v1",
				Params: map[string]any{
					"temperature": float64(0.2),
					"max_tokens":  float64(1200),
				},
			},
		},
		providers: map[string]Provider{
			"qwen": {
				ID:         7,
				Code:       "qwen",
				Name:       "通义千问",
				APIBaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
				Token:      "token",
				AuthType:   "bearer",
				Status:     StatusActive,
			},
		},
		models: map[string]Model{
			modelKey(7, "qwen-vl-plus"): {
				ID:         9,
				ProviderID: 7,
				ModelCode:  "qwen-vl-plus",
				Name:       "Qwen VL Plus",
				Caps:       []string{"text", "vision", "json"},
				Status:     StatusActive,
			},
		},
	})

	resolved, err := resolver.ResolveUsage(context.Background(), " wardrobe_image_recognition ", []string{"vision", "json"})
	if err != nil {
		t.Fatalf("resolve usage: %v", err)
	}

	if resolved.Usage.ProviderCode != "qwen" || resolved.Provider.ID != 7 || resolved.Model.ModelCode != "qwen-vl-plus" {
		t.Fatalf("unexpected resolved config: %#v", resolved)
	}
	if resolved.Usage.Params["max_tokens"] != float64(1200) {
		t.Fatalf("expected usage params to be preserved, got %#v", resolved.Usage.Params)
	}
}

func TestConfigResolverRejectsMissingRequiredCapability(t *testing.T) {
	resolver := NewConfigResolver(memoryConfigRepo{
		usages: map[string]Usage{
			"agent_chat": {Key: "agent_chat", ProviderCode: "qwen", ModelCode: "qwen-plus"},
		},
		providers: map[string]Provider{
			"qwen": {ID: 7, Code: "qwen", Status: StatusActive},
		},
		models: map[string]Model{
			modelKey(7, "qwen-plus"): {ID: 9, ProviderID: 7, ModelCode: "qwen-plus", Caps: []string{"text"}, Status: StatusActive},
		},
	})

	_, err := resolver.ResolveUsage(context.Background(), "agent_chat", []string{"vision"})
	if !errors.Is(err, ErrCapabilityNotSupported) {
		t.Fatalf("expected ErrCapabilityNotSupported, got %v", err)
	}
}

func TestConfigResolverReturnsNotFoundErrors(t *testing.T) {
	tests := []struct {
		name string
		repo memoryConfigRepo
		want error
	}{
		{
			name: "missing usage",
			repo: memoryConfigRepo{},
			want: ErrUsageNotFound,
		},
		{
			name: "missing provider",
			repo: memoryConfigRepo{
				usages: map[string]Usage{"agent_chat": {Key: "agent_chat", ProviderCode: "missing", ModelCode: "model"}},
			},
			want: ErrProviderNotFound,
		},
		{
			name: "missing model",
			repo: memoryConfigRepo{
				usages:    map[string]Usage{"agent_chat": {Key: "agent_chat", ProviderCode: "qwen", ModelCode: "missing"}},
				providers: map[string]Provider{"qwen": {ID: 7, Code: "qwen", Status: StatusActive}},
			},
			want: ErrModelNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewConfigResolver(tt.repo)

			_, err := resolver.ResolveUsage(context.Background(), "agent_chat", []string{"text"})
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}
