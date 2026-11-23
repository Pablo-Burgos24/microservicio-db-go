package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/api"
)

// ConfigWatcher watches Consul KV for configuration changes and hot-reloads.
type ConfigWatcher struct {
	client       *api.Client
	prefix       string
	token        string
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	currentConfig atomic.Pointer[map[string]string]
	onChange     func(map[string]string)
}

// ConfigWatcherConfig holds configuration for the config watcher.
type ConfigWatcherConfig struct {
	ConsulAddress string
	ConsulToken   string
	Prefix        string // e.g., "config/microservicio-db/"
	OnChange      func(map[string]string)
}

// NewConfigWatcher creates a new ConfigWatcher.
func NewConfigWatcher(config ConfigWatcherConfig) (*ConfigWatcher, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = config.ConsulAddress
	consulConfig.Token = config.ConsulToken

	client, err := api.NewClient(consulConfig)
	if err != nil {
		return nil, fmt.Errorf("create consul client: %w", err)
	}

	if config.Prefix == "" {
		return nil, fmt.Errorf("prefix is required")
	}
	// Ensure prefix ends with /
	if !strings.HasSuffix(config.Prefix, "/") {
		config.Prefix += "/"
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &ConfigWatcher{
		client:  client,
		prefix:  config.Prefix,
		token:   config.ConsulToken,
		cancel:  cancel,
		onChange: config.OnChange,
	}

	// Initialize with empty config
	w.currentConfig.Store(&map[string]string{})

	// Start watching
	w.wg.Add(1)
	go w.watch(ctx)

	// Do initial fetch
	w.fetchAndUpdate(ctx)

	return w, nil
}

// GetConfig returns the current configuration snapshot.
func (w *ConfigWatcher) GetConfig() map[string]string {
	ptr := w.currentConfig.Load()
	if ptr == nil {
		return map[string]string{}
	}
	return *ptr
}

// GetString returns a string config value, checking Consul first then env var.
func (w *ConfigWatcher) GetString(key, envKey, fallback string) string {
	if val, ok := w.GetConfig()[key]; ok && val != "" {
		return val
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

// GetInt returns an int config value, checking Consul first then env var.
func (w *ConfigWatcher) GetInt(key, envKey string, fallback int) int {
	if val, ok := w.GetConfig()[key]; ok && val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			return v
		}
	}
	if v := os.Getenv(envKey); v != "" {
		if val, err := strconv.Atoi(v); err == nil {
			return val
		}
	}
	return fallback
}

// GetInt64 returns an int64 config value, checking Consul first then env var.
func (w *ConfigWatcher) GetInt64(key, envKey string, fallback int64) int64 {
	if val, ok := w.GetConfig()[key]; ok && val != "" {
		if v, err := strconv.ParseInt(val, 10, 64); err == nil {
			return v
		}
	}
	if v := os.Getenv(envKey); v != "" {
		if val, err := strconv.ParseInt(v, 10, 64); err == nil {
			return val
		}
	}
	return fallback
}

// GetDuration returns a time.Duration config value, checking Consul first then env var.
func (w *ConfigWatcher) GetDuration(key, envKey string, fallback time.Duration) time.Duration {
	if val, ok := w.GetConfig()[key]; ok && val != "" {
		if v, err := time.ParseDuration(val); err == nil {
			return v
		}
	}
	if v := os.Getenv(envKey); v != "" {
		if val, err := time.ParseDuration(v); err == nil {
			return val
		}
	}
	return fallback
}

// GetFloat64 returns a float64 config value, checking Consul first then env var.
func (w *ConfigWatcher) GetFloat64(key, envKey string, fallback float64) float64 {
	if val, ok := w.GetConfig()[key]; ok && val != "" {
		if v, err := strconv.ParseFloat(val, 64); err == nil {
			return v
		}
	}
	if v := os.Getenv(envKey); v != "" {
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			return val
		}
	}
	return fallback
}

// Stop stops the config watcher.
func (w *ConfigWatcher) Stop() {
	w.cancel()
	w.wg.Wait()
	log.Printf("[consul-config] Config watcher stopped for prefix: %s", w.prefix)
}

// watch runs the blocking query loop.
func (w *ConfigWatcher) watch(ctx context.Context) {
	defer w.wg.Done()

	queryOpts := &api.QueryOptions{
		Token: w.token,
		WaitIndex: 0,
		WaitTime:  5 * time.Minute, // Long polling timeout
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Blocking query on the prefix
			kvp, meta, err := w.client.KV().List(w.prefix, queryOpts)
			if err != nil {
				log.Printf("[consul-config] Error watching config: %v", err)
				// Back off on error
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
				continue
			}

			// Update wait index for next query
			if meta != nil {
				queryOpts.WaitIndex = meta.LastIndex
			}

			// Parse and update config
			w.parseAndUpdate(kvp)
		}
	}
}

// fetchAndUpdate does an initial fetch (non-blocking).
func (w *ConfigWatcher) fetchAndUpdate(ctx context.Context) {
	kvp, _, err := w.client.KV().List(w.prefix, &api.QueryOptions{Token: w.token})
	if err != nil {
		log.Printf("[consul-config] Initial config fetch failed: %v", err)
		return
	}
	w.parseAndUpdate(kvp)
}

// parseAndUpdate parses KV pairs and updates the atomic config.
func (w *ConfigWatcher) parseAndUpdate(kvp api.KVPairs) {
	config := make(map[string]string)
	for _, pair := range kvp {
		// Strip the prefix to get the key
		key := strings.TrimPrefix(pair.Key, w.prefix)
		if key == "" {
			continue
		}
		// Decode value (Consul stores base64, but api.KV().List decodes it)
		val := string(pair.Value)
		config[key] = val
	}

	// Atomic swap
	w.currentConfig.Store(&config)

	log.Printf("[consul-config] Config updated (%d keys): %v", len(config), keys(config))

	// Notify callback if set
	if w.onChange != nil {
		go w.onChange(config)
	}
}

// keys returns the keys of a map for logging.
func keys(m map[string]string) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	return k
}

// UnmarshalJSONConfig parses a JSON-encoded config value into a struct.
func UnmarshalJSONConfig(val string, target interface{}) error {
	return json.Unmarshal([]byte(val), target)
}

// MarshalJSONConfig marshals a struct to JSON for Consul KV.
func MarshalJSONConfig(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}