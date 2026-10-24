package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rategate_requests_total",
			Help: "Total HTTP requests processed by route and status",
		},
		[]string{"route", "status"},
	)

	LimitedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rategate_limited_total",
			Help: "Total rate-limited requests by route and client tier",
		},
		[]string{"route", "tier"},
	)

	LimiterDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rategate_limiter_duration_seconds",
			Help:    "Latency incurred checking limits in Redis or fallback",
			Buckets: []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.02, 0.05, 0.1},
		},
		[]string{"algorithm"},
	)

	LimiterErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rategate_limiter_errors_total",
			Help: "Count of errors contacting the central Redis limiter",
		},
		[]string{"mode"},
	)

	BreakerState = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rategate_breaker_state",
			Help: "Redis circuit breaker status (0: Closed, 1: Open, 2: HalfOpen)",
		},
	)

	UpstreamDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rategate_upstream_duration_seconds",
			Help:    "Time spent waiting for response from upstream service",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route"},
	)
)
