package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/segmentio/kafka-go"
)

// consumer reads the bot-signals topic, feeds each signal to the detector,
// and asks the WAF blocklist to block any newly flagged IP.
type consumer struct {
	reader   *kafka.Reader
	detector BotDetector
	waf      *wafBlocklist
}

func newConsumer(brokers, topic, group string, d BotDetector, waf *wafBlocklist) *consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{brokers},
		Topic:          topic,
		GroupID:        group,
		MinBytes:       1,
		MaxBytes:       1e6,
		CommitInterval: 0, // commit synchronously after processing
	})
	return &consumer{reader: r, detector: d, waf: waf}
}

func (c *consumer) Close() error { return c.reader.Close() }

// run loops until ctx is cancelled. A signal that flags an IP triggers a WAF
// IPSet update; failure there is logged but does not stop the loop (the IP
// will be re-flagged on its next request, since the detector only fires once
// per IP - see note below).
func (c *consumer) run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		var sig requestSignal
		if err := json.Unmarshal(msg.Value, &sig); err != nil {
			log.Printf("malformed signal at offset %d: %v - skipping", msg.Offset, err)
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		if c.detector.Observe(sig) {
			log.Printf("bot detected: ip=%s ua=%q result=%s", sig.IP, sig.UserAgent, sig.Result)
			if err := c.waf.block(ctx, sig.IP); err != nil {
				log.Printf("WAF block failed for %s: %v", sig.IP, err)
			}
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit failed at offset %d: %v", msg.Offset, err)
		}
	}
}
