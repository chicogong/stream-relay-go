package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	content := `
server:
  port: 8080
  timeout: 30s
  max_body_size: 10485760

routes:
  - name: test-route
    path: /v1/test
    upstream: https://api.example.com
    auth_header: Authorization
    auth_env: TEST_API_KEY
    kind: sse

rate_limit:
  enabled: true
  default: 100
  burst: 20

auth:
  api_keys:
    - sk-test-key-123
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify server config
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", cfg.Server.Timeout)
	}

	// Verify routes
	if len(cfg.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(cfg.Routes))
	}
	if cfg.Routes[0].Name != "test-route" {
		t.Errorf("expected route name 'test-route', got '%s'", cfg.Routes[0].Name)
	}

	// Verify rate limit
	if !cfg.RateLimit.Enabled {
		t.Error("expected rate_limit.enabled to be true")
	}
	if cfg.RateLimit.Default != 100 {
		t.Errorf("expected rate_limit.default 100, got %d", cfg.RateLimit.Default)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []RouteConfig{
					{Name: "test", Path: "/test", Upstream: "https://example.com", Kind: "sse"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port - zero",
			config: Config{
				Server: ServerConfig{Port: 0},
				Routes: []RouteConfig{
					{Name: "test", Path: "/test", Upstream: "https://example.com", Kind: "sse"},
				},
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "invalid port - too high",
			config: Config{
				Server: ServerConfig{Port: 70000},
				Routes: []RouteConfig{
					{Name: "test", Path: "/test", Upstream: "https://example.com", Kind: "sse"},
				},
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "no routes",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []RouteConfig{},
			},
			wantErr: true,
			errMsg:  "no routes",
		},
		{
			name: "route without name",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []RouteConfig{
					{Name: "", Path: "/test", Upstream: "https://example.com", Kind: "sse"},
				},
			},
			wantErr: true,
			errMsg:  "route name is required",
		},
		{
			name: "route without path",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []RouteConfig{
					{Name: "test", Path: "", Upstream: "https://example.com", Kind: "sse"},
				},
			},
			wantErr: true,
			errMsg:  "route path is required",
		},
		{
			name: "route without upstream",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []RouteConfig{
					{Name: "test", Path: "/test", Upstream: "", Kind: "sse"},
				},
			},
			wantErr: true,
			errMsg:  "route upstream is required",
		},
		{
			name: "route with invalid kind",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Routes: []RouteConfig{
					{Name: "test", Path: "/test", Upstream: "https://example.com", Kind: "invalid"},
				},
			},
			wantErr: true,
			errMsg:  "invalid route kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConfig_GetRouteByPath(t *testing.T) {
	cfg := &Config{
		Routes: []RouteConfig{
			{Name: "chat", Path: "/v1/chat", Upstream: "https://api.example.com", Kind: "sse"},
			{Name: "tts", Path: "/v1/tts", Upstream: "https://tts.example.com", Kind: "raw"},
		},
	}

	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/chat/completions", "chat"},
		{"/v1/tts/speech", "tts"},
		{"/v1/unknown", ""},
		{"/other/path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			route := cfg.GetRouteByPath(tt.path)
			if tt.expected == "" {
				if route != nil {
					t.Errorf("expected nil route, got %s", route.Name)
				}
			} else {
				if route == nil {
					t.Errorf("expected route %s, got nil", tt.expected)
				} else if route.Name != tt.expected {
					t.Errorf("expected route %s, got %s", tt.expected, route.Name)
				}
			}
		})
	}
}

func TestRouteConfig_GetAuthValue(t *testing.T) {
	// Set up test env var
	os.Setenv("TEST_AUTH_KEY", "sk-test-12345")
	defer os.Unsetenv("TEST_AUTH_KEY")

	route := &RouteConfig{
		AuthEnv: "TEST_AUTH_KEY",
	}

	value := route.GetAuthValue()
	if value != "sk-test-12345" {
		t.Errorf("expected 'sk-test-12345', got '%s'", value)
	}

	// Test with empty AuthEnv
	route2 := &RouteConfig{
		AuthEnv: "",
	}
	value2 := route2.GetAuthValue()
	if value2 != "" {
		t.Errorf("expected empty string, got '%s'", value2)
	}

	// Test with nonexistent env var
	route3 := &RouteConfig{
		AuthEnv: "NONEXISTENT_VAR",
	}
	value3 := route3.GetAuthValue()
	if value3 != "" {
		t.Errorf("expected empty string for nonexistent var, got '%s'", value3)
	}
}
