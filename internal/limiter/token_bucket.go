package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenBucketLimiter struct {
	client     *redis.Client
	script     *redis.Script
	capacity   int64
	refillRate int64
}

func NewTokenBucketLimiter(rdb *redis.Client, capacity, refillRate int64) (*TokenBucketLimiter, error) {
	scriptData, err := LuaScripts.ReadFile("scripts/token_bucket.lua")
	if err != nil {
		return nil, fmt.Errorf("reading token_bucket.lua: %w", err)
	}
	return &TokenBucketLimiter{
		client:     rdb,
		script:     redis.NewScript(string(scriptData)),
		capacity:   capacity,
		refillRate: refillRate,
	}, nil
}

func (l *TokenBucketLimiter) Allow(ctx context.Context, key string, cost int64) (Result, error) {
	res, err := l.script.Run(ctx, l.client, []string{"tb:" + key}, l.capacity, l.refillRate, cost).Slice()
	if err != nil {
		return Result{}, err
	}

	allowed := res[0].(int64) == 1
	remaining := res[1].(int64)
	resetSec := res[2].(int64)

	return Result{
		Allowed:   allowed,
		Limit:     l.capacity,
		Remaining: remaining,
		ResetAt:   time.Unix(resetSec, 0),
	}, nil
}
