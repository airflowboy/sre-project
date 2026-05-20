package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

// wafBlocklist owns one AWS WAF v2 IPSet and appends bot IPs to it (ADR-020).
//
// UpdateIPSet overwrites the whole address list and needs the current
// LockToken, so every update is: GetIPSet -> append -> UpdateIPSet. We keep
// a local mirror too, mostly for the smoke-test log line.
type wafBlocklist struct {
	client *wafv2.Client
	name   string
	id     string
	scope  types.Scope

	mu      sync.Mutex
	blocked map[string]bool
}

func newWAFBlocklist(ctx context.Context, name, id string) (*wafBlocklist, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &wafBlocklist{
		client:  wafv2.NewFromConfig(cfg),
		name:    name,
		id:      id,
		scope:   types.ScopeRegional,
		blocked: map[string]bool{},
	}, nil
}

// block adds ip (as a /32 CIDR) to the IPSet. Idempotent - a repeat IP that
// is already present is skipped without an API call.
func (b *wafBlocklist) block(ctx context.Context, ip string) error {
	cidr := ip + "/32"

	b.mu.Lock()
	if b.blocked[cidr] {
		b.mu.Unlock()
		return nil
	}
	b.mu.Unlock()

	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 1. GetIPSet - current addresses + LockToken (optimistic locking).
	got, err := b.client.GetIPSet(cctx, &wafv2.GetIPSetInput{
		Name:  aws.String(b.name),
		Id:    aws.String(b.id),
		Scope: b.scope,
	})
	if err != nil {
		return fmt.Errorf("get ipset: %w", err)
	}

	addrs := got.IPSet.Addresses
	for _, a := range addrs {
		if a == cidr {
			b.mu.Lock()
			b.blocked[cidr] = true
			b.mu.Unlock()
			return nil // already in the set (added by a previous run)
		}
	}
	addrs = append(addrs, cidr)

	// 2. UpdateIPSet - overwrite with the LockToken from step 1.
	_, err = b.client.UpdateIPSet(cctx, &wafv2.UpdateIPSetInput{
		Name:      aws.String(b.name),
		Id:        aws.String(b.id),
		Scope:     b.scope,
		Addresses: addrs,
		LockToken: got.LockToken,
	})
	if err != nil {
		return fmt.Errorf("update ipset: %w", err)
	}

	b.mu.Lock()
	b.blocked[cidr] = true
	n := len(b.blocked)
	b.mu.Unlock()
	log.Printf("blocked %s via WAF IPSet (%d total)", cidr, n)
	return nil
}
