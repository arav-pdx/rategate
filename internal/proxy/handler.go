package proxy

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	time"

	"github.com/google/uuid"
	"github.com/rategate/rategate/internal/auth"
	"github.com/rategate/rategate/internal/breaker"
	"github.com/rategate/rategate/internal/config"
	"github.com/rategate/rategate/internal/limiter"
	"github.com/rategate/rategate/internal/metrics"
	"github.com/redis/go-redis/v9"
)

type GatewayHandler struct {
	cfgMgr      *config.Manager
	auth        *auth.Authenticator
	router      *Router
	redisClient *redis.Client
	breaker     *breaker.CircuitBreaker
	fallback    *limiter.LocalMemoryLimiter
	limiters    map[string]limiter.Limiter
}

func NewGatewayHandler(
	cfgMgr *config.Manager,
	authenticator *auth.Authenticator,
	router *Router,
	rdb *redis.Client,
) (*GatewayHandler, error) {
	cb := breaker.New(5, 10*time.Second, func(s breaker.State) {
		metrics.BreakerState.Set(float64(s))
	})

	gh := &GatewayHandler{
		cfgMgr:      cfgMgr,
		auth:        authenticator,
		router:      router,
		redisClient: rdb,
		breaker:     cb,
		fallback:    limiter.NewLocalMemoryLimiter(50, 10),
		limiters:    make(map[string]limiter.Limiter),
	}

	if err := gh.initLimiters(); err != nil {
		return nil, err
	}
	return gh, nil
}

func (gh *GatewayHandler) initLimiters() error {
	cfg := gh.cfgMgr.Get()
	for tierName, tierCfg := range cfg.Tiers {
		if tierCfg.Unlimited {
			continue
		}
		var lim limiter.Limiter
		var err error
		switch tierCfg.Algorithm {
		case "token_bucket":
			lim, err = limiter.NewTokenBucketLimiter(gh.redisClient, tierCfg.Capacity, tierCfg.RefillPerSec)
		case "sliding_window_counter":
			lim, err = limiter.NewSlidingWindowCounterLimiter(gh.redisClient, tierCfg.Limit, tierCfg.Window)
		case "sliding_window_log":
			lim, err = limiter.NewSlidingWindowLogLimiter(gh.redisClient, tierCfg.Limit, tierCfg.Window)
		case "fixed_window":
			lim, err = limiter.NewFixedWindowLimiter(gh.redisClient, tierCfg.Limit, tierCfg.Window)
		default:
			lim, err = limiter.NewSlidingWindowCounterLimiter(gh.redisClient, tierCfg.Limit, tierCfg.Window)
		}
		if err != nil {
			return err
		}
		gh.limiters[tierName] = lim
	}
	return nil
}

func (gh *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = uuid.NewString()
	}
	w.Header().Set("X-Request-ID", reqID)

	target := gh.router.Match(r.URL.Path)
	if target == nil {
		http.Error(w, `{"error":"route not found"}`, http.StatusNotFound)
		return
	}

	ident, err := gh.auth.Verify(r.Header.Get("Authorization"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "request_id": reqID})
		return
	}

	tierCfg, tierExists := gh.cfgMgr.Get().Tiers[ident.Tier]
	if !tierExists {
		tierCfg = config.TierConfig{Unlimited: false, Algorithm: "sliding_window_counter", Limit: 60, Window: time.Minute}
	}

	if !tierCfg.Unlimited {
		lim := gh.limiters[ident.Tier]
		key := gh.resolveKey(ident, target.Config, r)
		cost := target.Config.Cost
		if cost <= 0 {
			cost = 1
		}

		var limResult limiter.Result
		start := time.Now()
		execErr := gh.breaker.Execute(func() error {
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(gh.cfgMgr.Get().Redis.TimeoutMs)*time.Millisecond)
			defer cancel()
			var err error
			limResult, err = lim.Allow(ctx, key, cost)
			return err
		})
		metrics.LimiterDuration.WithLabelValues(tierCfg.Algorithm).Observe(time.Since(start).Seconds())

		if execErr != nil {
			metrics.LimiterErrorsTotal.WithLabelValues(target.Config.OnLimiterError).Inc()
			if target.Config.OnLimiterError == "fail_closed" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limiter unavailable", "request_id": reqID})
				return
			}

			fbResult, _ := gh.fallback.Allow(r.Context(), key, cost)
			if !fbResult.Allowed {
				gh.respondRateLimited(w, fbResult, target.Config.PathPrefix, ident.Tier, reqID)
				return
			}
		} else if !limResult.Allowed {
			gh.respondRateLimited(w, limResult, target.Config.PathPrefix, ident.Tier, reqID)
			return
		} else {
			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limResult.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(limResult.Remaining, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(limResult.ResetAt.Unix(), 10))
		}
	}

	r.Header.Set("X-Request-ID", reqID)
	r.Header.Set("X-Consumer-Subject", ident.Subject)
	r.Header.Set("X-Consumer-Tier", ident.Tier)

	rw := &responseInterceptor{ResponseWriter: w, status: http.StatusOK}
	upstreamStart := time.Now()
	target.Proxy.ServeHTTP(rw, r)

	metrics.UpstreamDuration.WithLabelValues(target.Config.PathPrefix).Observe(time.Since(upstreamStart).Seconds())
	metrics.RequestsTotal.WithLabelValues(target.Config.PathPrefix, strconv.Itoa(rw.status)).Inc()

	slog.Info("handled request",
		"request_id", reqID,
		"client", ident.Subject,
		"tier", ident.Tier,
		"route", target.Config.PathPrefix,
		"status", rw.status,
	)
}

func (gh *GatewayHandler) respondRateLimited(w http.ResponseWriter, res limiter.Result, pathPrefix, tier, reqID string) {
	metrics.LimitedTotal.WithLabelValues(pathPrefix, tier).Inc()
	retryAfter := int(time.Until(res.ResetAt).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}

	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(res.Limit, 10))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(res.ResetAt.Unix(), 10))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":       "rate limit exceeded",
		"retry_after": retryAfter,
		"request_id":  reqID,
	})
}

func (gh *GatewayHandler) resolveKey(ident auth.Identity, route config.RouteConfig, r *http.Request) string {
	switch route.KeyScope {
	case "ip":
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		return "ip:" + ip
	case "client+route":
		return ident.Subject + ":" + route.PathPrefix
	default:
		return ident.Subject
	}
}

type responseInterceptor struct {
	http.ResponseWriter
	status int
}

func (ri *responseInterceptor) WriteHeader(statusCode int) {
	ri.status = statusCode
	ri.ResponseWriter.WriteHeader(statusCode)
}
