package asset

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jmoiron/sqlx"
)

type MySQLRepository struct {
	ext sqlx.ExtContext
}

func NewMySQLRepository(db *sqlx.DB) *MySQLRepository {
	return NewMySQLRepositoryWithExt(db)
}

func NewMySQLRepositoryWithExt(ext sqlx.ExtContext) *MySQLRepository {
	return &MySQLRepository{ext: ext}
}

func (r *MySQLRepository) CreateMany(ctx context.Context, items []Asset) ([]Asset, error) {
	if r == nil || r.ext == nil {
		return nil, errors.New("asset repository database is nil")
	}
	for i := range items {
		metadata, err := jsonText(items[i].Metadata)
		if err != nil {
			return nil, err
		}
		result, err := r.ext.ExecContext(ctx, `
INSERT INTO assets
  (public_id, owner_user_id, bucket, object_key, mime_type, file_size, width, height, asset_type, source, status, review_status, metadata_json)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON))
`, items[i].PublicID, items[i].OwnerUserID, items[i].Bucket, items[i].ObjectKey, items[i].MimeType, items[i].FileSize, items[i].Width, items[i].Height, items[i].AssetType, items[i].Source, items[i].Status, items[i].ReviewStatus, metadata)
		if err != nil {
			return nil, err
		}
		if id, err := result.LastInsertId(); err == nil {
			items[i].ID = id
		}
	}
	return items, nil
}

func jsonText(value any) (string, error) {
	if value == nil {
		return "null", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
