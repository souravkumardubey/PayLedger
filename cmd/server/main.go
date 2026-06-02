package main

import (
	"log"
	"net/http"

	"github.com/souravkumardubey/PayLedger/internal/api"
	"github.com/souravkumardubey/PayLedger/internal/config"
)

func main() {
	cfg := config.Load()

	handler := api.NewHandler()
	middleware := api.NewMiddleware()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	wrapped := middleware.Logging(middleware.Idempotency(mux))

	addr := ":" + cfg.Server.Port
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, wrapped); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
