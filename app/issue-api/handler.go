package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/segmentio/ksuid"
)

type handler struct {
	store   *redisStore
	eventID string
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
	default:
		// Lua returned a *different* ID than the one we proposed,
		// which means this Idempotency-Key was previously seen.
		resp.IssueID = result
		resp.Status = "idempotent_replay"
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (h *handler) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("version=" + version + " commit=" + commit + "\n"))
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
