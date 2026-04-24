package config

import (
	"fmt"
	"os"
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
	Host           string   `mapstructure:"host"`
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

	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	viper.RegisterAlias("database.host", "DB_HOST")
	viper.RegisterAlias("database.port", "DB_PORT")
	viper.RegisterAlias("database.user", "DB_USER")
	viper.RegisterAlias("database.password", "DB_PASSWORD")
	viper.RegisterAlias("database.name", "DB_NAME")
	viper.RegisterAlias("database.sslmode", "DB_SSLMODE")

	viper.RegisterAlias("redis.host", "REDIS_HOST")
	viper.RegisterAlias("redis.port", "REDIS_PORT")
	viper.RegisterAlias("redis.password", "REDIS_PASSWORD")
	viper.RegisterAlias("redis.db", "REDIS_DB")

	viper.RegisterAlias("jwt.secret", "JWT_SECRET")
	viper.RegisterAlias("jwt.expiry", "JWT_EXPIRY")

	viper.RegisterAlias("server.port", "PORT")
	viper.RegisterAlias("server.host", "HOST")

	viper.RegisterAlias("logging.level", "LOG_LEVEL")
	viper.RegisterAlias("logging.format", "LOG_FORMAT")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	overrideFromEnv()

	if err := viper.Unmarshal(&Config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

func overrideFromEnv() {
	if v := os.Getenv("DB_HOST"); v != "" {
		viper.Set("database.host", v)
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		viper.Set("database.port", v)
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
		viper.Set("redis.port", v)
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		viper.Set("jwt.secret", v)
	}
	if v := os.Getenv("PORT"); v != "" {
		viper.Set("server.port", v)
	}
}
