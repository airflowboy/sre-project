package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/segmentio/ksuid"
)

type handler struct {
	store    *redisStore
	producer *kafkaProducer // may be nil when KAFKA_BROKERS is unset (tests/local)
	queue    *queueStore    // may be nil when QUEUE_ENABLED is unset (tests/local)
	eventID  string
}

type issueRequest struct {
	UserID string `json:"user_id"`
}

type issueResponse struct {
	IssueID string `json:"issue_id,omitempty"`
	Status  string `json:"status"` // issued | sold_out | idempotent_replay
	EventID string `json:"event_id"`
}

// POST /issue
//
// Headers:
//
//	Idempotency-Key: <UUID/KSUID, required>
//
// Body:
//
//	{"user_id": "..."}
//
// Responses:
//
//	201 Created       — issued (status="issued", issue_id=...)
//	200 OK            — idempotent replay (status="idempotent_replay", issue_id=...)
//	409 Conflict      — sold out (status="sold_out")
//	400 Bad Request   — missing Idempotency-Key or invalid JSON
func (h *handler) issue(w http.ResponseWriter, r *http.Request) {
	idemKey := r.Header.Get("Idempotency-Key")
	if idemKey == "" {
		http.Error(w, "Idempotency-Key header required", http.StatusBadRequest)
		return
	}

	// Phase E-2: virtual waiting queue (ADR-017). The queue is opt-in -
	// when the queue worker isn't running (h.queue == nil) we keep the
	// Phase C behavior so unit tests on miniredis don't need a queue.
	if h.queue != nil {
		token := r.Header.Get("X-Queue-Token")
		ok, err := h.queue.hasAdmission(r.Context(), token)
		if err != nil {
			log.Printf("queue check error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !ok {
			// A token-less hammering pattern is itself a bot signal.
			h.producer.produceBotSignal(botSignal{
				IP: clientIP(r), UserAgent: r.UserAgent(),
				Result: "rejected", Timestamp: time.Now().UTC(),
			})
			http.Error(w, "queue token required - call POST /queue/join first", http.StatusForbidden)
			return
		}
	}

	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Generate a candidate ID. If Redis Lua decides this is a replay, it
	// returns the *previous* ID instead and we discard this one.
	newID := ksuid.New().String()

	result, err := h.store.tryIssue(r.Context(), h.eventID, idemKey, newID)
	if err != nil {
		log.Printf("redis error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := issueResponse{EventID: h.eventID}
	switch result {
	case "SOLD_OUT":
		resp.Status = "sold_out"
		w.WriteHeader(http.StatusConflict)
	case newID:
		resp.IssueID = result
		resp.Status = "issued"
		w.WriteHeader(http.StatusCreated)
		// Fire-and-forget the persistence event to Kafka. Failure here does
		// not affect the user-visible result - Redis is the source of truth
		// for stock, and the consumer is replayable.
		h.producer.produceAsync(issuanceEvent{
			IssueID:        result,
			EventID:        h.eventID,
			UserID:         req.UserID,
			IdempotencyKey: idemKey,
			IssuedAt:       time.Now().UTC(),
		})
	default:
		// Lua returned a *different* ID than the one we proposed,
		// which means this Idempotency-Key was previously seen.
		resp.IssueID = result
		resp.Status = "idempotent_replay"
		w.WriteHeader(http.StatusOK)
	}

	// Phase F-2: every /issue outcome is a bot-detection signal (ADR-019).
	// Keyed by IP downstream; hammering shows up as a burst of these.
	h.producer.produceBotSignal(botSignal{
		IP: clientIP(r), UserAgent: r.UserAgent(),
		Result: resp.Status, Timestamp: time.Now().UTC(),
	})

	_ = json.NewEncoder(w).Encode(resp)
}

// clientIP extracts the real caller IP. Behind an ALB the original client is
// the first entry of X-Forwarded-For ("client, proxy1, proxy2"); RemoteAddr
// would just be the load balancer.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (h *handler) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("version=" + version + " commit=" + commit + "\n"))
}

// POST /queue/join - take a number and step into the waiting line (ADR-017).
//
// Responses:
//
//	201 Created   - {"token":"<KSUID>", "position":<0-based rank>}
//	503 Service Unavailable - queue is disabled (QUEUE_ENABLED unset)
func (h *handler) queueJoin(w http.ResponseWriter, r *http.Request) {
	if h.queue == nil {
		http.Error(w, "queue disabled", http.StatusServiceUnavailable)
		return
	}
	token, rank, err := h.queue.join(r.Context())
	if err != nil {
		log.Printf("queue join error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":    token,
		"position": rank,
		"event_id": h.eventID,
	})
}

// GET /queue/status?token=<KSUID>
//
//	200 OK        - {"status":"waiting","position":N} | {"status":"admitted"}
//	404 Not Found - token unknown (never joined or expired)
//	503           - queue disabled
func (h *handler) queueStatus(w http.ResponseWriter, r *http.Request) {
	if h.queue == nil {
		http.Error(w, "queue disabled", http.StatusServiceUnavailable)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token query param required", http.StatusBadRequest)
		return
	}
	st, rank, err := h.queue.status(r.Context(), token)
	if err != nil {
		log.Printf("queue status error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if st == "unknown" {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "unknown"})
		return
	}
	resp := map[string]any{"status": st}
	if st == "waiting" {
		resp["position"] = rank
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *handler) stock(w http.ResponseWriter, r *http.Request) {
	n, err := h.store.stock(r.Context(), h.eventID)
	if err != nil {
		http.Error(w, "stock unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"event_id":  h.eventID,
		"remaining": n,
	})
}
