# CLAUDE.md — FinControl Backend (fin.jcrlabs.net)

> Extiende: `SHARED-CLAUDE.md` (sección Go + Hexagonal)

## Project Overview

Control financiero personal estilo banking con double-entry accounting. Cada transacción afecta mínimo 2 cuentas, el sum de entries siempre = 0. Multi-cuenta, categorías jerárquicas, presupuestos, informes financieros, transacciones recurrentes, import CSV.

## Tech Stack

- **Language**: Go 1.26
- **HTTP**: `net/http` stdlib (enhanced routing Go 1.22+)
- **DB**: PostgreSQL 17 — `github.com/jackc/pgx/v5` (pool + explicit transactions)
- **Auth**: `github.com/golang-jwt/jwt/v5`
- **Scheduler**: goroutine con `time.Ticker` para transacciones recurrentes
- **CSV/OFX**: `encoding/csv` stdlib + custom OFX parser
- **Logging**: `log/slog` stdlib
- **Metrics**: `github.com/prometheus/client_golang`

## Architecture (Hexagonal)

```
fincontrol-back/
├── cmd/server/main.go
├── internal/
│   ├── domain/                          # ── PURE DOMAIN (cero imports externos) ──
│   │   ├── account.go                   # Account entity + AccountType enum (asset/liability/equity/income/expense)
│   │   ├── journal_entry.go             # JournalEntry (header) + Entry[] (lines)
│   │   │                                # INVARIANTE: sum(entries.amount) == 0 SIEMPRE
│   │   ├── entry.go                     # Entry: account_id, amount (+ debit, - credit), currency
│   │   ├── category.go                  # Category con parent_id (árbol N niveles)
│   │   ├── budget.go                    # Budget: category, month, amount, alert thresholds
│   │   ├── scheduled.go                 # ScheduledTransaction: cron expression, template entries
│   │   ├── currency.go                  # Currency + ExchangeRate value object
│   │   ├── money.go                     # Money value object: amount (decimal) + currency
│   │   │                                # Usar github.com/shopspring/decimal (NO float64 para dinero)
│   │   └── errors.go                    # ErrUnbalanced, ErrInsufficientFunds, ErrImmutable, etc.
│   │
│   ├── app/                             # ── APPLICATION SERVICES ──
│   │   ├── account_service.go           # CRUD cuentas, calcular balance
│   │   │   // port: AccountRepository interface
│   │   ├── transaction_service.go       # Crear journal entry (validate → begin tx → insert → commit)
│   │   │   // port: LedgerRepository interface (ACID transactions)
│   │   │   // REGLA: NUNCA delete/update en entries. Solo anular con reverse entry.
│   │   ├── budget_service.go            # CRUD budgets, evaluar alertas (80%/100%)
│   │   │   // port: BudgetRepository interface
│   │   ├── report_service.go            # P&L, balance sheet, cash flow, category breakdown
│   │   │   // port: ReportRepository interface (queries agregadas)
│   │   ├── import_service.go            # Parse CSV/OFX → crear journal entries batch
│   │   │   // port: ImportParser interface (CSV, OFX adapters)
│   │   └── scheduler_service.go         # Evaluar scheduled transactions → crear entries
│   │       // port: ScheduleRepository interface
│   │
│   ├── adapter/
│   │   ├── inbound/
│   │   │   ├── http/
│   │   │   │   ├── server.go
│   │   │   │   ├── account_handler.go       # CRUD /api/accounts
│   │   │   │   ├── transaction_handler.go   # POST /api/transactions, GET history
│   │   │   │   ├── budget_handler.go        # CRUD /api/budgets, GET alerts
│   │   │   │   ├── report_handler.go        # GET /api/reports/pnl, /balance-sheet, /cashflow
│   │   │   │   ├── import_handler.go        # POST /api/import (multipart CSV/OFX)
│   │   │   │   ├── dashboard_handler.go     # GET /api/dashboard (agregados)
│   │   │   │   └── auth_handler.go          # POST /api/auth/login, /refresh
│   │   │   └── scheduler/
│   │   │       └── cron.go                  # Goroutine: tick cada minuto → check scheduled txs
│   │   │
│   │   └── outbound/
│   │       ├── postgres/
│   │       │   ├── account_repo.go
│   │       │   ├── ledger_repo.go           # INSERT journal_entry + entries en 1 TX
│   │       │   │   # pgx.BeginTx → INSERT journal → INSERT entries → CHECK sum=0 → COMMIT
│   │       │   ├── budget_repo.go
│   │       │   ├── report_repo.go           # SQL agregados: SUM, GROUP BY month/category
│   │       │   ├── schedule_repo.go
│   │       │   └── audit_repo.go            # Append-only audit log
│   │       └── parser/
│   │           ├── csv_parser.go            # Implements ImportParser para CSV
│   │           └── ofx_parser.go            # Implements ImportParser para OFX
│   │
│   ├── middleware/                           # Standard (ver shared principles)
│   └── config/
│
├── migrations/
│   ├── 001_accounts.up.sql
│   ├── 002_journal_entries.up.sql
│   ├── 003_entries.up.sql                   # CHECK constraint: sum per journal = 0
│   ├── 004_categories.up.sql                # Recursive tree con parent_id
│   ├── 005_budgets.up.sql
│   ├── 006_scheduled_transactions.up.sql
│   └── 007_audit_log.up.sql                 # Append-only, no UPDATE/DELETE trigger
│
├── k8s/
├── deploy/helm/fincontrol-back/
│   ├── values-prod.yaml
│   └── values-test.yaml
├── .golangci.yml
├── Makefile
└── Dockerfile
```

