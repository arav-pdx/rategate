package limiter

import (
	"context"
	"sync"
	"time"
)

type inMemoryTokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

type LocalMemoryLimiter struct {
	mu         sync.Mutex
	capacity   float64
	refillRate float64
	buckets    map[string]*inMemoryTokenBucket
}

func NewLocalMemoryLimiter(capacity, refillRate float64) *LocalMemoryLimiter {
	l := &LocalMemoryLimiter{
		capacity:   capacity,
		refillRate: refillRate,
		buckets:    make(map[string]*inMemoryTokenBucket),
	}
	go l.cleanupLoop()
	return l
}

func (l *LocalMemoryLimiter) Allow(_ context.Context, key string, cost int64) (Result, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, exists := l.buckets[key]
	if !exists {
		b = &inMemoryTokenBucket{tokens: l.capacity, lastRefill: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(l.capacity, b.tokens+elapsed*l.refillRate)
	b.lastRefill = now

	costFloat := float64(cost)
	if b.tokens >= costFloat {
		b.tokens -= costFloat
		return Result{
			Allowed:   true,
			Limit:     int64(l.capacity),
			Remaining: int64(b.tokens),
			ResetAt:   now.Add(time.Second),
		}, nil
	}

	needed := costFloat - b.tokens
	retryAfter := time.Duration((needed / l.refillRate) * float64(time.Second))

	return Result{
		Allowed:   false,
		Limit:     int64(l.capacity),
		Remaining: int64(b.tokens),
		ResetAt:   now.Add(retryAfter),
	}, nil
}

func (l *LocalMemoryLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, b := range l.buckets {
			if now.Sub(b.lastRefill) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
