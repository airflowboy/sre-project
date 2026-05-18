package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// issuanceEvent is what we put on the topic for every successful issue.
// The consumer (app/issuance-consumer) uses issue_id as the primary key, so
// re-delivery of the same event is a no-op (ON CONFLICT DO NOTHING).
type issuanceEvent struct {
	IssueID        string    `json:"issue_id"`
	EventID        string    `json:"event_id"`
	UserID         string    `json:"user_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	IssuedAt       time.Time `json:"issued_at"`
}

type kafkaProducer struct {
	writer *kafka.Writer
	topic  string
}

// newKafkaProducer connects to the brokers (comma-separated). brokers="" returns nil
// without an error so local miniredis tests keep working with no Kafka around.
func newKafkaProducer(brokers, topic string) (*kafkaProducer, error) {
	if brokers == "" {
		return nil, nil
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers),
		Topic:        topic,
		Balancer:     &kafka.Hash{}, // hash by key -> same issue_id -> same partition
		RequiredAcks: kafka.RequireOne,
		Async:        false, // we use a goroutine in handler so blocking here is fine
		BatchTimeout: 50 * time.Millisecond,
	}
	return &kafkaProducer{writer: w, topic: topic}, nil
}

func (p *kafkaProducer) Close() error {
	if p == nil {
		return nil
	}
	return p.writer.Close()
}

// produceAsync fires the event on a goroutine so the HTTP response is never blocked.
// Failure is logged but does not surface to the caller - the issuance itself has
// already been recorded in Redis (the source of truth for stock), and the consumer
// can be replayed from Kafka after recovery.
func (p *kafkaProducer) produceAsync(evt issuanceEvent) {
	if p == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		body, _ := json.Marshal(evt)
		err := p.writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(evt.IssueID),
			Value: body,
		})
		if err != nil {
			log.Printf("kafka produce failed for %s: %v", evt.IssueID, err)
		}
	}()
}
