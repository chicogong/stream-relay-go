package internal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 配置结构 - 极简版
type Config struct {
	Server        ServerConfig         `yaml:"server"`
	Routes        []RouteConfig        `yaml:"routes"`
	Storage       StorageConfig        `yaml:"storage"`
	RateLimit     RateLimitConfig      `yaml:"rate_limit"`
	Observability ObservabilityConfig  `yaml:"observability"`
	Auth          AuthConfig           `yaml:"auth"`
}

type ServerConfig struct {
	Port        int           `yaml:"port"`
	Timeout     time.Duration `yaml:"timeout"`
	MaxBodySize int64         `yaml:"max_body_size"`
}

type RouteConfig struct {
	Name       string `yaml:"name"`
	Path       string `yaml:"path"`
	Upstream   string `yaml:"upstream"`
	AuthHeader string `yaml:"auth_header"`
	AuthEnv    string `yaml:"auth_env"`  // 从环境变量读取
	Kind       string `yaml:"kind"`      // sse | raw
}

type StorageConfig struct {
	Redis      RedisConfig      `yaml:"redis"`
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	TTL      int    `yaml:"ttl"`
}

type ClickHouseConfig struct {
	Addr     string `yaml:"addr"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Async    bool   `yaml:"async"`
}

type RateLimitConfig struct {
	Enabled bool `yaml:"enabled"`
	Default int  `yaml:"default"`  // requests per minute
	Burst   int  `yaml:"burst"`
}

type ObservabilityConfig struct {
	Prometheus   PrometheusConfig `yaml:"prometheus"`
	Logging      LoggingConfig    `yaml:"logging"`
	DetailedLogs bool             `yaml:"detailed_logs"`
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type AuthConfig struct {
	APIKeys []string `yaml:"api_keys"`
}

// LoadConfig 加载配置文件
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// 替换环境变量
	content := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}

	if len(c.Routes) == 0 {
		return fmt.Errorf("no routes configured")
	}

	for _, route := range c.Routes {
		if route.Name == "" {
			return fmt.Errorf("route name is required")
		}
		if route.Path == "" {
			return fmt.Errorf("route path is required for %s", route.Name)
		}
		if route.Upstream == "" {
			return fmt.Errorf("route upstream is required for %s", route.Name)
		}
		if route.Kind != "sse" && route.Kind != "raw" {
			return fmt.Errorf("invalid route kind: %s (must be 'sse' or 'raw')", route.Kind)
		}
	}

	return nil
}

// GetRouteByPath 根据路径匹配路由
func (c *Config) GetRouteByPath(path string) *RouteConfig {
	for _, route := range c.Routes {
		if strings.HasPrefix(path, route.Path) {
			return &route
		}
	}
	return nil
}

// GetAuthValue 获取认证值（从环境变量）
func (r *RouteConfig) GetAuthValue() string {
	if r.AuthEnv != "" {
		return os.Getenv(r.AuthEnv)
	}
	return ""
}
