// Package main implements the bot detector (Ch10 Phase F-2).
//
// It consumes the bot-signals Kafka topic that issue-api writes for every
// /issue request, runs a heuristic over a sliding window (ADR-019), and adds
// flagged IPs to an AWS WAF v2 IPSet (ADR-020). The WAF then blocks those IPs
// on their next request - closing the detect -> block feedback loop.
//
// IRSA gives the Pod wafv2:GetIPSet / wafv2:UpdateIPSet on one IPSet only.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	brokers := mustenv("KAFKA_BROKERS")
	topic := getenv("KAFKA_BOT_TOPIC", "bot.signals")
	group := getenv("CONSUMER_GROUP", "bot-detector")

	ipSetName := mustenv("WAF_IPSET_NAME")
	ipSetID := mustenv("WAF_IPSET_ID")

	window := time.Duration(atoiDefault("WINDOW_SECONDS", 10)) * time.Second
	burstLimit := atoiDefault("BURST_LIMIT", 30)
	soldOutLimit := atoiDefault("SOLDOUT_LIMIT", 15)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	waf, err := newWAFBlocklist(ctx, ipSetName, ipSetID)
	if err != nil {
		log.Fatalf("waf init: %v", err)
	}

	detector := newHeuristicDetector(window, burstLimit, soldOutLimit)

	log.Printf("bot-detector %s (%s) brokers=%s topic=%s window=%s burst=%d soldout=%d",
		version, commit, brokers, topic, window, burstLimit, soldOutLimit)

	c := newConsumer(brokers, topic, group, detector, waf)
	defer c.Close()

	if err := c.run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("consumer: %v", err)
	}
	log.Println("shutting down...")
	time.Sleep(200 * time.Millisecond)
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

func atoiDefault(k string, fallback int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
