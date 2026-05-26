package controlplane

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Config struct {
	RateLimit    float64  `json:"rate_limit"`
	MaxBodyBytes int64    `json:"max_body_bytes"`
	Blocklist    []string `json:"blocklist"`
	Allowlist    []string `json:"allowlist"`
	Origins      []string `json:"origins"`
	CacheMaxItems int     `json:"cache_max_items"`
	CacheTTLSecs  int     `json:"cache_ttl_secs"`
}

type ConfigWatcher struct {
	mu         sync.RWMutex
	config     Config
	configPath string
	onChange   []func(Config)
}

func DefaultConfig() Config {
	return Config{
		RateLimit:     100,
		MaxBodyBytes:  1 << 20,
		Blocklist:     []string{},
		Allowlist:     []string{},
		Origins:       []string{"http://localhost:9000", "http://localhost:9001"},
		CacheMaxItems: 1000,
		CacheTTLSecs:  60,
	}
}

func NewConfigWatcher(path string) (*ConfigWatcher, error) {
	cw := &ConfigWatcher{
		configPath: path,
		config:     DefaultConfig(),
	}

	// Write default config if file doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		data, _ := json.MarshalIndent(cw.config, "", "  ")
		os.WriteFile(path, data, 0644)
		slog.Info("created default config", "path", path)
	} else {
		if err := cw.load(); err != nil {
			return nil, err
		}
	}

	go cw.watch()
	return cw, nil
}

func (cw *ConfigWatcher) Get() Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.config
}

func (cw *ConfigWatcher) OnChange(fn func(Config)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onChange = append(cw.onChange, fn)
}

func (cw *ConfigWatcher) load() error {
	data, err := os.ReadFile(cw.configPath)
	if err != nil {
		return err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	cw.mu.Lock()
	cw.config = cfg
	callbacks := cw.onChange
	cw.mu.Unlock()

	for _, fn := range callbacks {
		fn(cfg)
	}
	slog.Info("config reloaded", "path", cw.configPath)
	return nil
}

func (cw *ConfigWatcher) watch() {
	var lastMod time.Time
	for {
		time.Sleep(2 * time.Second)
		info, err := os.Stat(cw.configPath)
		if err != nil {
			continue
		}
		if info.ModTime().After(lastMod) {
			lastMod = info.ModTime()
			if lastMod.IsZero() {
				continue
			}
			if err := cw.load(); err != nil {
				slog.Error("failed to reload config", "error", err)
			}
		}
	}
}
