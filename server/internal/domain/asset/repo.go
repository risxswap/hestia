package asset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"hestia/server/internal/common/dbutil"

	"github.com/go-sql-driver/mysql"
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

func (r *MySQLRepository) Create(ctx context.Context, item Asset) (Asset, error) {
	items, err := r.CreateMany(ctx, []Asset{item})
	if err != nil {
		return Asset{}, err
	}
	if len(items) == 0 {
		return Asset{}, errors.New("asset create returned no rows")
	}
	return items[0], nil
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
INSERT INTO files
  (public_id, owner_user_id, bucket, object_key, mime_type, file_size, width, height, asset_type, source, status, review_status, metadata_json)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON))
`, items[i].PublicID, items[i].OwnerUserID, items[i].Bucket, items[i].ObjectKey, items[i].MimeType, items[i].FileSize, items[i].Width, items[i].Height, items[i].AssetType, items[i].Source, items[i].Status, items[i].ReviewStatus, metadata)
		if err != nil {
			if isDuplicateKeyError(err) {
				return nil, ErrAssetAlreadyExists
			}
			return nil, err
		}
		id, err := dbutil.RequireLastInsertID(result, "asset create")
		if err != nil {
			return nil, err
		}
		items[i].ID = id
	}
	return items, nil
}

func (r *MySQLRepository) FindByPublicID(ctx context.Context, publicID string) (Asset, error) {
	if r == nil || r.ext == nil {
		return Asset{}, errors.New("asset repository database is nil")
	}
	var row assetRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT id, public_id, owner_user_id, bucket, object_key, mime_type, file_size, width, height, asset_type, source, status, review_status, metadata_json
FROM files
WHERE public_id = ?
  AND deleted_at IS NULL
LIMIT 1
`, publicID)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrAssetNotFound
	}
	if err != nil {
		return Asset{}, err
	}
	return row.asset()
}

type assetRow struct {
	ID           int64          `db:"id"`
	PublicID     string         `db:"public_id"`
	OwnerUserID  sql.NullInt64  `db:"owner_user_id"`
	Bucket       string         `db:"bucket"`
	ObjectKey    string         `db:"object_key"`
	MimeType     string         `db:"mime_type"`
	FileSize     int64          `db:"file_size"`
	Width        sql.NullInt64  `db:"width"`
	Height       sql.NullInt64  `db:"height"`
	AssetType    string         `db:"asset_type"`
	Source       string         `db:"source"`
	Status       string         `db:"status"`
	ReviewStatus string         `db:"review_status"`
	MetadataJSON sql.NullString `db:"metadata_json"`
}

func (r assetRow) asset() (Asset, error) {
	item := Asset{
		ID:           r.ID,
		PublicID:     r.PublicID,
		Bucket:       r.Bucket,
		ObjectKey:    r.ObjectKey,
		MimeType:     r.MimeType,
		FileSize:     r.FileSize,
		AssetType:    r.AssetType,
		Source:       r.Source,
		Status:       r.Status,
		ReviewStatus: r.ReviewStatus,
		Metadata:     map[string]any{},
	}
	if r.OwnerUserID.Valid {
		item.OwnerUserID = r.OwnerUserID.Int64
	}
	if r.Width.Valid {
		width := int(r.Width.Int64)
		item.Width = &width
	}
	if r.Height.Valid {
		height := int(r.Height.Int64)
		item.Height = &height
	}
	if r.MetadataJSON.Valid && r.MetadataJSON.String != "" {
		if err := json.Unmarshal([]byte(r.MetadataJSON.String), &item.Metadata); err != nil {
			return Asset{}, err
		}
	}
	return item, nil
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
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
