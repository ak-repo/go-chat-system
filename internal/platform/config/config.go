package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

type AppConfig struct {
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Server   Server         `mapstructure:"server"`
	Redis    RedisConfig    `mapstructure:"redis"`
	CORS     CORS           `mapstructure:"CORS"`
}
type Server struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type CORS struct {
	Host           string   `mapstructure:"host"` // just hostname, not full URL
	Port           int      `mapstructure:"port"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
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
	Secret        string        `mapstructure:"secret"`
	Expiry        time.Duration `mapstructure:"expiry"`
	Issuer        string        `mapstructure:"issuer"`
	RefreshExpiry time.Duration `mapstructure:"refresh_expiry"`
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

var Config AppConfig

// LOAD FUNCTION
func Load() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Read config file FIRST
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Manual env override - only sets values if env var is NON-EMPTY
	overrideFromEnv()

	if err := viper.Unmarshal(&Config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields - if empty after unmarshal, use values from YAML directly
	if Config.Database.Host == "" {
		Config.Database.Host = viper.GetString("database.host")
	}
	if Config.Database.Port == 0 {
		Config.Database.Port = viper.GetInt("database.port")
	}
	if Config.Database.User == "" {
		Config.Database.User = viper.GetString("database.user")
	}
	if Config.Database.Password == "" {
		Config.Database.Password = viper.GetString("database.password")
	}
	if Config.Database.Name == "" {
		Config.Database.Name = viper.GetString("database.name")
	}
	if Config.Database.SSLMode == "" {
		Config.Database.SSLMode = viper.GetString("database.sslmode")
	}
	if Config.Server.Port == 0 {
		Config.Server.Port = viper.GetInt("server.port")
	}
	if Config.Redis.Host == "" {
		Config.Redis.Host = viper.GetString("redis.host")
	}
	if Config.Redis.Port == 0 {
		Config.Redis.Port = viper.GetInt("redis.port")
	}
	if Config.JWT.Secret == "" {
		Config.JWT.Secret = viper.GetString("jwt.secret")
	}

	return nil
}

func overrideFromEnv() {
	// Only override if env var is explicitly set (non-empty)
	if v := os.Getenv("DB_HOST"); v != "" {
		viper.Set("database.host", v)
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			viper.Set("database.port", port)
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		viper.Set("database.user", v)
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		viper.Set("database.password", v)
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		viper.Set("database.name", v)
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		viper.Set("redis.host", v)
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			viper.Set("redis.port", port)
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		viper.Set("jwt.secret", v)
	}
	if v := os.Getenv("PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			viper.Set("server.port", port)
		}
	}
}
