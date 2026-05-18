// Package main implements the issuance event consumer (Ch10 Phase E-1).
//
// Pulls events off the Kafka topic that issue-api writes to and persists them
// into the RDS Postgres `issued` table. Idempotent INSERT on issue_id PK lets
// the consumer be replayed safely after a crash.
//
// IRSA: at startup we read SECRET_DB_URL_ARN from Secrets Manager via the
// Pod's projected ServiceAccount token. Never receives a static DB password.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	brokers := mustenv("KAFKA_BROKERS")
	topic := getenv("KAFKA_TOPIC", "issuance.events")
	group := getenv("CONSUMER_GROUP", "issuance-consumer")
	secretARN := mustenv("SECRET_DB_URL_ARN")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// IRSA -> Secrets Manager -> DSN. Done once at startup; on rotation the
	// Pod is rolled. Pgx pool then keeps connections warm against RDS.
	dsn, err := fetchDSN(ctx, secretARN)
	if err != nil {
		log.Fatalf("fetch dsn: %v", err)
	}

	store, err := newStore(ctx, dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer store.Close()

	if err := store.ensureSchema(ctx); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}
	log.Printf("issuance-consumer %s (%s) brokers=%s topic=%s group=%s",
		version, commit, brokers, topic, group)

	c := newConsumer(brokers, topic, group, store)
	defer c.Close()

	// Run until SIGTERM. ReadMessage blocks; the context cancel below
	// breaks the loop and lets us close cleanly.
	if err := c.run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("consumer: %v", err)
	}
	log.Println("shutting down...")
	time.Sleep(200 * time.Millisecond) // brief flush window
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func mustenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s required", k)
	}
	return v
}
