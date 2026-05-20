package main

import (
	"sync"
	"time"
)

// requestSignal mirrors the JSON that issue-api writes to the bot-signals
// topic. Re-declared here (not shared) so the two binaries evolve via the
// wire format contract only - same pattern as issuance-consumer (ADR-016).
type requestSignal struct {
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Result    string    `json:"result"` // issued | sold_out | idempotent_replay | rejected
	Timestamp time.Time `json:"timestamp"`
}

// BotDetector decides which IPs look abusive. Today the only implementation
// is HeuristicDetector; an IsolationForestDetector could be slotted in later
// behind this same interface without touching the Kafka loop or WAF updater
// (ADR-019).
type BotDetector interface {
	// Observe records one signal. Returns true the moment this IP crosses
	// into "bot" territory (so the caller can act once, not every signal).
	Observe(sig requestSignal) (flagged bool)
}

// HeuristicDetector flags an IP by two rules over a sliding time window:
//
//	rule 1 (burst)        - more than burstLimit requests within windowSize
//	rule 2 (sold-out hammer) - more than soldOutLimit sold_out/rejected results
//	                           within windowSize (kept hammering after no stock)
//
// State is in-memory, so the detector must run as a single replica (ADR-019
// lists a Redis shared window as the multi-replica upgrade).
type HeuristicDetector struct {
	windowSize   time.Duration
	burstLimit   int
	soldOutLimit int

	mu      sync.Mutex
	hits    map[string][]time.Time // IP -> request timestamps in window
	soldOut map[string][]time.Time // IP -> sold_out/rejected timestamps in window
	flagged map[string]bool        // IP -> already reported
}

func newHeuristicDetector(window time.Duration, burstLimit, soldOutLimit int) *HeuristicDetector {
	return &HeuristicDetector{
		windowSize:   window,
		burstLimit:   burstLimit,
		soldOutLimit: soldOutLimit,
		hits:         map[string][]time.Time{},
		soldOut:      map[string][]time.Time{},
		flagged:      map[string]bool{},
	}
}

func (d *HeuristicDetector) Observe(sig requestSignal) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-d.windowSize)

	d.hits[sig.IP] = append(prune(d.hits[sig.IP], cutoff), now)
	if sig.Result == "sold_out" || sig.Result == "rejected" {
		d.soldOut[sig.IP] = append(prune(d.soldOut[sig.IP], cutoff), now)
	}

	if d.flagged[sig.IP] {
		return false // already reported - don't double-fire
	}

	burst := len(d.hits[sig.IP]) > d.burstLimit
	hammer := len(d.soldOut[sig.IP]) > d.soldOutLimit
	if burst || hammer {
		d.flagged[sig.IP] = true
		return true
	}
	return false
}

// prune drops timestamps older than cutoff from a sorted-ascending slice.
func prune(ts []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(ts) && ts[i].Before(cutoff) {
		i++
	}
	return ts[i:]
}
