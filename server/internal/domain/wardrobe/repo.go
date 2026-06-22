package wardrobe

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

func (r *MySQLRepository) CreateCoreItems(ctx context.Context, items []Item) ([]Item, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("wardrobe repository database is nil")
	}
	for i := range items {
		result, err := r.db.ExecContext(ctx, `
INSERT INTO wardrobe_items
  (public_id, user_id, name, category, color, silhouette, material, season, user_notes, is_core, status)
VALUES
  (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)
`, items[i].PublicID, items[i].UserID, items[i].Name, items[i].Category, items[i].Color, items[i].Silhouette, items[i].Material, items[i].Season, items[i].UserNotes, items[i].IsCore, items[i].Status)
		if err != nil {
			return nil, err
		}
		if id, err := result.LastInsertId(); err == nil {
			items[i].ID = id
		}
	}
	return items, nil
}
