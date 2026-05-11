package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ivantung/todo-backend/internal/model"
)

var ErrCardNotFound = errors.New("card not found")

const cardColumns = `c.id, c.column_id, c.title, c.description, c.due_at, c.created_at, c.updated_at`

type CardRepo struct{ db *sql.DB }

func NewCardRepo(db *sql.DB) *CardRepo { return &CardRepo{db: db} }

// scanCardRow scans a card row (without labels).
func scanCardRow(row interface{ Scan(...any) error }) (*model.Card, error) {
	card := &model.Card{}
	var dueAt sql.NullTime
	err := row.Scan(
		&card.ID,
		&card.ColumnID,
		&card.Title,
		&card.Description,
		&dueAt,
		&card.CreatedAt,
		&card.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if dueAt.Valid {
		card.DueAt = &dueAt.Time
	}
	card.Labels = []string{}
	return card, nil
}

// Create inserts a new card in the specified column.
// The column must be owned by userID — ownership check is done by the caller (service).
func (r *CardRepo) Create(ctx context.Context, columnID int64, title, description string, dueAt *time.Time) (*model.Card, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO cards (column_id, title, description, due_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		columnID, title, description, dueAt,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	card, err := r.getByIDRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	card.Labels = []string{}
	return card, nil
}

// GetForUser fetches a card by ID, ensuring the card's column is owned by userID.
// Returns ErrCardNotFound if not found or not owned.
func (r *CardRepo) GetForUser(ctx context.Context, userID, id int64) (*model.Card, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+cardColumns+`
		FROM cards c
		JOIN columns col ON col.id = c.column_id
		WHERE c.id = $1 AND col.user_id = $2`,
		id, userID)
	card, err := scanCardRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	if err != nil {
		return nil, err
	}
	labels, err := r.loadLabels(ctx, id)
	if err != nil {
		return nil, err
	}
	card.Labels = labels
	return card, nil
}

// ListByColumnIDs loads all cards for a set of column IDs, ordered newest first.
// Returns a map from column_id → []Card (with labels populated).
func (r *CardRepo) ListByColumnIDs(ctx context.Context, columnIDs []int64) (map[int64][]model.Card, error) {
	if len(columnIDs) == 0 {
		return map[int64][]model.Card{}, nil
	}

	placeholders := make([]string, len(columnIDs))
	args := make([]any, len(columnIDs))
	for i, id := range columnIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT `+cardColumns+`
		FROM cards c
		WHERE c.column_id IN (%s)
		ORDER BY c.created_at DESC, c.id DESC`,
		strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]model.Card)
	var cardIDs []int64

	for rows.Next() {
		card, err := scanCardRow(rows)
		if err != nil {
			return nil, err
		}
		result[card.ColumnID] = append(result[card.ColumnID], *card)
		cardIDs = append(cardIDs, card.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(cardIDs) > 0 {
		if err := r.loadLabelsInto(ctx, cardIDs, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// CardPatch holds the optional fields for Update.
// DueAtSet signals that the caller explicitly provided a due_at value (even if nil = clear).
type CardPatch struct {
	ColumnID    *int64
	Title       *string
	Description *string
	DueAt       *time.Time
	DueAtSet    bool // true when due_at key was present in request (even if null)
}

// Update applies a partial update to a card, checking ownership via userID.
func (r *CardRepo) Update(ctx context.Context, userID, id int64, patch CardPatch) (*model.Card, error) {
	existing, err := r.GetForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	newColumnID := existing.ColumnID
	if patch.ColumnID != nil {
		newColumnID = *patch.ColumnID
	}
	newTitle := existing.Title
	if patch.Title != nil {
		newTitle = *patch.Title
	}
	newDescription := existing.Description
	if patch.Description != nil {
		newDescription = *patch.Description
	}

	// due_at: if DueAtSet=false keep existing, else use patch.DueAt (may be nil = clear)
	var newDueAt *time.Time
	if patch.DueAtSet {
		newDueAt = patch.DueAt
	} else {
		newDueAt = existing.DueAt
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE cards
		SET column_id = $1, title = $2, description = $3, due_at = $4, updated_at = now()
		WHERE id = $5`,
		newColumnID, newTitle, newDescription, newDueAt, id)
	if err != nil {
		return nil, err
	}

	card, err := r.getByIDRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	labels, err := r.loadLabels(ctx, id)
	if err != nil {
		return nil, err
	}
	card.Labels = labels
	return card, nil
}

// Delete hard-deletes a card, checking ownership via the column's user_id.
func (r *CardRepo) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM cards
		WHERE id = $1
		  AND column_id IN (SELECT id FROM columns WHERE user_id = $2)`,
		id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrCardNotFound
	}
	return nil
}

// SetLabels replaces all labels on a card.
// Caller must verify card ownership before calling.
func (r *CardRepo) SetLabels(ctx context.Context, cardID int64, colors []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM card_labels WHERE card_id = $1`, cardID); err != nil {
		return err
	}
	for _, color := range colors {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO card_labels (card_id, color) VALUES ($1, $2)`,
			cardID, color); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// getByIDRaw fetches a card by its raw ID without a user ownership check.
func (r *CardRepo) getByIDRaw(ctx context.Context, id int64) (*model.Card, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+cardColumns+` FROM cards c WHERE c.id = $1`, id)
	card, err := scanCardRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCardNotFound
	}
	return card, err
}

// loadLabels returns the label colors for a single card.
func (r *CardRepo) loadLabels(ctx context.Context, cardID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT color FROM card_labels WHERE card_id = $1 ORDER BY color`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var color string
		if err := rows.Scan(&color); err != nil {
			return nil, err
		}
		out = append(out, color)
	}
	return out, rows.Err()
}

// loadLabelsInto bulk-loads labels for the given card IDs and writes them into result.
func (r *CardRepo) loadLabelsInto(ctx context.Context, cardIDs []int64, result map[int64][]model.Card) error {
	placeholders := make([]string, len(cardIDs))
	args := make([]any, len(cardIDs))
	for i, id := range cardIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT card_id, color FROM card_labels WHERE card_id IN (%s) ORDER BY card_id, color`,
		strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	labelMap := make(map[int64][]string)
	for rows.Next() {
		var cid int64
		var color string
		if err := rows.Scan(&cid, &color); err != nil {
			return err
		}
		labelMap[cid] = append(labelMap[cid], color)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for colID, cards := range result {
		for i := range cards {
			if lbls, ok := labelMap[cards[i].ID]; ok {
				cards[i].Labels = lbls
			}
		}
		result[colID] = cards
	}
	return nil
}
