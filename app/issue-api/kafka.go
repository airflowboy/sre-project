package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

// issuanceEvent is what we put on the issuance topic for every successful issue.
// The consumer (app/issuance-consumer) uses issue_id as the primary key, so
// re-delivery of the same event is a no-op (ON CONFLICT DO NOTHING).
type issuanceEvent struct {
	IssueID        string    `json:"issue_id"`
	EventID        string    `json:"event_id"`
	UserID         string    `json:"user_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	IssuedAt       time.Time `json:"issued_at"`
}

// botSignal is emitted for EVERY /issue request (success or not) on the
// bot-signals topic. The bot-detector (app/bot-detector) consumes these and
// looks for abusive behavior patterns (ADR-019).
type botSignal struct {
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Result    string    `json:"result"` // issued | sold_out | idempotent_replay | rejected
	Timestamp time.Time `json:"timestamp"`
}

// kafkaProducer writes to two topics from one connection. The Writer has no
// fixed Topic - each Message carries its own, so issuance events and bot
// signals share the same broker connection.
type kafkaProducer struct {
	writer        *kafka.Writer
	issuanceTopic string
	botTopic      string
}

// newKafkaProducer connects to the brokers (comma-separated). brokers="" returns nil
// without an error so local miniredis tests keep working with no Kafka around.
func newKafkaProducer(brokers, issuanceTopic, botTopic string) (*kafkaProducer, error) {
	if brokers == "" {
		return nil, nil
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers),
		Balancer:     &kafka.Hash{}, // hash by key -> same key -> same partition
		RequiredAcks: kafka.RequireOne,
		Async:        false, // we use a goroutine in handler so blocking here is fine
		BatchTimeout: 50 * time.Millisecond,
	}
	return &kafkaProducer{writer: w, issuanceTopic: issuanceTopic, botTopic: botTopic}, nil
}

func (p *kafkaProducer) Close() error {
	if p == nil {
		return nil
	}
	return p.writer.Close()
}

// produceAsync fires the issuance event on a goroutine so the HTTP response is
// never blocked. Failure is logged but does not surface to the caller - the
// issuance itself has already been recorded in Redis (the source of truth for
// stock), and the consumer can be replayed from Kafka after recovery.
func (p *kafkaProducer) produceAsync(evt issuanceEvent) {
	if p == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		body, _ := json.Marshal(evt)
		err := p.writer.WriteMessages(ctx, kafka.Message{
			Topic: p.issuanceTopic,
			Key:   []byte(evt.IssueID),
			Value: body,
		})
		if err != nil {
			log.Printf("kafka issuance produce failed for %s: %v", evt.IssueID, err)
		}
	}()
}

// produceBotSignal fires a bot-detection signal on a goroutine. Keyed by IP so
// the bot-detector sees one IP's traffic on one partition (ordered per IP).
func (p *kafkaProducer) produceBotSignal(sig botSignal) {
	if p == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		body, _ := json.Marshal(sig)
		err := p.writer.WriteMessages(ctx, kafka.Message{
			Topic: p.botTopic,
			Key:   []byte(sig.IP),
			Value: body,
		})
		if err != nil {
			log.Printf("kafka bot-signal produce failed for %s: %v", sig.IP, err)
		}
	}()
}
