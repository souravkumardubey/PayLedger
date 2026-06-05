package api

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

type wrappedHandler struct {
	inner  http.Handler
	logger *slog.Logger
}

func WrapHandler(inner http.Handler, logger *slog.Logger) http.Handler {
	return &wrappedHandler{inner: inner, logger: logger}
}

func (h *wrappedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
	h.inner.ServeHTTP(rw, r)
	h.logger.Info("request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", rw.status,
		"duration", time.Since(start),
	)
}
