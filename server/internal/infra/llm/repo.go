package llm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
)

type MySQLConfigRepository struct {
	ext sqlx.ExtContext
}

func NewMySQLConfigRepository(db *sqlx.DB) *MySQLConfigRepository {
	return NewMySQLConfigRepositoryWithExt(db)
}

func NewMySQLConfigRepositoryWithExt(ext sqlx.ExtContext) *MySQLConfigRepository {
	return &MySQLConfigRepository{ext: ext}
}

func (r *MySQLConfigRepository) FindUsage(ctx context.Context, key string) (Usage, error) {
	if r == nil || r.ext == nil {
		return Usage{}, ErrUsageNotFound
	}
	var row llmUsageRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT `+"`key`, `value`"+`
FROM system_configs
WHERE `+"`group`"+` = 'llm.usages'
  AND `+"`key`"+` = ?
  AND status = ?
LIMIT 1
`, strings.TrimSpace(key), StatusActive)
	if errors.Is(err, sql.ErrNoRows) {
		return Usage{}, ErrUsageNotFound
	}
	if err != nil {
		return Usage{}, err
	}
	return row.usage()
}

func (r *MySQLConfigRepository) FindProviderByCode(ctx context.Context, code string) (Provider, error) {
	if r == nil || r.ext == nil {
		return Provider{}, ErrProviderNotFound
	}
	var provider Provider
	err := sqlx.GetContext(ctx, r.ext, &provider, `
SELECT id, code, name, api_base_url, token, auth_type, status
FROM llm_providers
WHERE code = ?
  AND status = ?
  AND deleted_at IS NULL
LIMIT 1
`, strings.TrimSpace(code), StatusActive)
	if errors.Is(err, sql.ErrNoRows) {
		return Provider{}, ErrProviderNotFound
	}
	if err != nil {
		return Provider{}, err
	}
	return provider, nil
}

func (r *MySQLConfigRepository) FindModel(ctx context.Context, providerID int64, modelCode string) (Model, error) {
	if r == nil || r.ext == nil {
		return Model{}, ErrModelNotFound
	}
	var row llmModelRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT id, provider_id, model_code, name, caps_json, max_input_tokens, max_output_tokens, status
FROM llm_models
WHERE provider_id = ?
  AND model_code = ?
  AND status = ?
  AND deleted_at IS NULL
LIMIT 1
`, providerID, strings.TrimSpace(modelCode), StatusActive)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, ErrModelNotFound
	}
	if err != nil {
		return Model{}, err
	}
	return row.model()
}

type llmUsageRow struct {
	Key   string          `db:"key"`
	Value json.RawMessage `db:"value"`
}

func (r llmUsageRow) usage() (Usage, error) {
	var raw struct {
		ProviderCode  string         `json:"provider_code"`
		ModelCode     string         `json:"model_code"`
		PromptVersion string         `json:"prompt_version"`
		Params        map[string]any `json:"params"`
	}
	if err := json.Unmarshal(r.Value, &raw); err != nil {
		return Usage{}, err
	}
	if raw.Params == nil {
		raw.Params = map[string]any{}
	}
	return Usage{
		Key:           strings.TrimSpace(r.Key),
		ProviderCode:  strings.TrimSpace(raw.ProviderCode),
		ModelCode:     strings.TrimSpace(raw.ModelCode),
		PromptVersion: strings.TrimSpace(raw.PromptVersion),
		Params:        raw.Params,
	}, nil
}

type llmModelRow struct {
	ID              int64           `db:"id"`
	ProviderID      int64           `db:"provider_id"`
	ModelCode       string          `db:"model_code"`
	Name            string          `db:"name"`
	CapsJSON        json.RawMessage `db:"caps_json"`
	MaxInputTokens  sql.NullInt64   `db:"max_input_tokens"`
	MaxOutputTokens sql.NullInt64   `db:"max_output_tokens"`
	Status          string          `db:"status"`
}

func (r llmModelRow) model() (Model, error) {
	var caps []string
	if err := json.Unmarshal(r.CapsJSON, &caps); err != nil {
		return Model{}, err
	}
	model := Model{
		ID:         r.ID,
		ProviderID: r.ProviderID,
		ModelCode:  strings.TrimSpace(r.ModelCode),
		Name:       strings.TrimSpace(r.Name),
		Caps:       trimStrings(caps),
		Status:     strings.TrimSpace(r.Status),
	}
	if r.MaxInputTokens.Valid {
		value := int(r.MaxInputTokens.Int64)
		model.MaxInputTokens = &value
	}
	if r.MaxOutputTokens.Valid {
		value := int(r.MaxOutputTokens.Int64)
		model.MaxOutputTokens = &value
	}
	return model, nil
}

func trimStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
