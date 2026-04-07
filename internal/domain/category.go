package domain

import (
	"time"

	"github.com/google/uuid"
)

// Category supports hierarchical classification via parent_id (N levels deep).
type Category struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	ParentID  *uuid.UUID
	Icon      string
	CreatedAt time.Time
}
