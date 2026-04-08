package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
)

// NewServer wires all routes and returns an http.Handler with all middleware applied.
func NewServer(
	authHandler *AuthHandler,
	accountHandler *AccountHandler,
	transactionHandler *TransactionHandler,
	categoryHandler *CategoryHandler,
	budgetHandler *BudgetHandler,
	reportHandler *ReportHandler,
	scheduledHandler *ScheduledHandler,
	dashboardHandler *DashboardHandler,
	importHandler *ImportHandler,
	authValidator middleware.TokenValidator,
	corsOrigins []string,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()

	loginLimiter := middleware.NewRateLimiter(5, 15*time.Minute)
	registerLimiter := middleware.NewRateLimiter(3, time.Hour)
	refreshLimiter := middleware.NewRateLimiter(10, 15*time.Minute)

	// Public
	mux.HandleFunc("GET /api/health", HealthHandler)

	mux.Handle("POST /api/v1/auth/register",
		middleware.Limit(registerLimiter)(http.HandlerFunc(authHandler.Register)),
	)
	mux.Handle("POST /api/v1/auth/login",
		middleware.Limit(loginLimiter)(http.HandlerFunc(authHandler.Login)),
	)
	mux.Handle("POST /api/v1/auth/refresh",
		middleware.Limit(refreshLimiter)(http.HandlerFunc(authHandler.Refresh)),
	)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)

	// Protected
	auth := middleware.Auth(authValidator)

	// Accounts
	mux.Handle("GET /api/v1/accounts", auth(http.HandlerFunc(accountHandler.List)))
	mux.Handle("POST /api/v1/accounts", auth(http.HandlerFunc(accountHandler.Create)))
	mux.Handle("GET /api/v1/accounts/{id}", auth(http.HandlerFunc(accountHandler.Get)))
	mux.Handle("PUT /api/v1/accounts/{id}", auth(http.HandlerFunc(accountHandler.Update)))
	mux.Handle("GET /api/v1/accounts/{id}/entries", auth(http.HandlerFunc(accountHandler.ListEntries)))

	// Transactions
	mux.Handle("GET /api/v1/transactions", auth(http.HandlerFunc(transactionHandler.List)))
	mux.Handle("POST /api/v1/transactions", auth(http.HandlerFunc(transactionHandler.Create)))
	mux.Handle("GET /api/v1/transactions/{id}", auth(http.HandlerFunc(transactionHandler.Get)))
	mux.Handle("POST /api/v1/transactions/{id}/void", auth(http.HandlerFunc(transactionHandler.Void)))

	// Categories
	mux.Handle("GET /api/v1/categories", auth(http.HandlerFunc(categoryHandler.List)))
	mux.Handle("POST /api/v1/categories", auth(http.HandlerFunc(categoryHandler.Create)))
	mux.Handle("PUT /api/v1/categories/{id}", auth(http.HandlerFunc(categoryHandler.Update)))

	// Budgets
	mux.Handle("GET /api/v1/budgets", auth(http.HandlerFunc(budgetHandler.List)))
	mux.Handle("POST /api/v1/budgets", auth(http.HandlerFunc(budgetHandler.Create)))
	mux.Handle("PUT /api/v1/budgets/{id}", auth(http.HandlerFunc(budgetHandler.Update)))

	// Reports
	mux.Handle("GET /api/v1/reports/pnl", auth(http.HandlerFunc(reportHandler.ProfitAndLoss)))
	mux.Handle("GET /api/v1/reports/balance-sheet", auth(http.HandlerFunc(reportHandler.BalanceSheet)))
	mux.Handle("GET /api/v1/reports/cash-flow", auth(http.HandlerFunc(reportHandler.CashFlow)))
	mux.Handle("GET /api/v1/reports/categories", auth(http.HandlerFunc(reportHandler.Categories)))

	// Scheduled transactions
	mux.Handle("GET /api/v1/scheduled", auth(http.HandlerFunc(scheduledHandler.List)))
	mux.Handle("POST /api/v1/scheduled", auth(http.HandlerFunc(scheduledHandler.Create)))
	mux.Handle("PUT /api/v1/scheduled/{id}", auth(http.HandlerFunc(scheduledHandler.Update)))
	mux.Handle("DELETE /api/v1/scheduled/{id}", auth(http.HandlerFunc(scheduledHandler.Delete)))

	// Import
	mux.Handle("POST /api/v1/import/preview", auth(http.HandlerFunc(importHandler.Preview)))
	mux.Handle("POST /api/v1/import/confirm", auth(http.HandlerFunc(importHandler.Confirm)))

	// Dashboard
	mux.Handle("GET /api/v1/dashboard", auth(http.HandlerFunc(dashboardHandler.Get)))

	// Global middleware stack
	var h http.Handler = mux
	h = middleware.Logging(logger)(h)
	h = middleware.CORS(corsOrigins)(h)
	h = middleware.Security(h)

	return h
}
