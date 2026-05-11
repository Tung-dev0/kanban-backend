package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ivantung/todo-backend/internal/model"
	"github.com/ivantung/todo-backend/internal/repository"
)

var (
	ErrCardNotFound = errors.New("card not found")
	ErrInvalidLabel = errors.New("invalid label color")
	// ErrInvalidInput is shared across services; already declared in column.go
)

// validLabelColors is the fixed set allowed by the spec.
var validLabelColors = map[string]bool{
	"red":    true,
	"orange": true,
	"yellow": true,
	"green":  true,
	"blue":   true,
	"purple": true,
}

// CardService handles card business rules.
type CardService struct {
	cards   *repository.CardRepo
	columns *repository.ColumnRepo
}

func NewCardService(cards *repository.CardRepo, columns *repository.ColumnRepo) *CardService {
	return &CardService{cards: cards, columns: columns}
}

// Create validates fields, checks column ownership, and inserts the card.
func (s *CardService) Create(ctx context.Context, userID int64, columnID int64, title, description string, dueAt *time.Time) (*model.Card, error) {
	title = strings.TrimSpace(title)
	if err := validateCardTitle(title); err != nil {
		return nil, err
	}
	if err := validateDescription(description); err != nil {
		return nil, err
	}

	// Ensure target column is owned by the user
	if _, err := s.columns.GetByID(ctx, userID, columnID); err != nil {
		if errors.Is(err, repository.ErrColumnNotFound) {
			return nil, ErrColumnNotFound
		}
		return nil, err
	}

	return s.cards.Create(ctx, columnID, title, description, dueAt)
}

// Get fetches a card, returning ErrCardNotFound if not owned.
func (s *CardService) Get(ctx context.Context, userID, id int64) (*model.Card, error) {
	card, err := s.cards.GetForUser(ctx, userID, id)
	if errors.Is(err, repository.ErrCardNotFound) {
		return nil, ErrCardNotFound
	}
	return card, err
}

// UpdatePatch carries the fields for a partial card update.
// Use pointer fields: nil = not provided. DueAtSet separates "not provided" from "provided as null".
type UpdatePatch struct {
	ColumnID    *int64
	Title       *string
	Description *string
	DueAt       *time.Time
	DueAtSet    bool
}

// Update applies a partial update. Validates any provided fields and checks ownership
// of the target column if column_id changes.
func (s *CardService) Update(ctx context.Context, userID, id int64, patch UpdatePatch) (*model.Card, error) {
	if patch.Title != nil {
		trimmed := strings.TrimSpace(*patch.Title)
		if err := validateCardTitle(trimmed); err != nil {
			return nil, err
		}
		patch.Title = &trimmed
	}
	if patch.Description != nil {
		if err := validateDescription(*patch.Description); err != nil {
			return nil, err
		}
	}
	if patch.ColumnID != nil {
		// Validate new column ownership
		if _, err := s.columns.GetByID(ctx, userID, *patch.ColumnID); err != nil {
			if errors.Is(err, repository.ErrColumnNotFound) {
				return nil, ErrColumnNotFound
			}
			return nil, err
		}
	}

	card, err := s.cards.Update(ctx, userID, id, repository.CardPatch{
		ColumnID:    patch.ColumnID,
		Title:       patch.Title,
		Description: patch.Description,
		DueAt:       patch.DueAt,
		DueAtSet:    patch.DueAtSet,
	})
	if errors.Is(err, repository.ErrCardNotFound) {
		return nil, ErrCardNotFound
	}
	return card, err
}

// Delete hard-deletes a card owned by userID.
func (s *CardService) Delete(ctx context.Context, userID, id int64) error {
	err := s.cards.Delete(ctx, userID, id)
	if errors.Is(err, repository.ErrCardNotFound) {
		return ErrCardNotFound
	}
	return err
}

// SetLabels replaces all labels. Colors are validated, duplicates silently deduped.
func (s *CardService) SetLabels(ctx context.Context, userID, cardID int64, colors []string) ([]string, error) {
	// Validate colors and dedupe
	seen := make(map[string]bool)
	deduped := make([]string, 0, len(colors))
	for _, c := range colors {
		if !validLabelColors[c] {
			return nil, fmt.Errorf("%w: %q is not a valid label color", ErrInvalidLabel, c)
		}
		if !seen[c] {
			seen[c] = true
			deduped = append(deduped, c)
		}
	}

	// Verify card ownership
	if _, err := s.cards.GetForUser(ctx, userID, cardID); err != nil {
		if errors.Is(err, repository.ErrCardNotFound) {
			return nil, ErrCardNotFound
		}
		return nil, err
	}

	if err := s.cards.SetLabels(ctx, cardID, deduped); err != nil {
		return nil, err
	}
	return deduped, nil
}

func validateCardTitle(title string) error {
	n := utf8.RuneCountInString(title)
	if n < 1 || n > 200 {
		return fmt.Errorf("%w: card title must be 1-200 chars", ErrInvalidInput)
	}
	return nil
}

func validateDescription(desc string) error {
	if utf8.RuneCountInString(desc) > 10000 {
		return fmt.Errorf("%w: description must be at most 10000 chars", ErrInvalidInput)
	}
	return nil
}
