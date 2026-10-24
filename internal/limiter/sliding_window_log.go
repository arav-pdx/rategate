package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type SlidingWindowLogLimiter struct {
	client *redis.Client
	script *redis.Script
	limit  int64
	window time.Duration
}

func NewSlidingWindowLogLimiter(rdb *redis.Client, limit int64, window time.Duration) (*SlidingWindowLogLimiter, error) {
	data, err := LuaScripts.ReadFile("scripts/sliding_window_log.lua")
	if err != nil {
		return nil, fmt.Errorf("reading sliding_window_log.lua: %w", err)
	}
	return &SlidingWindowLogLimiter{
		client: rdb,
		script: redis.NewScript(string(data)),
		limit:  limit,
		window: window,
	}, nil
}

func (l *SlidingWindowLogLimiter) Allow(ctx context.Context, key string, cost int64) (Result, error) {
	res, err := l.script.Run(ctx, l.client, []string{"swl:" + key}, l.limit, l.window.Milliseconds(), cost).Slice()
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
