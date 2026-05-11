package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ivantung/todo-backend/internal/model"
	"github.com/ivantung/todo-backend/internal/repository"
)

// BoardService handles the single-user Kanban board.
type BoardService struct {
	columns *repository.ColumnRepo
	cards   *repository.CardRepo
}

func NewBoardService(columns *repository.ColumnRepo, cards *repository.CardRepo) *BoardService {
	return &BoardService{columns: columns, cards: cards}
}

// GetOrInit returns the user's board, creating 3 default columns on first call.
// It is safe for concurrent first calls: the UNIQUE(user_id, position) constraint
// causes a 23505 on the race loser, which re-fetches instead.
func (s *BoardService) GetOrInit(ctx context.Context, userID int64) (*model.Board, error) {
	// Fast path: any existing column means board already bootstrapped.
	has, err := s.columns.HasAnyColumns(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !has {
		tx, err := s.columns.BeginTx(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback() //nolint:errcheck

		if err := s.columns.InsertDefaults(ctx, tx, userID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				// Another goroutine won the race — just re-fetch below.
				_ = tx.Rollback()
			} else {
				return nil, err
			}
		} else {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
		}
	}

	return s.buildBoard(ctx, userID)
}

// buildBoard assembles columns + cards for the user.
func (s *BoardService) buildBoard(ctx context.Context, userID int64) (*model.Board, error) {
	cols, err := s.columns.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	columnIDs := make([]int64, len(cols))
	for i, c := range cols {
		columnIDs[i] = c.ID
	}

	cardsByCol, err := s.cards.ListByColumnIDs(ctx, columnIDs)
	if err != nil {
		return nil, err
	}

	board := &model.Board{Columns: make([]model.ColumnWithCards, len(cols))}
	for i, col := range cols {
		cards := cardsByCol[col.ID]
		if cards == nil {
			cards = []model.Card{}
		}
		board.Columns[i] = model.ColumnWithCards{
			Column: col,
			Cards:  cards,
		}
	}
	return board, nil
}
