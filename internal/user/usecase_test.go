package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	byEmail map[string]User
	created User
}

func (r *fakeRepository) Create(_ context.Context, value User) (User, error) {
	value.ID = 1
	r.created = value
	if r.byEmail == nil {
		r.byEmail = make(map[string]User)
	}
	r.byEmail[value.Email] = value
	return value, nil
}

func (r *fakeRepository) GetByEmail(_ context.Context, email string) (User, error) {
	value, ok := r.byEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return value, nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(password string) (string, error) { return "hash:" + password, nil }
func (fakeHasher) Compare(hash, password string) error {
	if hash != "hash:"+password {
		return errors.New("mismatch")
	}
	return nil
}

type fakeTokens struct{}

func (fakeTokens) Issue(userID int64) (string, error) { return "token-for-1", nil }

func TestRegisterHashesPassword(t *testing.T) {
	repository := &fakeRepository{}
	uc := newTestUseCase(repository, fakeHasher{}, fakeTokens{})

	created, err := uc.Register(context.Background(), RegisterInput{
		Email: "  ALICE@example.com ", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if created.Email != "alice@example.com" {
		t.Fatalf("Email = %q", created.Email)
	}
	if repository.created.PasswordHash != "hash:secret123" {
		t.Fatalf("PasswordHash = %q", repository.created.PasswordHash)
	}
	if repository.created.PasswordHash == "secret123" {
		t.Fatal("plaintext password was stored")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	repository := &fakeRepository{byEmail: map[string]User{
		"alice@example.com": {ID: 1, Email: "alice@example.com"},
	}}
	uc := newTestUseCase(repository, fakeHasher{}, fakeTokens{})

	_, err := uc.Register(context.Background(), RegisterInput{
		Email: "alice@example.com", Password: "secret123",
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "correct password", password: "secret123"},
		{name: "wrong password", password: "wrong-pass", wantErr: ErrInvalidCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &fakeRepository{byEmail: map[string]User{
				"alice@example.com": {ID: 1, Email: "alice@example.com", PasswordHash: "hash:secret123"},
			}}
			uc := newTestUseCase(repository, fakeHasher{}, fakeTokens{})
			result, err := uc.Login(context.Background(), LoginInput{
				Email: "alice@example.com", Password: tt.password,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && result.AccessToken == "" {
				t.Fatal("empty token")
			}
		})
	}
}

func TestLoginHidesMissingUser(t *testing.T) {
	uc := newTestUseCase(&fakeRepository{}, fakeHasher{}, fakeTokens{})
	_, err := uc.Login(context.Background(), LoginInput{
		Email: "missing@example.com", Password: "secret123",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}
}

func TestPasswordIsNotTrimmed(t *testing.T) {
	repo := &fakeRepository{}
	uc := newTestUseCase(repo, fakeHasher{}, fakeTokens{})
	_, err := uc.Register(context.Background(), RegisterInput{Email: "alice@example.com", Password: "  secret123  "})
	if err != nil {
		t.Fatal(err)
	}
	if repo.created.PasswordHash != "hash:  secret123  " {
		t.Fatalf("hash=%q", repo.created.PasswordHash)
	}
	if _, err := uc.Login(context.Background(), LoginInput{Email: "ALICE@example.com", Password: "  secret123  "}); err != nil {
		t.Fatalf("login=%v", err)
	}
	if _, err := uc.Login(context.Background(), LoginInput{Email: "alice@example.com", Password: "secret123"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("trimmed login=%v", err)
	}
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		email, password string
		want            error
	}{
		{"missing-at", "secret123", ErrInvalidEmail},
		{"alice@example.com", "short", ErrWeakPassword},
		{"alice@example.com", strings.Repeat("x", 73), ErrWeakPassword},
	} {
		_, err := newTestUseCase(&fakeRepository{}, fakeHasher{}, fakeTokens{}).Register(context.Background(), RegisterInput{Email: tt.email, Password: tt.password})
		if !errors.Is(err, tt.want) {
			t.Fatalf("register=%v want=%v", err, tt.want)
		}
	}
}

func newTestUseCase(repository Repository, hasher PasswordHasher, tokens TokenIssuer) *UseCase {
	return NewUseCase(repository, hasher, tokens, newFakeSessions(), &fakeRefreshTokens{},
		SessionConfig{AccessTTL: 15 * time.Minute, RefreshTTL: 7 * 24 * time.Hour})
}
