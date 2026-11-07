package user

import (
	"context"
	"errors"
	"strings"
	"time"
)

// PasswordHasher и TokenIssuer — небольшие интерфейсы потребителя. UseCase
// зависит от нужного поведения, а не от bcrypt и конкретной JWT-библиотеки.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenIssuer interface {
	Issue(userID int64) (string, error)
}

type UseCase struct {
	repository    Repository
	hasher        PasswordHasher
	tokens        TokenIssuer
	sessions      RefreshSessionRepository
	refreshTokens RefreshTokenGenerator
	config        SessionConfig
	now           func() time.Time
}

func NewUseCase(repository Repository, hasher PasswordHasher, tokens TokenIssuer,
	sessions RefreshSessionRepository, refreshTokens RefreshTokenGenerator, config SessionConfig,
) *UseCase {
	return &UseCase{repository: repository, hasher: hasher, tokens: tokens,
		sessions: sessions, refreshTokens: refreshTokens, config: config, now: time.Now}
}

func (uc *UseCase) Register(ctx context.Context, input RegisterInput) (User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := input.Password
	if !strings.Contains(email, "@") || strings.HasPrefix(email, "@") || strings.HasSuffix(email, "@") {
		return User{}, ErrInvalidEmail
	}
	if len(password) < 8 || len(password) > 72 {
		return User{}, ErrWeakPassword
	}

	if _, err := uc.repository.GetByEmail(ctx, email); err == nil {
		return User{}, ErrAlreadyExists
	} else if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}

	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return User{}, err
	}

	return uc.repository.Create(ctx, User{Email: email, PasswordHash: hash})
}

func (uc *UseCase) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	if len(input.Password) < 8 || len(input.Password) > 72 {
		return LoginResult{}, ErrInvalidCredentials
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	value, err := uc.repository.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	if err := uc.hasher.Compare(value.PasswordHash, input.Password); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	result, session, err := uc.newSessionTokens(value.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if _, err := uc.sessions.Create(ctx, session); err != nil {
		return LoginResult{}, err
	}
	return result, nil
}

func (uc *UseCase) Refresh(ctx context.Context, raw string) (LoginResult, error) {
	if raw == "" || len(raw) > 512 {
		return LoginResult{}, ErrInvalidRefreshToken
	}
	hash := uc.refreshTokens.Hash(raw)
	current, err := uc.sessions.GetByTokenHash(ctx, hash)
	if err != nil {
		return LoginResult{}, err
	}
	if current.RevokedAt != nil {
		return LoginResult{}, ErrRefreshRevoked
	}
	if !current.ExpiresAt.After(uc.now()) {
		return LoginResult{}, ErrRefreshExpired
	}

	result, next, err := uc.newSessionTokens(current.UserID)
	if err != nil {
		return LoginResult{}, err
	}
	// Параллельный Refresh/Logout мог изменить сессию после чтения.
	// Rotate повторно проверяет её состояние под блокировкой.
	if err := uc.sessions.Rotate(ctx, hash, next); err != nil {
		return LoginResult{}, err
	}
	return result, nil
}

func (uc *UseCase) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > 512 {
		return nil
	}
	return uc.sessions.Revoke(ctx, uc.refreshTokens.Hash(raw))
}

func (uc *UseCase) newSessionTokens(userID int64) (LoginResult, RefreshSession, error) {
	if uc.config.AccessTTL < time.Second || uc.config.RefreshTTL < time.Second {
		return LoginResult{}, RefreshSession{}, errors.New("invalid session lifetime")
	}
	access, err := uc.tokens.Issue(userID)
	if err != nil {
		return LoginResult{}, RefreshSession{}, err
	}
	raw, err := uc.refreshTokens.Generate()
	if err != nil {
		return LoginResult{}, RefreshSession{}, err
	}
	now := uc.now().UTC()
	expires := now.Add(uc.config.RefreshTTL)
	return LoginResult{AccessToken: access, ExpiresIn: int64(uc.config.AccessTTL / time.Second),
			RefreshToken: raw, RefreshExpiresAt: expires},
		RefreshSession{UserID: userID, TokenHash: uc.refreshTokens.Hash(raw),
			ExpiresAt: expires, CreatedAt: now}, nil
}
