// Package main implements the limited-edition issuance API.
//
// Endpoints:
//
//	POST /issue     — atomically decrement stock and issue an ID (Idempotency-Key header required)
//	GET  /healthz   — liveness/readiness probe
//	GET  /version   — build version + git commit
//	GET  /stock     — current remaining stock (observability)
//
// The hot path is a single Redis EVAL of issueScript (see redis.go). See:
//   - ADR-008: why Redis Lua for atomic counter
//   - ADR-009: why net/http stdlib (no framework)
//   - ADR-010: why Redis with TTL for idempotency
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Overridden at build time via -ldflags "-X main.version=... -X main.commit=..."
var (
	version = "dev"
	commit  = "none"
)

func main() {
	addr := getenv("ADDR", ":8080")
	redisAddr := getenv("REDIS_ADDR", "localhost:6379")
	eventID := getenv("EVENT_ID", "summer-shoes-2026")

	store, err := newRedisStore(redisAddr)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer store.Close()

	h := &handler{store: store, eventID: eventID}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /issue", h.issue)
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /version", h.handleVersion)
	mux.HandleFunc("GET /stock", h.stock)

	server := &http.Server{
		Addr:              addr,
		Handler:           withLogging(withRecover(mux)),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	// Graceful shutdown — SIGTERM from K8s gives us 30s to drain.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("issue-api %s (commit %s) event=%s redis=%s listening on %s",
			version, commit, eventID, redisAddr, addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
