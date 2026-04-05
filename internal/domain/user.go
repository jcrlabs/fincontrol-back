package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is the authentication entity.
type User struct {
	ID           uuid.UUID
	Email        string
	Name         string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
