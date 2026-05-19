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
	"strconv"
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
	secretARN := os.Getenv("SECRET_DB_PASSWORD_ARN") // optional; enables /aws-check
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")       // empty -> producer disabled
	kafkaTopic := getenv("KAFKA_TOPIC", "issuance.events")

	store, err := newRedisStore(redisAddr)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer store.Close()

	producer, err := newKafkaProducer(kafkaBrokers, kafkaTopic)
	if err != nil {
		log.Fatalf("kafka producer: %v", err)
	}
	defer producer.Close()
	if producer != nil {
		log.Printf("kafka producer enabled brokers=%s topic=%s", kafkaBrokers, kafkaTopic)
	}

	h := &handler{store: store, producer: producer, eventID: eventID}

	// Phase E-2: virtual waiting queue (ADR-017). Activated by env so unit
	// tests and bare local runs skip the queue and the worker.
	queueEnabled := os.Getenv("QUEUE_ENABLED") == "true"
	admissionRate, _ := strconv.Atoi(getenv("QUEUE_ADMISSION_RATE", "5"))
	if queueEnabled {
		h.queue = newQueueStore(store.client, eventID)
		log.Printf("queue enabled rate=%d/s event=%s", admissionRate, eventID)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /issue", h.issue)
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /version", h.handleVersion)
	mux.HandleFunc("GET /stock", h.stock)
	if queueEnabled {
		mux.HandleFunc("POST /queue/join", h.queueJoin)
		mux.HandleFunc("GET /queue/status", h.queueStatus)
	}

	if secretARN != "" {
		checker, err := newAWSChecker(context.Background(), secretARN)
		if err != nil {
			log.Fatalf("aws check init: %v", err)
		}
		mux.HandleFunc("GET /aws-check", checker.handle)
		log.Printf("aws-check enabled for secret %s", secretARN)
	}

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

	// Phase E-2: every replica runs this loop. The Lua script enforces the
	// global N/sec cap, so calling it from 1 or 10 replicas yields the same
	// admission rate. No leader election needed.
	if queueEnabled && h.queue != nil {
		go runAdmitLoop(ctx, h.queue, admissionRate)
	}

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
