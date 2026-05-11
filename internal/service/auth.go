package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ivantung/todo-backend/internal/auth"
	"github.com/ivantung/todo-backend/internal/model"
	"github.com/ivantung/todo-backend/internal/oauth"
	"github.com/ivantung/todo-backend/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUseGoogle          = errors.New("this account uses Google sign-in")
	ErrEmailNotVerified   = errors.New("google email is not verified")
)

type AuthService struct {
	users  *repository.UserRepo
	signer *auth.Signer
}

func NewAuthService(users *repository.UserRepo, signer *auth.Signer) *AuthService {
	return &AuthService{users: users, signer: signer}
}

type TokenResult struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      *model.User `json:"user"`
}

func (s *AuthService) Register(ctx context.Context, username, password string) (*TokenResult, error) {
	username = strings.TrimSpace(username)
	if !validUsername(username) || !validPassword(password) {
		return nil, ErrInvalidInput
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	u, err := s.users.Create(ctx, username, hash)
	if err != nil {
		return nil, err
	}
	return s.issue(u)
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*TokenResult, error) {
	username = strings.TrimSpace(username)
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if u.PasswordHash == nil {
		return nil, ErrUseGoogle
	}
	if !auth.CheckPassword(*u.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return s.issue(u)
}

// LoginOrRegisterFromGoogle resolves a Google profile into a user (creating
// or linking as needed) and returns a fresh JWT.
func (s *AuthService) LoginOrRegisterFromGoogle(ctx context.Context, p oauth.Profile) (*TokenResult, error) {
	if !p.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	// 1) existing google_id → straight login
	if u, err := s.users.GetByGoogleID(ctx, p.Sub); err == nil {
		return s.issue(u)
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}

	// 2) existing email (e.g. created by password earlier with email backfill, or by Google previously) → link
	if u, err := s.users.GetByEmail(ctx, p.Email); err == nil {
		linked, err := s.users.LinkGoogle(ctx, u.ID, p.Sub, p.Email, p.Name, p.Picture)
		if err != nil {
			return nil, err
		}
		return s.issue(linked)
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}

	// 3) brand new user
	username, err := s.deriveUniqueUsername(ctx, p.Email, p.Name)
	if err != nil {
		return nil, err
	}
	u, err := s.users.CreateOAuth(ctx, username, p.Email, p.Sub, p.Name, p.Picture)
	if err != nil {
		return nil, err
	}
	return s.issue(u)
}

func (s *AuthService) issue(u *model.User) (*TokenResult, error) {
	tok, exp, err := s.signer.Sign(u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	return &TokenResult{Token: tok, ExpiresAt: exp, User: u}, nil
}

// deriveUniqueUsername builds a base from the email local-part (or name),
// sanitises to the username charset, then appends an incrementing suffix
// until it doesn't collide.
func (s *AuthService) deriveUniqueUsername(ctx context.Context, email, name string) (string, error) {
	base := sanitizeUsername(usernameSeed(email, name))
	if base == "" {
		base = randomFallback()
	}
	if len(base) < 3 {
		base = base + "_user"
	}
	if len(base) > 28 { // leave space for a 4-digit suffix
		base = base[:28]
	}

	candidate := base
	for i := 2; i < 1000; i++ {
		taken, err := s.users.UsernameExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s%d", base, i)
	}
	return "", errors.New("could not derive unique username")
}

func usernameSeed(email, name string) string {
	if email != "" {
		local := strings.SplitN(email, "@", 2)[0]
		local = strings.SplitN(local, "+", 2)[0]
		return local
	}
	if name != "" {
		return strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	}
	return ""
}

func sanitizeUsername(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r == '_' || r == '-' || r == '.' ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func randomFallback() string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return "user_" + hex.EncodeToString(buf[:])
}

func validUsername(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 3 || n > 32 {
		return false
	}
	for _, r := range s {
		if !(r == '_' || r == '-' || r == '.' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func validPassword(s string) bool {
	n := len(s)
	return n >= 6 && n <= 128
}
