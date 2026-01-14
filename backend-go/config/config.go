package config

import (
	"fmt"
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
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
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

var Config AppConfig

// LOAD FUNCTION
func Load() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := viper.Unmarshal(&Config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}
