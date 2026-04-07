package domain

import "errors"

var (
	ErrUnbalanced        = errors.New("journal entry is unbalanced: sum of entries must equal zero")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrImmutable         = errors.New("this record is immutable and cannot be modified or deleted")
	ErrNotFound          = errors.New("record not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrConflict          = errors.New("record already exists")
	ErrInvalidInput      = errors.New("invalid input")
)
