package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpAdapter "github.com/jonathanCaamano/fincontrol-back/internal/adapter/inbound/http"
	"github.com/jonathanCaamano/fincontrol-back/internal/adapter/inbound/scheduler"
	"github.com/jonathanCaamano/fincontrol-back/internal/adapter/outbound/parser"
	"github.com/jonathanCaamano/fincontrol-back/internal/adapter/outbound/postgres"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(context.Background(), pool, "migrations"); err != nil {
		logger.Error("run migrations", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	// Repositories
	authRepo := postgres.NewAuthRepo(pool)
	accountRepo := postgres.NewAccountRepo(pool)
	ledgerRepo := postgres.NewLedgerRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	categoryRepo := postgres.NewCategoryRepo(pool)
	budgetRepo := postgres.NewBudgetRepo(pool)
	reportRepo := postgres.NewReportRepo(pool)
	scheduleRepo := postgres.NewScheduleRepo(pool)
	importRepo := postgres.NewImportRepo(pool)

	// Services
	authSvc := app.NewAuthService(
		authRepo,
		cfg.JWTPrivateKey,
		cfg.JWTPublicKey,
		cfg.AccessTokenTTL,
		cfg.RefreshTokenTTL,
	)
	accountSvc := app.NewAccountService(accountRepo, auditRepo)
	transactionSvc := app.NewTransactionService(ledgerRepo, auditRepo)
	categorySvc := app.NewCategoryService(categoryRepo)
	budgetSvc := app.NewBudgetService(budgetRepo)
	scheduledSvc := app.NewScheduledService(scheduleRepo, ledgerRepo, auditRepo)
	importSvc := app.NewImportService(importRepo, ledgerRepo, auditRepo, accountRepo)

	// Parsers
	csvParser := parser.NewCSVParser()
	ofxParser := parser.NewOFXParser()

	// Handlers
	authHandler := httpAdapter.NewAuthHandler(authSvc, cfg.RefreshTokenTTL)
	accountHandler := httpAdapter.NewAccountHandler(accountSvc, ledgerRepo)
	transactionHandler := httpAdapter.NewTransactionHandler(transactionSvc)
	categoryHandler := httpAdapter.NewCategoryHandler(categorySvc)
	budgetHandler := httpAdapter.NewBudgetHandler(budgetSvc)
	reportHandler := httpAdapter.NewReportHandler(reportRepo)
	scheduledHandler := httpAdapter.NewScheduledHandler(scheduledSvc)
	dashboardHandler := httpAdapter.NewDashboardHandler(reportRepo, budgetRepo, ledgerRepo, accountRepo)
	importHandler := httpAdapter.NewImportHandler(csvParser, ofxParser, importSvc)

	// Scheduler goroutine
	sched := scheduler.New(scheduledSvc, logger)
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	go sched.Run(schedCtx)

	handler := httpAdapter.NewServer(
		authHandler, accountHandler, transactionHandler,
		categoryHandler, budgetHandler,
		reportHandler, scheduledHandler, dashboardHandler,
		importHandler,
		authSvc, cfg.CORSOrigins, logger,
	)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
	logger.Info("server stopped")
}
