package main

import (
	"context"
	"log"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := postgres.New(ctx, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	if err := db.RunMigrations(ctx); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	accountRepo := postgres.NewAccountRepo(db)
	txnRepo := postgres.NewTransactionRepo(db)

	locker := engine.NewOptimisticLock(accountRepo)
	idempotency := engine.NewIdempotency(txnRepo)
	eng := engine.New(accountRepo, txnRepo, locker, idempotency)

	handler := api.NewHandler(eng, accountRepo, txnRepo)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	wrapped := api.Logging(mux)

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

		log.Println("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on :%s", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}

	db.Close()
}
