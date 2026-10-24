package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rategate/rategate/internal/auth"
	"github.com/rategate/rategate/internal/config"
	"github.com/rategate/rategate/internal/proxy"
	"github.com/redis/go-redis/v9"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to YAML config file")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfgMgr, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}
	cfg := cfgMgr.Get()

	authenticator, err := auth.NewAuthenticator(cfg.Auth.JWTPublicKeyFile, cfg.Auth.TierClaim)
	if err != nil {
		slog.Error("failed to initialize jwt authenticator", "error", err)
		os.Exit(1)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  time.Duration(cfg.Redis.TimeoutMs) * time.Millisecond,
		ReadTimeout:  time.Duration(cfg.Redis.TimeoutMs) * time.Millisecond,
		WriteTimeout: time.Duration(cfg.Redis.TimeoutMs) * time.Millisecond,
	})

	router, err := proxy.NewRouter(cfg.Routes)
	if err != nil {
		slog.Error("failed to initialize router", "error", err)
		os.Exit(1)
	}

	handler, err := proxy.NewGatewayHandler(cfgMgr, authenticator, router, rdb)
	if err != nil {
		slog.Error("failed to create gateway handler", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	mux.Handle("/", handler)

	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}
	addr := fmt.Sprintf(":%d", port)
	slog.Info("starting rategate gateway", "listen_addr", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server terminated", "error", err)
	}
}
