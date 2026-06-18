package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Postgres  PostgresConfig  `mapstructure:"postgres"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Queue     QueueConfig     `mapstructure:"queue"`
	Worker    WorkerConfig    `mapstructure:"worker"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
}

type ServerConfig struct {
	Port         int    `mapstructure:"port"`
	GRPCPort     int    `mapstructure:"grpc_port"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	Mode         string `mapstructure:"mode"`
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	PoolSize int    `mapstructure:"pool_size"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	DialTimeout  int    `mapstructure:"dial_timeout"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
}

type QueueConfig struct {
	PriorityLevels    int `mapstructure:"priority_levels"`
	FairnessN         int `mapstructure:"fairness_n"`
	DelayScanInterval int `mapstructure:"delay_scan_interval"`
	LeaseTTL          int `mapstructure:"lease_ttl"`
	MaxRetries        int `mapstructure:"max_retries"`
}

type WorkerConfig struct {
	DefaultSlots             int `mapstructure:"default_slots"`
	HeartbeatInterval        int `mapstructure:"heartbeat_interval"`
	HeartbeatTimeout         int `mapstructure:"heartbeat_timeout"`
	GracefulShutdownTimeout  int `mapstructure:"graceful_shutdown_timeout"`
}

type SchedulerConfig struct {
	DispatchInterval int `mapstructure:"dispatch_interval"`
	BatchSize        int `mapstructure:"batch_size"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/task-queue")

	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.grpc_port", 50051)
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.mode", "release")

	viper.SetDefault("postgres.host", "localhost")
	viper.SetDefault("postgres.port", 5432)
	viper.SetDefault("postgres.user", "postgres")
	viper.SetDefault("postgres.password", "postgres")
	viper.SetDefault("postgres.dbname", "task_queue")
	viper.SetDefault("postgres.sslmode", "disable")
	viper.SetDefault("postgres.pool_size", 50)

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.dial_timeout", 5)
	viper.SetDefault("redis.read_timeout", 3)
	viper.SetDefault("redis.write_timeout", 3)

	viper.SetDefault("queue.priority_levels", 5)
	viper.SetDefault("queue.fairness_n", 10)
	viper.SetDefault("queue.delay_scan_interval", 1)
	viper.SetDefault("queue.lease_ttl", 30)
	viper.SetDefault("queue.max_retries", 5)

	viper.SetDefault("worker.default_slots", 10)
	viper.SetDefault("worker.heartbeat_interval", 5)
	viper.SetDefault("worker.heartbeat_timeout", 15)
	viper.SetDefault("worker.graceful_shutdown_timeout", 300)

	viper.SetDefault("scheduler.dispatch_interval", 10)
	viper.SetDefault("scheduler.batch_size", 100)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config error: %w", err)
		}
	}

	viper.SetEnvPrefix("TQ")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	envBindings := []string{
		"server.port", "server.grpc_port", "server.read_timeout", "server.write_timeout", "server.mode",
		"postgres.host", "postgres.port", "postgres.user", "postgres.password",
		"postgres.dbname", "postgres.sslmode", "postgres.pool_size",
		"redis.host", "redis.port", "redis.password", "redis.db",
		"redis.pool_size", "redis.dial_timeout", "redis.read_timeout", "redis.write_timeout",
		"queue.priority_levels", "queue.fairness_n", "queue.delay_scan_interval",
		"queue.lease_ttl", "queue.max_retries",
		"worker.default_slots", "worker.heartbeat_interval", "worker.heartbeat_timeout",
		"worker.graceful_shutdown_timeout",
		"scheduler.dispatch_interval", "scheduler.batch_size",
	}
	for _, key := range envBindings {
		_ = viper.BindEnv(key)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config error: %w", err)
	}

	applyDirectEnvOverrides(&cfg)

	return &cfg, nil
}

func applyDirectEnvOverrides(cfg *Config) {
	if v := os.Getenv("TQ_POSTGRES_HOST"); v != "" {
		cfg.Postgres.Host = v
	}
	if v := os.Getenv("TQ_POSTGRES_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Postgres.Port)
	}
	if v := os.Getenv("TQ_POSTGRES_USER"); v != "" {
		cfg.Postgres.User = v
	}
	if v := os.Getenv("TQ_POSTGRES_PASSWORD"); v != "" {
		cfg.Postgres.Password = v
	}
	if v := os.Getenv("TQ_POSTGRES_DBNAME"); v != "" {
		cfg.Postgres.DBName = v
	}
	if v := os.Getenv("TQ_POSTGRES_SSLMODE"); v != "" {
		cfg.Postgres.SSLMode = v
	}
	if v := os.Getenv("TQ_POSTGRES_POOL_SIZE"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Postgres.PoolSize)
	}
	if v := os.Getenv("TQ_REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("TQ_REDIS_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Redis.Port)
	}
	if v := os.Getenv("TQ_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("TQ_REDIS_DB"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Redis.DB)
	}
	if v := os.Getenv("TQ_SERVER_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Server.Port)
	}
}

func (c *PostgresConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode, c.PoolSize)
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *WorkerConfig) HeartbeatIntervalDuration() time.Duration {
	return time.Duration(c.HeartbeatInterval) * time.Second
}

func (c *WorkerConfig) HeartbeatTimeoutDuration() time.Duration {
	return time.Duration(c.HeartbeatTimeout) * time.Second
}

func (c *ServerConfig) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
