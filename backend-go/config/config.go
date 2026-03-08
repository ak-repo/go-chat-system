package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Server   Server         `mapstructure:"server"`
	Redis    RedisConfig    `mapstructure:"redis"`
	CORS      CORS            `mapstructure:"CORS"`
	WebSocket WebSocketConfig `mapstructure:"websocket"`
}
type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type CORS struct {
	Host         string   `mapstructure:"host"`
	Port         int      `mapstructure:"port"`
	AllowOrigins []string `mapstructure:"allow_origins"` // optional override for production; if set, used for CORS and WS CheckOrigin
}

// DATABASE
type DatabaseConfig struct {
	Host     string       `mapstructure:"host"`
	Port     int          `mapstructure:"port"`
	User     string       `mapstructure:"user"`
	Password string       `mapstructure:"password"`
	Name     string       `mapstructure:"name"`
	SSLMode  string       `mapstructure:"sslmode"`
	Pool     DBPoolConfig `mapstructure:"pool"`
}

type DBPoolConfig struct {
	MaxConnections    int    `mapstructure:"max_connections"`
	MinConnections    int    `mapstructure:"min_connections"`
	MaxConnLifetime   string `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime   string `mapstructure:"max_conn_idle_time"`
	HealthCheckPeriod string `mapstructure:"health_check_period"`
}

// JWT
type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	Expiry time.Duration `mapstructure:"expiry"`
	Issuer string        `mapstructure:"issuer"`
}

// Redis
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// LOGGING
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// WebSocket
type WebSocketConfig struct {
	MaxMessageSize   int64 `mapstructure:"max_message_size"`   // bytes; 0 = default 512
	ReadDeadlineSec  int   `mapstructure:"read_deadline_sec"`   // ping/read timeout
	MessagesPerSec   int   `mapstructure:"messages_per_sec"`    // per-client rate limit; 0 = no limit
}

var Config AppConfig

// LOAD FUNCTION loads config from YAML and overrides with env vars (e.g. DATABASE_HOST).
// Env vars take precedence. Do not put secrets in repo; use env in production.
func Load() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.AutomaticEnv() // e.g. DATABASE_HOST overrides database.host

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := viper.Unmarshal(&Config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applyEnvOverrides()
	return nil
}

// applyEnvOverrides overrides Config with env vars when set. Use in production to avoid secrets in repo.
func applyEnvOverrides() {
	if v := os.Getenv("DATABASE_HOST"); v != "" {
		Config.Database.Host = v
	}
	if v := os.Getenv("DATABASE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			Config.Database.Port = n
		}
	}
	if v := os.Getenv("DATABASE_USER"); v != "" {
		Config.Database.User = v
	}
	if v := os.Getenv("DATABASE_PASSWORD"); v != "" {
		Config.Database.Password = v
	}
	if v := os.Getenv("DATABASE_NAME"); v != "" {
		Config.Database.Name = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			Config.Server.Port = n
		}
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		Config.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			Config.Redis.Port = n
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		Config.JWT.Secret = v
	}
	if v := os.Getenv("CORS_HOST"); v != "" {
		Config.CORS.Host = v
	}
	if v := os.Getenv("CORS_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			Config.CORS.Port = n
		}
	}
	if v := os.Getenv("CORS_ALLOW_ORIGINS"); v != "" {
		Config.CORS.AllowOrigins = strings.Split(v, ",")
		for i := range Config.CORS.AllowOrigins {
			Config.CORS.AllowOrigins[i] = strings.TrimSpace(Config.CORS.AllowOrigins[i])
		}
	}
}
