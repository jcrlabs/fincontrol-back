package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// LedgerRepo implements app.LedgerRepository using PostgreSQL.
// All journal operations run in SERIALIZABLE transactions.
type LedgerRepo struct {
	pool *pgxpool.Pool
}

func NewLedgerRepo(pool *pgxpool.Pool) *LedgerRepo {
	return &LedgerRepo{pool: pool}
}

func (r *LedgerRepo) CreateJournalEntry(ctx context.Context, input app.CreateTransactionInput) (domain.JournalEntry, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.JournalEntry{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	journalID := uuid.New()
	var createdAt time.Time

	err = tx.QueryRow(ctx, `
		INSERT INTO journal_entries (id, user_id, description, date, category_id, is_reversal)
		VALUES ($1, $2, $3, $4, $5, false)
		RETURNING created_at
	`, journalID, input.UserID, input.Description, input.Date, input.CategoryID,
	).Scan(&createdAt)
	if err != nil {
		return domain.JournalEntry{}, fmt.Errorf("insert journal header: %w", err)
	}

	entries := make([]domain.Entry, 0, len(input.Entries))
	for _, e := range input.Entries {
		entryID := uuid.New()
		var entryCreatedAt time.Time
		err = tx.QueryRow(ctx, `
			INSERT INTO entries (id, journal_entry_id, account_id, amount, currency)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING created_at
		`, entryID, journalID, e.AccountID, e.Amount, e.Currency,
		).Scan(&entryCreatedAt)
		if err != nil {
			return domain.JournalEntry{}, fmt.Errorf("insert entry: %w", err)
		}
		entries = append(entries, domain.Entry{
			ID:             entryID,
			JournalEntryID: journalID,
			AccountID:      e.AccountID,
			Amount:         e.Amount,
			Currency:       e.Currency,
			CreatedAt:      entryCreatedAt,
		})
	}

	// Defense-in-depth: verify balance in DB (application layer already checked)
	var sum decimal.Decimal
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM entries WHERE journal_entry_id = $1`,
		journalID,
	).Scan(&sum); err != nil || !sum.IsZero() {
		return domain.JournalEntry{}, domain.ErrUnbalanced
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.JournalEntry{}, fmt.Errorf("commit: %w", err)
	}

	return domain.JournalEntry{
		ID:          journalID,
		UserID:      input.UserID,
		Description: input.Description,
		Date:        input.Date,
		CategoryID:  input.CategoryID,
		IsReversal:  false,
		Entries:     entries,
		CreatedAt:   createdAt,
	}, nil
}

func (r *LedgerRepo) VoidJournalEntry(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error) {
	// Fetch original journal + entries
	original, err := r.GetJournalEntry(ctx, id, userID)
	if err != nil {
		return domain.JournalEntry{}, err
	}
	if original.IsReversal {
		return domain.JournalEntry{}, fmt.Errorf("%w: cannot void a reversal entry", domain.ErrInvalidInput)
	}

	// Build reverse entries (flip signs)
	reversalEntries := make([]app.EntryInput, len(original.Entries))
	for i, e := range original.Entries {
		reversalEntries[i] = app.EntryInput{
			AccountID: e.AccountID,
			Amount:    e.Amount.Neg(),
			Currency:  e.Currency,
		}
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.JournalEntry{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	reversalID := uuid.New()
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO journal_entries (id, user_id, description, date, category_id, is_reversal, reversed_entry_id)
		VALUES ($1, $2, $3, now(), $4, true, $5)
		RETURNING created_at
	`, reversalID, userID,
		fmt.Sprintf("VOID: %s", original.Description),
		original.CategoryID,
		original.ID,
	).Scan(&createdAt)
	if err != nil {
		return domain.JournalEntry{}, fmt.Errorf("insert reversal header: %w", err)
	}

	entries := make([]domain.Entry, 0, len(reversalEntries))
	for _, e := range reversalEntries {
		entryID := uuid.New()
		var entryCreatedAt time.Time
		if err := tx.QueryRow(ctx, `
			INSERT INTO entries (id, journal_entry_id, account_id, amount, currency)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING created_at
		`, entryID, reversalID, e.AccountID, e.Amount, e.Currency,
		).Scan(&entryCreatedAt); err != nil {
			return domain.JournalEntry{}, fmt.Errorf("insert reversal entry: %w", err)
		}
		entries = append(entries, domain.Entry{
			ID:             entryID,
			JournalEntryID: reversalID,
			AccountID:      e.AccountID,
			Amount:         e.Amount,
			Currency:       e.Currency,
			CreatedAt:      entryCreatedAt,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.JournalEntry{}, fmt.Errorf("commit void: %w", err)
	}

	reversedID := original.ID
	return domain.JournalEntry{
		ID:              reversalID,
		UserID:          userID,
		Description:     fmt.Sprintf("VOID: %s", original.Description),
		Date:            time.Now().UTC(),
		CategoryID:      original.CategoryID,
		IsReversal:      true,
		ReversedEntryID: &reversedID,
		Entries:         entries,
		CreatedAt:       createdAt,
	}, nil
}

func (r *LedgerRepo) GetJournalEntry(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error) {
	var j domain.JournalEntry
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, description, date, category_id, is_reversal, reversed_entry_id, created_at
		FROM journal_entries WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(
		&j.ID, &j.UserID, &j.Description, &j.Date,
		&j.CategoryID, &j.IsReversal, &j.ReversedEntryID, &j.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.JournalEntry{}, domain.ErrNotFound
		}
		return domain.JournalEntry{}, fmt.Errorf("get journal entry: %w", err)
	}

	entries, err := r.loadEntries(ctx, id)
	if err != nil {
		return domain.JournalEntry{}, err
	}
	j.Entries = entries
	return j, nil
}

func (r *LedgerRepo) ListJournalEntries(ctx context.Context, filters app.JournalFilters) ([]domain.JournalEntry, int, error) {
	args := []interface{}{filters.UserID}
	where := "WHERE je.user_id = $1"
	argN := 2

	if filters.DateFrom != nil {
		where += fmt.Sprintf(" AND je.date >= $%d", argN)
		args = append(args, filters.DateFrom)
		argN++
	}
	if filters.DateTo != nil {
		where += fmt.Sprintf(" AND je.date <= $%d", argN)
		args = append(args, filters.DateTo)
		argN++
	}
	if filters.CategoryID != nil {
		where += fmt.Sprintf(" AND je.category_id = $%d", argN)
		args = append(args, filters.CategoryID)
		argN++
	}
	if filters.AccountID != nil {
		where += fmt.Sprintf(" AND EXISTS(SELECT 1 FROM entries e WHERE e.journal_entry_id = je.id AND e.account_id = $%d)", argN)
		args = append(args, filters.AccountID)
		argN++
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM journal_entries je %s", where),
		args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count journal entries: %w", err)
	}

	page := filters.Page
	if page < 1 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT je.id, je.user_id, je.description, je.date, je.category_id,
		       je.is_reversal, je.reversed_entry_id, je.created_at
		FROM journal_entries je %s
		ORDER BY je.date DESC, je.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argN, argN+1)
	args = append(args, pageSize, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list journal entries: %w", err)
	}
	defer rows.Close()

	var journals []domain.JournalEntry
	for rows.Next() {
		var j domain.JournalEntry
		if err := rows.Scan(
			&j.ID, &j.UserID, &j.Description, &j.Date,
			&j.CategoryID, &j.IsReversal, &j.ReversedEntryID, &j.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan journal entry: %w", err)
		}
		journals = append(journals, j)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Load entries for each journal
	for i := range journals {
		entries, err := r.loadEntries(ctx, journals[i].ID)
		if err != nil {
			return nil, 0, err
		}
		journals[i].Entries = entries
	}

	return journals, total, nil
}

func (r *LedgerRepo) ListEntriesByAccount(ctx context.Context, accountID, userID uuid.UUID, page, pageSize int) ([]domain.Entry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Verify account ownership
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2)`,
		accountID, userID,
	).Scan(&exists); err != nil || !exists {
		return nil, 0, domain.ErrNotFound
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM entries WHERE account_id = $1`,
		accountID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count entries: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, journal_entry_id, account_id, amount, currency, created_at
		FROM entries WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, accountID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list entries by account: %w", err)
	}
	defer rows.Close()

	var entries []domain.Entry
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(&e.ID, &e.JournalEntryID, &e.AccountID, &e.Amount, &e.Currency, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

func (r *LedgerRepo) loadEntries(ctx context.Context, journalID uuid.UUID) ([]domain.Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, journal_entry_id, account_id, amount, currency, created_at
		FROM entries WHERE journal_entry_id = $1
	`, journalID)
	if err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.Entry
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(&e.ID, &e.JournalEntryID, &e.AccountID, &e.Amount, &e.Currency, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
