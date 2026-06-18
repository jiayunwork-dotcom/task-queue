package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"task-queue/internal/cache"
)

type RateLimitConfig struct {
	TaskType      string `json:"task_type"`
	MaxPerSecond  int    `json:"max_per_second"`
	WindowSizeMs  int    `json:"window_size_ms"`
	Enabled       bool   `json:"enabled"`
	UpdatedAt     int64  `json:"updated_at"`
}

type ConfigManager struct {
	cache   *cache.Cache
	configs map[string]*RateLimitConfig
	mu      sync.RWMutex
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewConfigManager(cache *cache.Cache) *ConfigManager {
	return &ConfigManager{
		cache:   cache,
		configs: make(map[string]*RateLimitConfig),
		stopCh:  make(chan struct{}),
	}
}

func (cm *ConfigManager) Start(ctx context.Context) {
	cm.wg.Add(1)
	go cm.watchConfigChanges(ctx)
}

func (cm *ConfigManager) Stop() {
	close(cm.stopCh)
	cm.wg.Wait()
}

func (cm *ConfigManager) GetConfig(taskType string) *RateLimitConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cfg, ok := cm.configs[taskType]; ok {
		if cfg.Enabled {
			return cfg
		}
		return nil
	}
	return nil
}

func (cm *ConfigManager) GetAllConfigs() map[string]*RateLimitConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*RateLimitConfig, len(cm.configs))
	for k, v := range cm.configs {
		result[k] = v
	}
	return result
}

func (cm *ConfigManager) SetConfig(ctx context.Context, cfg *RateLimitConfig) error {
	if cfg.TaskType == "" {
		return fmt.Errorf("task type is required")
	}
	if cfg.MaxPerSecond < 0 {
		return fmt.Errorf("max_per_second must be >= 0")
	}
	if cfg.WindowSizeMs <= 0 {
		cfg.WindowSizeMs = 1000
	}
	if cfg.WindowSizeMs < 100 {
		cfg.WindowSizeMs = 100
	}

	cfg.UpdatedAt = time.Now().UnixNano()

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	key := configKey(cfg.TaskType)
	if err := cm.cache.Client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("save config to redis: %w", err)
	}

	cm.mu.Lock()
	cm.configs[cfg.TaskType] = cfg
	cm.mu.Unlock()

	_ = cm.cache.Client.Publish(ctx, cache.ChannelRateLimitConfig, string(data)).Err()

	return nil
}

func (cm *ConfigManager) DeleteConfig(ctx context.Context, taskType string) error {
	key := configKey(taskType)
	if err := cm.cache.Client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete config from redis: %w", err)
	}

	cm.mu.Lock()
	delete(cm.configs, taskType)
	cm.mu.Unlock()

	return nil
}

func (cm *ConfigManager) LoadAllConfigs(ctx context.Context) error {
	pattern := configKey("*")
	iter := cm.cache.Client.Scan(ctx, 0, pattern, 100).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		data, err := cm.cache.Client.Get(ctx, key).Bytes()
		if err != nil {
			continue
		}

		var cfg RateLimitConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}

		cm.mu.Lock()
		cm.configs[cfg.TaskType] = &cfg
		cm.mu.Unlock()
	}

	return iter.Err()
}

func (cm *ConfigManager) watchConfigChanges(ctx context.Context) {
	defer cm.wg.Done()

	pubsub := cm.cache.Client.Subscribe(ctx, cache.ChannelRateLimitConfig)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopCh:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var cfg RateLimitConfig
			if err := json.Unmarshal([]byte(msg.Payload), &cfg); err == nil {
				cm.mu.Lock()
				cm.configs[cfg.TaskType] = &cfg
				cm.mu.Unlock()
			}
		}
	}
}

func configKey(taskType string) string {
	return fmt.Sprintf("%s%s", cache.KeyPrefixRateLimitConfig, taskType)
}
