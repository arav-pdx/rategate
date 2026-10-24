package config

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Redis struct {
		Addr      string `yaml:"addr"`
		Password  string `yaml:"password"`
		DB        int    `yaml:"db"`
		TimeoutMs int    `yaml:"timeout_ms"`
	} `yaml:"redis"`
	Auth struct {
		JWTPublicKeyFile string `yaml:"jwt_public_key_file"`
		TierClaim        string `yaml:"tier_claim"`
	} `yaml:"auth"`
	Tiers  map[string]TierConfig `yaml:"tiers"`
	Routes []RouteConfig         `yaml:"routes"`
}

type TierConfig struct {
	Algorithm    string        `yaml:"algorithm"`
	Limit        int64         `yaml:"limit"`
	Window       time.Duration `yaml:"window"`
	Capacity     int64         `yaml:"capacity"`
	RefillPerSec int64         `yaml:"refill_per_sec"`
	Unlimited    bool          `yaml:"unlimited"`
}

type RouteConfig struct {
	PathPrefix     string `yaml:"path_prefix"`
	Upstream       string `yaml:"upstream"`
	KeyScope       string `yaml:"key_scope"`
	OnLimiterError string `yaml:"on_limiter_error"`
	Cost           int64  `yaml:"cost"`
}

type Manager struct {
	mu     sync.RWMutex
	path   string
	cfg    *Config
}

func Load(path string) (*Manager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	mgr := &Manager{
		path: path,
		cfg:  &cfg,
	}

	go mgr.watchSignals()
	return mgr, nil
}

func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) Reload() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	var newCfg Config
	if err := yaml.Unmarshal(data, &newCfg); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = &newCfg
	m.mu.Unlock()
	return nil
}

func (m *Manager) watchSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)
	for range sigChan {
		_ = m.Reload()
	}
}
