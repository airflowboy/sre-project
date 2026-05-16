package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/ksuid"
)

// setup spins up an in-process Redis (miniredis) and returns a handler
// configured against it with the given initial stock.
func setup(t *testing.T, initialStock int64) *handler {
	t.Helper()
	mr := miniredis.RunT(t) // automatically Close()d when test ends
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := &redisStore{
		client:  client,
		script:  redis.NewScript(issueScript),
		idemTTL: 24 * time.Hour,
	}
	if err := store.SetStock(context.Background(), "test-event", initialStock); err != nil {
		t.Fatalf("set stock: %v", err)
	}
	return &handler{store: store, eventID: "test-event"}
}

func doIssue(t *testing.T, h *handler, idemKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/issue", strings.NewReader(`{"user_id":"u1"}`))
	req.Header.Set("Idempotency-Key", idemKey)
	rr := httptest.NewRecorder()
	h.issue(rr, req)
	return rr
}

func TestIssue_Success(t *testing.T) {
	h := setup(t, 5)
	rr := doIssue(t, h, "k1")
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var resp issueResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "issued" || resp.IssueID == "" {
		t.Fatalf("bad response: %+v", resp)
	}
}

func TestIssue_Idempotent_ReturnsSameID(t *testing.T) {
	h := setup(t, 5)
	rr1 := doIssue(t, h, "same-key")
	rr2 := doIssue(t, h, "same-key")

	var r1, r2 issueResponse
	json.NewDecoder(rr1.Body).Decode(&r1)
	json.NewDecoder(rr2.Body).Decode(&r2)

	if rr1.Code != http.StatusCreated || rr2.Code != http.StatusOK {
		t.Fatalf("expected 201 then 200, got %d then %d", rr1.Code, rr2.Code)
	}
	if r1.IssueID != r2.IssueID {
		t.Fatalf("idempotent replay returned different IDs: %q vs %q", r1.IssueID, r2.IssueID)
	}
	if r2.Status != "idempotent_replay" {
		t.Fatalf("second response status = %q, want idempotent_replay", r2.Status)
	}

	// Stock should have decremented only ONCE (5 -> 4), not twice.
	n, _ := h.store.stock(context.Background(), "test-event")
	if n != 4 {
		t.Fatalf("stock = %d, want 4 (idempotent retry must not double-decrement)", n)
	}
}

func TestIssue_SoldOut(t *testing.T) {
	h := setup(t, 0)
	rr := doIssue(t, h, "k1")
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
	var resp issueResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Status != "sold_out" {
		t.Fatalf("status = %q, want sold_out", resp.Status)
	}
}

func TestIssue_MissingIdempotencyKey(t *testing.T) {
	h := setup(t, 5)
	req := httptest.NewRequest("POST", "/issue", strings.NewReader(`{"user_id":"u1"}`))
	rr := httptest.NewRecorder()
	h.issue(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestIssue_InvalidJSON(t *testing.T) {
	h := setup(t, 5)
	req := httptest.NewRequest("POST", "/issue", strings.NewReader(`{not json`))
	req.Header.Set("Idempotency-Key", "k1")
	rr := httptest.NewRecorder()
	h.issue(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// 🎯 The oversell test — the single most important guarantee of the system.
//
// N=100 goroutines try to claim from stock=5. We MUST end with exactly:
//   - 5 successful issues (HTTP 201)
//   - 95 sold_out responses (HTTP 409)
//   - 0 other status codes
//   - stock = 0 in Redis
//
// If any of these are off, the atomic counter is broken and we'd have oversell
// in production. This test guards ADR-008 by execution.
func TestIssue_NoOversell_Race(t *testing.T) {
	const (
		N            = 100
		initialStock = 5
	)
	h := setup(t, initialStock)

	var (
		wg      sync.WaitGroup
		issued  int64
		soldOut int64
		other   int64
	)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := doIssue(t, h, ksuid.New().String()) // unique per request
			switch rr.Code {
			case http.StatusCreated:
				atomic.AddInt64(&issued, 1)
			case http.StatusConflict:
				atomic.AddInt64(&soldOut, 1)
			default:
				atomic.AddInt64(&other, 1)
			}
		}()
	}
	wg.Wait()

	if issued != initialStock {
		t.Fatalf("issued = %d, want %d (sold_out=%d other=%d) — OVERSELL OR UNDERSELL!",
			issued, initialStock, soldOut, other)
	}
	if soldOut != N-initialStock {
		t.Fatalf("sold_out = %d, want %d", soldOut, N-initialStock)
	}
	if other != 0 {
		t.Fatalf("unexpected non-201/409 responses: %d", other)
	}

	finalStock, _ := h.store.stock(context.Background(), "test-event")
	if finalStock != 0 {
		t.Fatalf("final stock = %d, want 0", finalStock)
	}
}

func TestHealthz(t *testing.T) {
	h := setup(t, 0)
	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	h.healthz(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestStock(t *testing.T) {
	h := setup(t, 42)
	req := httptest.NewRequest("GET", "/stock", nil)
	rr := httptest.NewRecorder()
	h.stock(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	// JSON unmarshal makes numbers float64
	if int(resp["remaining"].(float64)) != 42 {
		t.Fatalf("remaining = %v, want 42", resp["remaining"])
	}
}
