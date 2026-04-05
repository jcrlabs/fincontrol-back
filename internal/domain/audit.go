package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditEntry is an append-only record of a user action.
// The audit_log table has a trigger that prevents UPDATE and DELETE.
type AuditEntry struct {
	UserID     uuid.UUID
	Action     string
	EntityType string
	EntityID   uuid.UUID
	Payload    json.RawMessage
	CreatedAt  time.Time
}
