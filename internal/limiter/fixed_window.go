package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type FixedWindowLimiter struct {
	client *redis.Client
	script *redis.Script
	limit  int64
	window time.Duration
}

func NewFixedWindowLimiter(rdb *redis.Client, limit int64, window time.Duration) (*FixedWindowLimiter, error) {
	data, err := LuaScripts.ReadFile("scripts/fixed_window.lua")
	if err != nil {
		return nil, fmt.Errorf("reading fixed_window.lua: %w", err)
	}
	return &FixedWindowLimiter{
		client: rdb,
		script: redis.NewScript(string(data)),
		limit:  limit,
		window: window,
	}, nil
}

func (l *FixedWindowLimiter) Allow(ctx context.Context, key string, cost int64) (Result, error) {
	windowSec := int64(l.window.Seconds())
	if windowSec <= 0 {
		windowSec = 1
	}
	windowStart := time.Now().Unix() / windowSec
	fullKey := fmt.Sprintf("fw:%s:%d", key, windowStart)

	res, err := l.script.Run(ctx, l.client, []string{fullKey}, l.limit, l.window.Milliseconds(), cost).Slice()
	if err != nil {
		return Result{}, err
	}

	return Result{
		Allowed:   res[0].(int64) == 1,
		Limit:     l.limit,
		Remaining: res[1].(int64),
		ResetAt:   time.Unix(res[2].(int64), 0),
	}, nil
}
