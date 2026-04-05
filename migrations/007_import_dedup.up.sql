-- Tracks imported rows to prevent duplicates.
-- Hash is derived from: hash(date || amount || description) per user.
CREATE TABLE import_dedup (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    row_hash VARCHAR(64) NOT NULL,
    journal_entry_id UUID REFERENCES journal_entries(id),
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, row_hash)
);

CREATE INDEX idx_import_dedup_user_id ON import_dedup(user_id);
