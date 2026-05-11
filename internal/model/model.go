package model

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        *string   `json:"email,omitempty"`
	DisplayName  *string   `json:"display_name,omitempty"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	GoogleID     *string   `json:"-"`
	PasswordHash *string   `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Column is a per-user ordered column on the Kanban board.
type Column struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Card belongs to a column (and transitively to the column's user).
type Card struct {
	ID          int64      `json:"id"`
	ColumnID    int64      `json:"column_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueAt       *time.Time `json:"due_at"`
	Labels      []string   `json:"labels"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ColumnWithCards is a column together with its ordered cards — used in the
// GET /api/board response.
type ColumnWithCards struct {
	Column
	Cards []Card `json:"cards"`
}

// Board is the full GET /api/board payload.
type Board struct {
	Columns []ColumnWithCards `json:"columns"`
}
