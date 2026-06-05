package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/souravkumardubey/PayLedger/internal/api"
	"github.com/souravkumardubey/PayLedger/internal/config"
	"github.com/souravkumardubey/PayLedger/internal/engine"
	"github.com/souravkumardubey/PayLedger/internal/repository/postgres"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.New(ctx, cfg.Database.DSN)
	if err != nil {
		logger.Error("database", "error", err)
		os.Exit(1)
	}

	if err := db.RunMigrations(ctx); err != nil {
		logger.Error("migrations", "error", err)
		os.Exit(1)
	}

	accountRepo := postgres.NewAccountRepo(db)
	txnRepo := postgres.NewTransactionRepo(db)
	walStore := postgres.NewWALStore(db)

	if err := engine.RecoverWAL(ctx, walStore, txnRepo); err != nil {
		logger.Error("WAL recovery", "error", err)
		os.Exit(1)
	}

	locker := engine.NewPessimisticLock(db.Pool)
	idempotency := engine.NewIdempotency(txnRepo)
	eng := engine.New(accountRepo, txnRepo, locker, idempotency, walStore)

	handler := api.NewHandler(eng, accountRepo, txnRepo)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	rl := api.NewRateLimiter(10, 20)
	defer rl.Stop()

	wrapped := api.RateLimit(rl, mux, logger)
	wrapped = api.WrapHandler(wrapped, logger)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      wrapped,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("listening", "port", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server", "error", err)
		os.Exit(1)
	}

	db.Close()
}
