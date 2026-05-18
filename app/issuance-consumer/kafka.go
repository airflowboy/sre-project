package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// issuanceEvent mirrors the type in app/issue-api/kafka.go. We deliberately
// re-declare it here rather than share a module so the two binaries can evolve
// independently - the JSON wire format is the contract.
type issuanceEvent struct {
	IssueID        string    `json:"issue_id"`
	EventID        string    `json:"event_id"`
	UserID         string    `json:"user_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	IssuedAt       time.Time `json:"issued_at"`
}

type consumer struct {
	reader *kafka.Reader
	store  *store
}

func newConsumer(brokers, topic, group string, s *store) *consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{brokers},
		Topic:          topic,
		GroupID:        group,
		MinBytes:       1,
		MaxBytes:       1e6,
		CommitInterval: 0, // commit synchronously after DB write
	})
	return &consumer{reader: r, store: s}
}

func (c *consumer) Close() error { return c.reader.Close() }

// run is the main loop. FetchMessage returns one message; we INSERT it, then
// commit only after a successful write. A crash between insert and commit
// re-delivers the message, but ON CONFLICT DO NOTHING absorbs the duplicate.
func (c *consumer) run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		var evt issuanceEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			log.Printf("malformed event at offset %d: %v - skipping", msg.Offset, err)
			_ = c.reader.CommitMessages(ctx, msg) // poison-pill: don't block the partition
			continue
		}

		if err := c.store.insertIssued(ctx, evt); err != nil {
			log.Printf("insert failed for %s: %v - will retry", evt.IssueID, err)
			// Do not commit; loop retries the same offset after a backoff.
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit failed at offset %d: %v", msg.Offset, err)
		}
	}
}
