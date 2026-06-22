package asset

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
)

type MySQLRepository struct {
	db *sqlx.DB
}

func NewMySQLRepository(db *sqlx.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) CreateMany(ctx context.Context, items []Asset) ([]Asset, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("asset repository database is nil")
	}
	for i := range items {
		result, err := r.db.ExecContext(ctx, `
INSERT INTO assets
  (public_id, owner_user_id, bucket, object_key, mime_type, file_size, width, height, asset_type, source, status, review_status)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, items[i].PublicID, items[i].OwnerUserID, items[i].Bucket, items[i].ObjectKey, items[i].MimeType, items[i].FileSize, items[i].Width, items[i].Height, items[i].AssetType, items[i].Source, items[i].Status, items[i].ReviewStatus)
		if err != nil {
			return nil, err
		}
		if id, err := result.LastInsertId(); err == nil {
			items[i].ID = id
		}
	}
	return items, nil
}
