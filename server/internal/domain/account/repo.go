package account

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
)

type UserRepository interface {
	UpsertDevUser(ctx context.Context, input DevUserInput) (User, error)
}

type MySQLUserRepository struct {
	db *sqlx.DB
}

func NewMySQLUserRepository(db *sqlx.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

func (r *MySQLUserRepository) UpsertDevUser(ctx context.Context, input DevUserInput) (User, error) {
	if r == nil || r.db == nil {
		return User{}, errors.New("account repository database is nil")
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO users (public_id, wechat_openid, nickname, onboarding_status, status, last_active_at)
VALUES (?, ?, ?, 'not_started', 'active', NOW(3))
ON DUPLICATE KEY UPDATE
  nickname = VALUES(nickname),
  last_active_at = NOW(3),
  deleted_at = NULL
`, input.PublicID, input.WechatOpenID, input.Nickname)
	if err != nil {
		return User{}, err
	}
	var user User
	err = r.db.GetContext(ctx, &user, `
SELECT
  id,
  public_id,
  COALESCE(wechat_openid, '') AS wechat_openid,
  COALESCE(nickname, '') AS nickname,
  onboarding_status,
  status
FROM users
WHERE wechat_openid = ?
LIMIT 1
`, input.WechatOpenID)
	return user, err
}
