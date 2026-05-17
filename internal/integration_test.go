package internal

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEndToEnd_FromConfig wires the full stack the way cmd/relay does —
// LoadConfig -> Storage -> Proxy -> RateLimiter -> Server — and verifies a
// request flows through to a real upstream with credential injection, while
// an unauthenticated request is rejected.
func TestEndToEnd_FromConfig(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer upstream-secret" {
			t.Errorf("upstream Authorization = %q, want injected credential", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\ndata: [DONE]\n"))
	}))
	defer upstream.Close()

	t.Setenv("E2E_UPSTREAM_KEY", "upstream-secret")

	cfgYAML := `
server:
  port: 8080
routes:
  - name: chat
    path: /v1/chat
    upstream: ` + upstream.URL + `
    auth_header: Authorization
    auth_env: E2E_UPSTREAM_KEY
    kind: sse
rate_limit:
  enabled: true
auth:
  api_keys:
    - sk-e2e-client
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(cfgYAML), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	// applyDefaults should have filled the omitted fields.
	if cfg.Server.MaxBodySize == 0 || cfg.Server.Timeout == 0 {
		t.Errorf("defaults not applied: %+v", cfg.Server)
	}

	storage, err := NewStorage(&cfg.Storage)
	if err != nil {
		t.Fatalf("NewStorage: %v", err)
	}
	defer func() { _ = storage.Close() }()

	proxy := NewProxy(cfg, storage, getTestMetrics())
	limiter := NewRateLimiter(&cfg.RateLimit)
	defer limiter.Stop()
	srv := NewServer(cfg, proxy, limiter, storage)

	// Authenticated request flows through to the upstream.
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m"}`))
	req.Header.Set("Authorization", "Bearer sk-e2e-client")
	rec := httptest.NewRecorder()
	srv.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Errorf("response missing streamed payload: %q", rec.Body.String())
	}

	// Unauthenticated request is rejected before reaching the upstream.
	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	rec2 := httptest.NewRecorder()
	srv.engine.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated status = %d, want 401", rec2.Code)
	}
}
