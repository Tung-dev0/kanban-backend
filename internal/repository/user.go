package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ivantung/todo-backend/internal/model"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrUsernameTaken = errors.New("username already taken")
	ErrEmailTaken    = errors.New("email already taken")
)

const userColumns = `id, username, email, display_name, avatar_url, google_id, password_hash, created_at`

type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

func scanUser(row interface{ Scan(...any) error }) (*model.User, error) {
	u := &model.User{}
	var email, displayName, avatarURL, googleID, passwordHash sql.NullString
	err := row.Scan(
		&u.ID,
		&u.Username,
		&email,
		&displayName,
		&avatarURL,
		&googleID,
		&passwordHash,
		&u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.Email = nullStr(email)
	u.DisplayName = nullStr(displayName)
	u.AvatarURL = nullStr(avatarURL)
	u.GoogleID = nullStr(googleID)
	u.PasswordHash = nullStr(passwordHash)
	return u, nil
}

func nullStr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

// Create inserts a username/password user (no OAuth fields yet).
func (r *UserRepo) Create(ctx context.Context, username, passwordHash string) (*model.User, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`,
		username, passwordHash,
	).Scan(&id)
	if err != nil {
		return nil, mapUniqueErr(err)
	}
	return r.GetByID(ctx, id)
}

// CreateOAuth inserts a Google-only user (no password_hash).
func (r *UserRepo) CreateOAuth(ctx context.Context, username, email, googleID, displayName, avatarURL string) (*model.User, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (username, email, google_id, display_name, avatar_url)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
		RETURNING id`,
		username, email, googleID, displayName, avatarURL,
	).Scan(&id)
	if err != nil {
		return nil, mapUniqueErr(err)
	}
	return r.GetByID(ctx, id)
}

// LinkGoogle attaches Google identity to an existing user.
func (r *UserRepo) LinkGoogle(ctx context.Context, userID int64, googleID, email, displayName, avatarURL string) (*model.User, error) {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET
			google_id    = COALESCE(google_id, $2),
			email        = COALESCE(email, $3),
			display_name = COALESCE(display_name, NULLIF($4, '')),
			avatar_url   = COALESCE(avatar_url,   NULLIF($5, ''))
		WHERE id = $1`,
		userID, googleID, email, displayName, avatarURL,
	)
	if err != nil {
		return nil, mapUniqueErr(err)
	}
	return r.GetByID(ctx, userID)
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = $1`, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, email)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetByGoogleID(ctx context.Context, googleID string) (*model.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE google_id = $1`, googleID)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

// UsernameExists is used by username-derivation to find a unique suffix.
func (r *UserRepo) UsernameExists(ctx context.Context, username string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM users WHERE username = $1 LIMIT 1`, username,
	).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func mapUniqueErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return ErrEmailTaken
		case "users_username_key":
			return ErrUsernameTaken
		default:
			// generic unique violation — treat as username taken for the password path,
			// callers needing more nuance should branch earlier.
			return ErrUsernameTaken
		}
	}
	return err
}