## PostgreSQL Schema (double-entry ledger)

```sql
-- Tipos de cuenta (GAAP)
CREATE TYPE account_type AS ENUM ('asset', 'liability', 'equity', 'income', 'expense');

-- Cuentas
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    type account_type NOT NULL,
    currency CHAR(3) NOT NULL DEFAULT 'EUR',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Cabecera de transacción
CREATE TABLE journal_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    description VARCHAR(500) NOT NULL,
    date DATE NOT NULL,
    category_id UUID REFERENCES categories(id),
    is_reversal BOOLEAN NOT NULL DEFAULT false,
    reversed_entry_id UUID REFERENCES journal_entries(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    -- NUNCA se borra ni modifica. Solo se anula con reverse entry.
);

-- Líneas de asiento (double-entry)
CREATE TABLE entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_entry_id UUID NOT NULL REFERENCES journal_entries(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount NUMERIC(19,4) NOT NULL,  -- positivo = debit, negativo = credit
    currency CHAR(3) NOT NULL DEFAULT 'EUR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CONSTRAINT CRÍTICO: sum de entries por journal SIEMPRE = 0
-- Se verifica en app layer (Go) Y con trigger como safety net
CREATE OR REPLACE FUNCTION check_journal_balance()
RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT SUM(amount) FROM entries WHERE journal_entry_id = NEW.journal_entry_id) != 0 THEN
        RAISE EXCEPTION 'Journal entry % is unbalanced', NEW.journal_entry_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Categorías jerárquicas (árbol)
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    parent_id UUID REFERENCES categories(id),
    icon VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Presupuestos
CREATE TABLE budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    category_id UUID NOT NULL REFERENCES categories(id),
    month DATE NOT NULL,  -- primer día del mes
    amount NUMERIC(19,4) NOT NULL,
    alert_threshold_pct INT NOT NULL DEFAULT 80,
    UNIQUE(user_id, category_id, month)
);

-- Audit log (APPEND-ONLY — trigger previene UPDATE/DELETE)
CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % not allowed', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();
```

## Ejemplo: crear transacción (Go)

```go
// internal/app/transaction_service.go
func (s *TransactionService) CreateTransaction(ctx context.Context, input CreateTransactionInput) (domain.JournalEntry, error) {
    // 1. Validate: sum of entries must = 0
    var sum decimal.Decimal
    for _, e := range input.Entries {
        sum = sum.Add(e.Amount)
    }
    if !sum.IsZero() {
        return domain.JournalEntry{}, domain.ErrUnbalanced
    }

    // 2. Execute in DB transaction (ACID)
    journal, err := s.ledger.CreateJournalEntry(ctx, input)
    if err != nil {
        return domain.JournalEntry{}, fmt.Errorf("create journal: %w", err)
    }

    // 3. Audit log (append-only)
    s.audit.Log(ctx, domain.AuditEntry{
        UserID:     input.UserID,
        Action:     "create_transaction",
        EntityType: "journal_entry",
        EntityID:   journal.ID,
    })

    return journal, nil
}
```

