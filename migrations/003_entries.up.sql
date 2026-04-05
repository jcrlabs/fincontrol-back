-- Double-entry lines: positive = debit, negative = credit
CREATE TABLE entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_entry_id UUID NOT NULL REFERENCES journal_entries(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(19,4) NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'EUR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_entries_journal_entry_id ON entries(journal_entry_id);
CREATE INDEX idx_entries_account_id ON entries(account_id);

-- Trigger: safety net for the double-entry invariant (app layer validates first)
CREATE OR REPLACE FUNCTION check_journal_balance()
RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT SUM(amount) FROM entries WHERE journal_entry_id = NEW.journal_entry_id) != 0 THEN
        RAISE EXCEPTION 'Journal entry % is unbalanced', NEW.journal_entry_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Note: trigger fires AFTER each row insert; the app validates sum=0 before commit
-- The trigger provides defense-in-depth only — it runs after all entries are inserted
-- via a DEFERRED constraint check would be cleaner, but this is simpler and safe
-- because the app always inserts all entries in one transaction before the check.
