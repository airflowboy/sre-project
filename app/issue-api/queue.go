package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/ksuid"
)

// admitScript implements the global N-per-second admission cap (ADR-017).
//
// All issue-api replicas may call this every second; Redis single-thread
// semantics guarantees the rate window key is read+incremented atomically,
// so the total tokens admitted across the fleet never exceeds the cap.
//
//	KEYS[1] = waiting zset            "queue:waiting:<eventID>"
//	KEYS[2] = admitted set            "queue:admitted:<eventID>"
//	KEYS[3] = rate window counter     "queue:rate:<eventID>:<unix_sec>"
//	ARGV[1] = max admit per second
//
// Returns: list of tokens just admitted (possibly empty).
const admitScript = `
local already = redis.call('GET', KEYS[3])
local n_so_far = already and tonumber(already) or 0
local quota = tonumber(ARGV[1]) - n_so_far
if quota <= 0 then return {} end

local popped = redis.call('ZPOPMIN', KEYS[1], quota)
local tokens = {}
for i = 1, #popped, 2 do table.insert(tokens, popped[i]) end

if #tokens > 0 then
  redis.call('SADD', KEYS[2], unpack(tokens))
  redis.call('INCRBY', KEYS[3], #tokens)
  redis.call('EXPIRE', KEYS[3], 2)
end
return tokens
`

type queueStore struct {
	client      *redis.Client
	admit       *redis.Script
	eventID     string
}

func newQueueStore(client *redis.Client, eventID string) *queueStore {
	return &queueStore{
		client:  client,
		admit:   redis.NewScript(admitScript),
		eventID: eventID,
	}
}

func (q *queueStore) waitingKey() string  { return "queue:waiting:" + q.eventID }
func (q *queueStore) admittedKey() string { return "queue:admitted:" + q.eventID }
func (q *queueStore) rateKey() string {
	return fmt.Sprintf("queue:rate:%s:%d", q.eventID, time.Now().Unix())
}

// join adds the caller to the back of the waiting line and returns the
// fresh token plus its current rank (0-based - rank 0 means "next up").
func (q *queueStore) join(ctx context.Context) (token string, rank int64, err error) {
	token = ksuid.New().String()
	score := float64(time.Now().UnixMilli())
	if err := q.client.ZAdd(ctx, q.waitingKey(), redis.Z{Score: score, Member: token}).Err(); err != nil {
		return "", 0, fmt.Errorf("zadd: %w", err)
	}
	r, err := q.client.ZRank(ctx, q.waitingKey(), token).Result()
	if err != nil {
		return "", 0, fmt.Errorf("zrank: %w", err)
	}
	return token, r, nil
}

// status returns one of: "waiting" (with rank), "admitted", or "unknown".
func (q *queueStore) status(ctx context.Context, token string) (string, int64, error) {
	r, err := q.client.ZRank(ctx, q.waitingKey(), token).Result()
	if err == nil {
		return "waiting", r, nil
	}
	if err != redis.Nil {
		return "", 0, err
	}
	ok, err := q.client.SIsMember(ctx, q.admittedKey(), token).Result()
	if err != nil {
		return "", 0, err
	}
	if ok {
		return "admitted", 0, nil
	}
	return "unknown", 0, nil
}

// hasAdmission gates POST /issue. Token must be in the admitted set.
func (q *queueStore) hasAdmission(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	return q.client.SIsMember(ctx, q.admittedKey(), token).Result()
}

// admitOnce runs one tick of the admission loop. Returns how many tokens
// were admitted this call (for log/metric visibility).
func (q *queueStore) admitOnce(ctx context.Context, maxPerSec int) (int, error) {
	keys := []string{q.waitingKey(), q.admittedKey(), q.rateKey()}
	res, err := q.admit.Run(ctx, q.client, keys, strconv.Itoa(maxPerSec)).Result()
	if err != nil {
		return 0, err
	}
	tokens, _ := res.([]any)
	return len(tokens), nil
}

// runAdmitLoop is started by main() as a goroutine on every replica.
// The Lua script enforces the global cap, so calling this from N replicas
// in parallel still admits at most maxPerSec tokens per second cluster-wide.
func runAdmitLoop(ctx context.Context, q *queueStore, maxPerSec int) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			n, err := q.admitOnce(ctx, maxPerSec)
			if err != nil && ctx.Err() == nil {
				// Transient errors are normal during Redis blips; do not crash.
				continue
			}
			if n > 0 {
				// Visibility for smoke tests + ops.
				fmt.Printf("admitted %d tokens\n", n)
			}
		}
	}
}
