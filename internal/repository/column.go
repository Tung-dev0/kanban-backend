package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ivantung/todo-backend/internal/model"
)

var ErrColumnNotFound = errors.New("column not found")

const columnColumns = `id, user_id, name, position, created_at, updated_at`

type ColumnRepo struct{ db *sql.DB }

func NewColumnRepo(db *sql.DB) *ColumnRepo { return &ColumnRepo{db: db} }

func scanColumn(row interface{ Scan(...any) error }) (*model.Column, error) {
	c := &model.Column{}
	err := row.Scan(&c.ID, &c.UserID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Create inserts a new column for the user at MAX(position)+1.
func (r *ColumnRepo) Create(ctx context.Context, userID int64, name string) (*model.Column, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO columns (user_id, name, position)
		VALUES ($1, $2, COALESCE((SELECT MAX(position) FROM columns WHERE user_id = $1), 0) + 1)
		RETURNING id`,
		userID, name,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, userID, id)
}

// GetByID fetches a single column owned by the user.
func (r *ColumnRepo) GetByID(ctx context.Context, userID, id int64) (*model.Column, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+columnColumns+` FROM columns WHERE id = $1 AND user_id = $2`, id, userID)
	c, err := scanColumn(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrColumnNotFound
	}
	return c, err
}

// ListByUser returns all columns for the user ordered by position.
func (r *ColumnRepo) ListByUser(ctx context.Context, userID int64) ([]model.Column, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+columnColumns+` FROM columns WHERE user_id = $1 ORDER BY position`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.Column, 0)
	for rows.Next() {
		c := model.Column{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Update renames a column.
func (r *ColumnRepo) Update(ctx context.Context, userID, id int64, name string) (*model.Column, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE columns SET name = $1, updated_at = now() WHERE id = $2 AND user_id = $3`,
		name, id, userID)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrColumnNotFound
	}
	return r.GetByID(ctx, userID, id)
}

// Delete removes a column. Caller must ensure it's empty first.
func (r *ColumnRepo) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM columns WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrColumnNotFound
	}
	return nil
}

// CountCards returns the number of cards in a column.
func (r *ColumnRepo) CountCards(ctx context.Context, columnID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cards WHERE column_id = $1`, columnID).Scan(&n)
	return n, err
}

// ReorderInTx reassigns positions 1..N to columns in the provided order.
// The caller is responsible for validating that ids matches the full user set.
// This runs inside the provided transaction (tx must not be nil).
func (r *ColumnRepo) ReorderInTx(ctx context.Context, tx *sql.Tx, userID int64, ids []int64) error {
	for i, id := range ids {
		pos := i + 1
		res, err := tx.ExecContext(ctx,
			`UPDATE columns SET position = $1, updated_at = now() WHERE id = $2 AND user_id = $3`,
			pos, id, userID)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrColumnNotFound
		}
	}
	return nil
}

// HasAnyColumns returns true when the user already has at least one column.
func (r *ColumnRepo) HasAnyColumns(ctx context.Context, userID int64) (bool, error) {
	var dummy int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM columns WHERE user_id = $1 LIMIT 1`, userID).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// InsertDefaults inserts the 3 bootstrap columns inside an already-open transaction.
func (r *ColumnRepo) InsertDefaults(ctx context.Context, tx *sql.Tx, userID int64) error {
	defaults := []struct {
		name string
		pos  int
	}{
		{"To Do", 1},
		{"Doing", 2},
		{"Done", 3},
	}
	for _, d := range defaults {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO columns (user_id, name, position) VALUES ($1, $2, $3)`,
			userID, d.name, d.pos)
		if err != nil {
			return err
		}
	}
	return nil
}

// BeginTx opens a new transaction on the underlying db.
func (r *ColumnRepo) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}