```go
// internal/adapter/outbound/postgres/ledger_repo.go
func (r *LedgerRepo) CreateJournalEntry(ctx context.Context, input CreateTransactionInput) (domain.JournalEntry, error) {
    tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
    if err != nil {
        return domain.JournalEntry{}, fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx) // no-op si ya committed

    // Insert journal header
    var journalID uuid.UUID
    err = tx.QueryRow(ctx,
        `INSERT INTO journal_entries (user_id, description, date, category_id)
         VALUES ($1, $2, $3, $4) RETURNING id`,
        input.UserID, input.Description, input.Date, input.CategoryID,
    ).Scan(&journalID)
    if err != nil {
        return domain.JournalEntry{}, fmt.Errorf("insert journal: %w", err)
    }

    // Insert all entries
    for _, e := range input.Entries {
        _, err = tx.Exec(ctx,
            `INSERT INTO entries (journal_entry_id, account_id, amount, currency)
             VALUES ($1, $2, $3, $4)`,
            journalID, e.AccountID, e.Amount, e.Currency,
        )
        if err != nil {
            return domain.JournalEntry{}, fmt.Errorf("insert entry: %w", err)
        }
    }

    // Safety check (redundant con app validation, pero defense in depth)
    var sum decimal.Decimal
    err = tx.QueryRow(ctx,
        `SELECT COALESCE(SUM(amount), 0) FROM entries WHERE journal_entry_id = $1`,
        journalID,
    ).Scan(&sum)
    if err != nil || !sum.IsZero() {
        return domain.JournalEntry{}, domain.ErrUnbalanced
    }

    if err := tx.Commit(ctx); err != nil {
        return domain.JournalEntry{}, fmt.Errorf("commit: %w", err)
    }

    return domain.JournalEntry{ID: journalID, /* ... */}, nil
}
```

## Reglas de dominio financiero (inviolables)

1. **NUNCA float64 para dinero** — usar `github.com/shopspring/decimal` (precision NUMERIC(19,4))
2. **NUNCA DELETE/UPDATE en entries ni journal_entries** — solo anular con reverse entry
3. **Sum entries por journal = 0** — validar en app + CHECK constraint en DB (defense in depth)
4. **Audit log inmutable** — trigger previene UPDATE/DELETE, solo INSERT
5. **Transactions ACID**: nivel `SERIALIZABLE` para operaciones de ledger
6. **Idempotencia**: import CSV con dedup por hash(date+amount+description) para evitar duplicados

## Reportes SQL (ejemplos)

```sql
-- P&L mensual (ingresos - gastos)
SELECT
    date_trunc('month', je.date) AS month,
    SUM(CASE WHEN a.type = 'income' THEN -e.amount ELSE 0 END) AS income,
    SUM(CASE WHEN a.type = 'expense' THEN e.amount ELSE 0 END) AS expenses,
    SUM(CASE WHEN a.type = 'income' THEN -e.amount ELSE -e.amount END) AS net
FROM entries e
JOIN accounts a ON e.account_id = a.id
JOIN journal_entries je ON e.journal_entry_id = je.id
WHERE a.type IN ('income', 'expense') AND a.user_id = $1
GROUP BY month ORDER BY month;

-- Balance por cuenta
SELECT a.id, a.name, a.type, COALESCE(SUM(e.amount), 0) AS balance
FROM accounts a
LEFT JOIN entries e ON e.account_id = a.id
WHERE a.user_id = $1 AND a.is_active = true
GROUP BY a.id ORDER BY a.type, a.name;
```

## Deploy

- **Dominio**: `fin.jcrlabs.net` (prod), `fin-test.jcrlabs.net` (test)
- **Namespace**: `fincontrol` (prod), `fincontrol-test` (test)
- **PostgreSQL**: PVC 10Gi con WAL archiving
- **CronJob**: pg_dump diario para backups (datos financieros = críticos)
- **HPA**: 2-3 replicas prod, 1 replica test

## CI local

Ejecutar **antes de cada commit** para evitar que lleguen errores a GitHub Actions:

```bash
gofmt -l .                      # no debe mostrar ficheros
go vet ./...
golangci-lint run --timeout=5m
go test -race ./...
```
## Git

- Ramas: `feature/`, `bugfix/`, `hotfix/`, `release/` — sin prefijos adicionales
- Commits: convencional (`feat:`, `fix:`, `chore:`, etc.) — sin mencionar herramientas externas ni agentes en el mensaje
- PRs: título y descripción propios del cambio — sin mencionar herramientas externas ni agentes
- Comentarios y documentación: redactar en primera persona del equipo — sin atribuir autoría a herramientas
