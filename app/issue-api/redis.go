package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// issueScript performs the entire issue operation atomically:
//   1. Idempotency check — if this Idempotency-Key was seen before, return the same ID.
//   2. Stock check + DECR — if stock > 0, decrement and proceed; else return SOLD_OUT.
//   3. Save idempotency mapping with TTL so retries within the window get the same ID.
//
// Because Redis runs scripts on a single thread, these three steps appear as
// one indivisible operation — no other client can read or modify stock between
// them. This is the foundation of the "oversell 0건" guarantee.
//
//   KEYS[1] = stock key,            e.g. "stock:event:summer-shoes-2026"
//   KEYS[2] = idempotency key,      e.g. "idem:summer-shoes-2026:<key>"
//   ARGV[1] = idempotency TTL in seconds (string)
//   ARGV[2] = candidate issue ID (returned to caller on success)
//
// Returns: the issue ID (new or replayed) on success, or "SOLD_OUT".
const issueScript = `
local prev = redis.call('GET', KEYS[2])
if prev then return prev end

local stock = redis.call('GET', KEYS[1])
if not stock or tonumber(stock) <= 0 then
  return 'SOLD_OUT'
end

redis.call('DECR', KEYS[1])
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[1])
return ARGV[2]
`

type redisStore struct {
	client  *redis.Client
	script  *redis.Script
	idemTTL time.Duration
}

func newRedisStore(addr string) (*redisStore, error) {
	c := redis.NewClient(&redis.Options{Addr: addr})
	if err := c.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &redisStore{
		client:  c,
		script:  redis.NewScript(issueScript),
		idemTTL: 24 * time.Hour,
	}, nil
}

func (s *redisStore) Close() error { return s.client.Close() }

// tryIssue runs the Lua script. Returns the resulting issue ID (or "SOLD_OUT").
func (s *redisStore) tryIssue(ctx context.Context, eventID, idemKey, newID string) (string, error) {
	keys := []string{
		"stock:event:" + eventID,
		"idem:" + eventID + ":" + idemKey,
	}
	args := []any{int(s.idemTTL.Seconds()), newID}
	return s.script.Run(ctx, s.client, keys, args...).Text()
}

// stock returns the current remaining count.
func (s *redisStore) stock(ctx context.Context, eventID string) (int64, error) {
	return s.client.Get(ctx, "stock:event:"+eventID).Int64()
}

// SetStock initializes (or resets) the stock counter — used by admin/tests.
func (s *redisStore) SetStock(ctx context.Context, eventID string, n int64) error {
	return s.client.Set(ctx, "stock:event:"+eventID, n, 0).Err()
}
