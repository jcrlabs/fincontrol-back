package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/shopspring/decimal"
)

// ReportRepo implements app.ReportRepository using PostgreSQL aggregate queries.
type ReportRepo struct {
	pool *pgxpool.Pool
}

func NewReportRepo(pool *pgxpool.Pool) *ReportRepo {
	return &ReportRepo{pool: pool}
}

func (r *ReportRepo) GetProfitAndLoss(ctx context.Context, userID uuid.UUID, from, to time.Time) (app.ProfitAndLoss, error) {
	var income, expenses decimal.Decimal

	err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(e.amount) FILTER (WHERE a.type = 'income' AND e.amount < 0), 0) * -1 AS income,
			COALESCE(SUM(e.amount) FILTER (WHERE a.type = 'expense' AND e.amount > 0), 0)      AS expenses
		FROM entries e
		JOIN accounts a ON e.account_id = a.id
		JOIN journal_entries je ON e.journal_entry_id = je.id
		WHERE a.user_id = $1
		  AND je.date >= $2 AND je.date <= $3
		  AND a.type IN ('income', 'expense')
	`, userID, from, to).Scan(&income, &expenses)
	if err != nil {
		return app.ProfitAndLoss{}, fmt.Errorf("get p&l: %w", err)
	}

	return app.ProfitAndLoss{
		From:     from,
		To:       to,
		Income:   income,
		Expenses: expenses,
		Net:      income.Sub(expenses),
	}, nil
}

func (r *ReportRepo) GetBalanceSheet(ctx context.Context, userID uuid.UUID, asOf time.Time) (app.BalanceSheet, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.name, a.type, COALESCE(SUM(e.amount), 0) AS balance
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		LEFT JOIN journal_entries je ON e.journal_entry_id = je.id AND je.date <= $2
		WHERE a.user_id = $1 AND a.is_active = true
		GROUP BY a.id, a.name, a.type
		ORDER BY a.type, a.name
	`, userID, asOf)
	if err != nil {
		return app.BalanceSheet{}, fmt.Errorf("get balance sheet: %w", err)
	}
	defer rows.Close()

	bs := app.BalanceSheet{AsOf: asOf}
	var totalAssets, totalLiabilities decimal.Decimal

	for rows.Next() {
		var id uuid.UUID
		var name, accountType string
		var balance decimal.Decimal
		if err := rows.Scan(&id, &name, &accountType, &balance); err != nil {
			return app.BalanceSheet{}, fmt.Errorf("scan balance sheet row: %w", err)
		}
		ab := app.AccountBalance{AccountID: id, Name: name, Balance: balance}
		switch accountType {
		case "asset":
			bs.Assets = append(bs.Assets, ab)
			totalAssets = totalAssets.Add(balance)
		case "liability":
			bs.Liabilities = append(bs.Liabilities, ab)
			totalLiabilities = totalLiabilities.Add(balance)
		case "equity":
			bs.Equity = append(bs.Equity, ab)
		}
	}
	if err := rows.Err(); err != nil {
		return app.BalanceSheet{}, err
	}

	bs.NetWorth = totalAssets.Sub(totalLiabilities)
	return bs, nil
}

func (r *ReportRepo) GetCashFlow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]app.CashFlowPeriod, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			date_trunc('month', je.date)                                                         AS month,
			COALESCE(SUM(e.amount) FILTER (WHERE a.type = 'income' AND e.amount < 0), 0) * -1  AS inflow,
			COALESCE(SUM(e.amount) FILTER (WHERE a.type = 'expense' AND e.amount > 0), 0)       AS outflow
		FROM entries e
		JOIN accounts a ON e.account_id = a.id
		JOIN journal_entries je ON e.journal_entry_id = je.id
		WHERE a.user_id = $1
		  AND je.date >= $2 AND je.date <= $3
		  AND a.type IN ('income', 'expense')
		GROUP BY month
		ORDER BY month
	`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get cash flow: %w", err)
	}
	defer rows.Close()

	var periods []app.CashFlowPeriod
	for rows.Next() {
		var p app.CashFlowPeriod
		if err := rows.Scan(&p.Month, &p.Inflow, &p.Outflow); err != nil {
			return nil, fmt.Errorf("scan cash flow row: %w", err)
		}
		p.Net = p.Inflow.Sub(p.Outflow)
		periods = append(periods, p)
	}
	return periods, rows.Err()
}

func (r *ReportRepo) GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]app.CategoryBreakdown, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			c.id, c.name,
			COALESCE(SUM(e.amount) FILTER (WHERE e.amount > 0), 0) AS total
		FROM categories c
		JOIN journal_entries je ON je.category_id = c.id AND je.user_id = $1
		JOIN entries e ON e.journal_entry_id = je.id
		WHERE c.user_id = $1
		  AND je.date >= $2 AND je.date <= $3
		GROUP BY c.id, c.name
		ORDER BY total DESC
	`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get category breakdown: %w", err)
	}
	defer rows.Close()

	var items []app.CategoryBreakdown
	var grandTotal decimal.Decimal
	for rows.Next() {
		var cb app.CategoryBreakdown
		if err := rows.Scan(&cb.CategoryID, &cb.Name, &cb.Total); err != nil {
			return nil, fmt.Errorf("scan category breakdown: %w", err)
		}
		grandTotal = grandTotal.Add(cb.Total)
		items = append(items, cb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Calculate percentages
	for i := range items {
		if grandTotal.GreaterThan(decimal.Zero) {
			items[i].Percentage = items[i].Total.Div(grandTotal).Mul(decimal.NewFromInt(100))
		}
	}
	return items, nil
}
