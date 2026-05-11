package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ivantung/todo-backend/internal/model"
	"github.com/ivantung/todo-backend/internal/repository"
)

var (
	ErrColumnNotFound  = errors.New("column not found")
	ErrColumnNotEmpty  = errors.New("column not empty")
	ErrReorderMismatch = errors.New("reorder id set does not match current columns")
)

// ColumnNotEmptyError carries the card count for the 409 response body.
type ColumnNotEmptyError struct {
	Count int
}

func (e *ColumnNotEmptyError) Error() string {
	return fmt.Sprintf("column has %d cards", e.Count)
}

func (e *ColumnNotEmptyError) Is(target error) bool {
	return target == ErrColumnNotEmpty
}

// ColumnService wraps the column repository with business rules.
type ColumnService struct {
	columns *repository.ColumnRepo
}

func NewColumnService(columns *repository.ColumnRepo) *ColumnService {
	return &ColumnService{columns: columns}
}

// Create validates the name and appends a new column.
func (s *ColumnService) Create(ctx context.Context, userID int64, name string) (*model.Column, error) {
	name = strings.TrimSpace(name)
	if err := validateColumnName(name); err != nil {
		return nil, err
	}
	return s.columns.Create(ctx, userID, name)
}

// Rename validates and renames a column.
func (s *ColumnService) Rename(ctx context.Context, userID, id int64, name string) (*model.Column, error) {
	name = strings.TrimSpace(name)
	if err := validateColumnName(name); err != nil {
		return nil, err
	}
	col, err := s.columns.Update(ctx, userID, id, name)
	if errors.Is(err, repository.ErrColumnNotFound) {
		return nil, ErrColumnNotFound
	}
	return col, err
}

// Delete removes a column if it has no cards.
func (s *ColumnService) Delete(ctx context.Context, userID, id int64) error {
	// Verify ownership first
	_, err := s.columns.GetByID(ctx, userID, id)
	if errors.Is(err, repository.ErrColumnNotFound) {
		return ErrColumnNotFound
	}
	if err != nil {
		return err
	}

	n, err := s.columns.CountCards(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return &ColumnNotEmptyError{Count: n}
	}
	if err := s.columns.Delete(ctx, userID, id); errors.Is(err, repository.ErrColumnNotFound) {
		return ErrColumnNotFound
	} else {
		return err
	}
}

// Reorder reassigns positions such that columns are in the requested order.
// The provided ids must be exactly the user's current column set — no add/remove.
type ReorderResult struct {
	Columns []model.Column `json:"columns"`
}

func (s *ColumnService) Reorder(ctx context.Context, userID int64, ids []int64) (*ReorderResult, error) {
	current, err := s.columns.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(ids) != len(current) {
		return nil, ErrReorderMismatch
	}

	// Build set of current IDs
	currentSet := make(map[int64]bool, len(current))
	for _, c := range current {
		currentSet[c.ID] = true
	}
	// Validate incoming IDs match exactly
	inSet := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if !currentSet[id] {
			return nil, ErrReorderMismatch
		}
		if inSet[id] {
			return nil, ErrReorderMismatch // duplicate
		}
		inSet[id] = true
	}

	tx, err := s.columns.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := s.columns.ReorderInTx(ctx, tx, userID, ids); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Re-fetch updated columns
	updated, err := s.columns.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Sort by new position for deterministic output
	sort.Slice(updated, func(i, j int) bool { return updated[i].Position < updated[j].Position })

	return &ReorderResult{Columns: updated}, nil
}

// CardCount returns the number of cards in a column (used by handler to build 409 message).
func (s *ColumnService) CardCount(ctx context.Context, columnID int64) (int, error) {
	return s.columns.CountCards(ctx, columnID)
}

func validateColumnName(name string) error {
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 60 {
		return fmt.Errorf("%w: column name must be 1-60 chars", ErrInvalidInput)
	}
	return nil
}
