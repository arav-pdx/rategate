package limiter

import (
	"context"
	"embed"
	"time"
)

//go:embed scripts/*.lua
var LuaScripts embed.FS

type Result struct {
	Allowed   bool
	Limit     int64
	Remaining int64
	ResetAt   time.Time
}

type Limiter interface {
	Allow(ctx context.Context, key string, cost int64) (Result, error)
}
