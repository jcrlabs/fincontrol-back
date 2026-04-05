package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// ScheduledService manages recurring transaction templates.
type ScheduledService struct {
	repo   ScheduleRepository
	ledger LedgerRepository
	audit  AuditRepository
}

func NewScheduledService(repo ScheduleRepository, ledger LedgerRepository, audit AuditRepository) *ScheduledService {
	return &ScheduledService{repo: repo, ledger: ledger, audit: audit}
}

type CreateScheduledInput struct {
	UserID      uuid.UUID
	Description string
	Frequency   domain.Frequency
	NextRun     time.Time
	Entries     []domain.ScheduledEntry
}

func (s *ScheduledService) Create(ctx context.Context, input CreateScheduledInput) (domain.ScheduledTransaction, error) {
	if len(input.Entries) < 2 {
		return domain.ScheduledTransaction{}, fmt.Errorf("%w: minimum 2 entries required", domain.ErrInvalidInput)
	}
	st := domain.ScheduledTransaction{
		ID:              uuid.New(),
		UserID:          input.UserID,
		Description:     input.Description,
		Frequency:       input.Frequency,
		NextRun:         input.NextRun,
		IsActive:        true,
		TemplateEntries: input.Entries,
		CreatedAt:       time.Now().UTC(),
	}
	return s.repo.Create(ctx, st)
}

func (s *ScheduledService) List(ctx context.Context, userID uuid.UUID) ([]domain.ScheduledTransaction, error) {
	return s.repo.ListByUser(ctx, userID)
}

type UpdateScheduledInput struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Frequency domain.Frequency
	NextRun   time.Time
	IsActive  bool
}

func (s *ScheduledService) Update(ctx context.Context, input UpdateScheduledInput) (domain.ScheduledTransaction, error) {
	all, err := s.repo.ListByUser(ctx, input.UserID)
	if err != nil {
		return domain.ScheduledTransaction{}, err
	}
	var existing *domain.ScheduledTransaction
	for i := range all {
		if all[i].ID == input.ID {
			existing = &all[i]
			break
		}
	}
	if existing == nil {
		return domain.ScheduledTransaction{}, domain.ErrNotFound
	}
	if input.Frequency != "" {
		existing.Frequency = input.Frequency
	}
	if !input.NextRun.IsZero() {
		existing.NextRun = input.NextRun
	}
	existing.IsActive = input.IsActive
	return s.repo.Update(ctx, *existing)
}

func (s *ScheduledService) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.Delete(ctx, id, userID)
}

// ProcessDue creates journal entries for all scheduled transactions that are due.
// Called by the scheduler goroutine every minute.
func (s *ScheduledService) ProcessDue(ctx context.Context) error {
	due, err := s.repo.ListDue(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("list due: %w", err)
	}

	for _, st := range due {
		if err := s.processSingle(ctx, st); err != nil {
			// Log but continue — don't let one failure block the rest
			fmt.Printf("process scheduled %s: %v\n", st.ID, err)
			continue
		}
	}
	return nil
}

func (s *ScheduledService) processSingle(ctx context.Context, st domain.ScheduledTransaction) error {
	entries := make([]EntryInput, 0, len(st.TemplateEntries))
	for _, te := range st.TemplateEntries {
		amount, err := decimal.NewFromString(te.Amount)
		if err != nil {
			return fmt.Errorf("parse template amount: %w", err)
		}
		entries = append(entries, EntryInput{
			AccountID: te.AccountID,
			Amount:    amount,
			Currency:  te.Currency,
		})
	}

	_, err := s.ledger.CreateJournalEntry(ctx, CreateTransactionInput{
		UserID:      st.UserID,
		Description: st.Description,
		Date:        time.Now().UTC(),
		Entries:     entries,
	})
	if err != nil {
		return fmt.Errorf("create journal entry: %w", err)
	}

	// Advance next_run based on frequency
	st.NextRun = advanceNextRun(st.NextRun, st.Frequency)
	if _, err := s.repo.Update(ctx, st); err != nil {
		return fmt.Errorf("advance next_run: %w", err)
	}

	s.audit.Log(ctx, domain.AuditEntry{ //nolint:gosec
		UserID:     st.UserID,
		Action:     "process_scheduled",
		EntityType: "scheduled_transaction",
		EntityID:   st.ID,
	})

	return nil
}

func advanceNextRun(from time.Time, freq domain.Frequency) time.Time {
	switch freq {
	case domain.FrequencyDaily:
		return from.AddDate(0, 0, 1)
	case domain.FrequencyWeekly:
		return from.AddDate(0, 0, 7)
	case domain.FrequencyMonthly:
		return from.AddDate(0, 1, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}
