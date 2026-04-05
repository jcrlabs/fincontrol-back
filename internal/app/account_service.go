package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// AccountService handles business logic for account management.
type AccountService struct {
	repo  AccountRepository
	audit AuditRepository
}

func NewAccountService(repo AccountRepository, audit AuditRepository) *AccountService {
	return &AccountService{repo: repo, audit: audit}
}

type CreateAccountInput struct {
	UserID         uuid.UUID
	Name           string
	Type           domain.AccountType
	Currency       string
	InitialBalance decimal.Decimal
}

func (s *AccountService) Create(ctx context.Context, input CreateAccountInput) (domain.Account, error) {
	if !input.Type.IsValid() {
		return domain.Account{}, fmt.Errorf("%w: invalid account type %q", domain.ErrInvalidInput, input.Type)
	}
	if input.Name == "" {
		return domain.Account{}, fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if input.Currency == "" {
		input.Currency = "EUR"
	}

	account := domain.Account{
		ID:        uuid.New(),
		UserID:    input.UserID,
		Name:      input.Name,
		Type:      input.Type,
		Currency:  input.Currency,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
	}

	created, err := s.repo.Create(ctx, account)
	if err != nil {
		return domain.Account{}, err
	}

	s.audit.Log(ctx, domain.AuditEntry{ //nolint:gosec
		UserID:     input.UserID,
		Action:     "create_account",
		EntityType: "account",
		EntityID:   created.ID,
	})

	return created, nil
}

func (s *AccountService) GetByID(ctx context.Context, id, userID uuid.UUID) (domain.Account, error) {
	account, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return domain.Account{}, err
	}
	balance, err := s.repo.GetBalance(ctx, id, userID)
	if err != nil {
		return domain.Account{}, err
	}
	account.Balance = balance
	return account, nil
}

func (s *AccountService) List(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	accounts, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		balance, err := s.repo.GetBalance(ctx, accounts[i].ID, userID)
		if err != nil {
			return nil, fmt.Errorf("get balance for account %s: %w", accounts[i].ID, err)
		}
		accounts[i].Balance = balance
	}
	return accounts, nil
}

type UpdateAccountInput struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Name   string
}

func (s *AccountService) Update(ctx context.Context, input UpdateAccountInput) (domain.Account, error) {
	account, err := s.repo.GetByID(ctx, input.ID, input.UserID)
	if err != nil {
		return domain.Account{}, err
	}
	if input.Name != "" {
		account.Name = input.Name
	}
	return s.repo.Update(ctx, account)
}
