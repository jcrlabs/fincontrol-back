CREATE TABLE budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    category_id UUID NOT NULL REFERENCES categories(id),
    month DATE NOT NULL,  -- stored as first day of the month
    amount NUMERIC(19,4) NOT NULL,
    alert_threshold_pct INT NOT NULL DEFAULT 80,
    UNIQUE(user_id, category_id, month)
);

CREATE INDEX idx_budgets_user_id ON budgets(user_id);
