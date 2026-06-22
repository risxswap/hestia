package account

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/id"
)

const defaultSessionTTL = 30 * 24 * time.Hour
const maxDevKeyLength = 120
const maxNicknameLength = 128

var ErrValidation = errors.New("account validation failed")

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func (e ValidationError) Is(target error) bool {
	return target == ErrValidation
}

type Service struct {
	repo       UserRepository
	sessions   auth.SessionWriter
	sessionTTL time.Duration
}

func NewService(repo UserRepository, sessions auth.SessionWriter, sessionTTL time.Duration) *Service {
	if sessionTTL == 0 {
		sessionTTL = defaultSessionTTL
	}
	return &Service{repo: repo, sessions: sessions, sessionTTL: sessionTTL}
}

func (s *Service) DevLogin(ctx context.Context, input DevLoginInput) (DevLoginResult, error) {
	devKey := strings.TrimSpace(input.DevKey)
	nickname := strings.TrimSpace(input.Nickname)
	if devKey == "" {
		return DevLoginResult{}, ValidationError{Field: "dev_key", Message: "required"}
	}
	if utf8.RuneCountInString(devKey) > maxDevKeyLength {
		return DevLoginResult{}, ValidationError{Field: "dev_key", Message: "too long"}
	}
	if utf8.RuneCountInString(nickname) > maxNicknameLength {
		return DevLoginResult{}, ValidationError{Field: "nickname", Message: "too long"}
	}
	if s == nil || s.repo == nil || s.sessions == nil {
		return DevLoginResult{}, errors.New("account service dependencies are nil")
	}

	user, err := s.repo.UpsertDevUser(ctx, DevUserInput{
		PublicID:     id.NewPublicID("usr"),
		WechatOpenID: "dev:" + devKey,
		Nickname:     nickname,
	})
	if err != nil {
		return DevLoginResult{}, err
	}

	token := id.NewToken("dev")
	err = s.sessions.Set(ctx, token, auth.Session{
		UserID:       user.ID,
		UserPublicID: user.PublicID,
		Surface:      "user",
		CreatedAt:    time.Now().UTC(),
	}, s.sessionTTL)
	if err != nil {
		return DevLoginResult{}, err
	}

	return DevLoginResult{
		UserPublicID:     user.PublicID,
		Token:            token,
		OnboardingStatus: user.OnboardingStatus,
	}, nil
}
