CREATE TYPE frequency AS ENUM ('daily', 'weekly', 'monthly');

CREATE TABLE scheduled_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    description VARCHAR(500) NOT NULL,
    frequency frequency NOT NULL,
    next_run TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    template_entries JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_scheduled_transactions_user_id ON scheduled_transactions(user_id);
CREATE INDEX idx_scheduled_transactions_next_run ON scheduled_transactions(next_run) WHERE is_active = true;
