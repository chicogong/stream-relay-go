package internal

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// newTestServer wires a Server -> Proxy -> fake upstream for HTTP-level tests.
func newTestServer(t *testing.T, route RouteConfig, auth AuthConfig, rl RateLimitConfig) *Server {
	t.Helper()
	cfg := &Config{
		Server: ServerConfig{
			Port:        8080,
			Timeout:     5 * time.Second,
			MaxBodySize: 1 << 20,
		},
		Routes:    []RouteConfig{route},
		RateLimit: rl,
		Auth:      auth,
	}
	proxy := NewProxy(cfg, nil, getTestMetrics())
	limiter := NewRateLimiter(&cfg.RateLimit)
	return NewServer(cfg, proxy, limiter, nil)
}

// fakeSSEUpstream returns an httptest server emitting a small SSE stream.
func fakeSSEUpstream() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		for _, line := range []string{"data: hi", "data: [DONE]"} {
			io.WriteString(w, line+"\n")
			fl.Flush()
		}
	}))
}

// TestServer_HandleProxy_Auth covers the bearer-token auth paths.
func TestServer_HandleProxy_Auth(t *testing.T) {
	upstream := fakeSSEUpstream()
	defer upstream.Close()

	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL, Kind: "sse"}
	auth := AuthConfig{APIKeys: []string{"sk-valid-key"}}
	rl := RateLimitConfig{Enabled: false}

	tests := []struct {
		name       string
		authHeader string
		setHeader  bool
		wantStatus int
	}{
		{"valid bearer key", "Bearer sk-valid-key", true, http.StatusOK},
		{"wrong key", "Bearer sk-wrong-key", true, http.StatusUnauthorized},
		{"missing authorization", "", false, http.StatusUnauthorized},
		{"empty authorization", "", true, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t, route, auth, rl)

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
			if tt.setHeader {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			srv.engine.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				if !strings.Contains(rec.Body.String(), "data: hi") {
					t.Errorf("expected forwarded SSE body, got %q", rec.Body.String())
				}
			}
		})
	}
}

// TestServer_HandleProxy_RateLimit verifies that a burst beyond the configured
// limit returns 429.
func TestServer_HandleProxy_RateLimit(t *testing.T) {
	upstream := fakeSSEUpstream()
	defer upstream.Close()

	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL, Kind: "sse"}
	auth := AuthConfig{APIKeys: []string{"sk-valid-key"}}
	// Tiny limit: burst of 2, ~negligible refill within the test window.
	rl := RateLimitConfig{Enabled: true, Default: 1, Burst: 2}

	srv := newTestServer(t, route, auth, rl)

	var okCount, limitedCount int
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer sk-valid-key")
		rec := httptest.NewRecorder()
		srv.engine.ServeHTTP(rec, req)

		switch rec.Code {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			limitedCount++
		default:
			t.Errorf("unexpected status %d", rec.Code)
		}
	}

	if limitedCount == 0 {
		t.Error("expected at least one 429 response from rate limiting")
	}
	if okCount == 0 {
		t.Error("expected at least one 200 response within the burst")
	}
	if okCount > 4 {
		t.Errorf("expected ~burst (2) successful requests, got %d", okCount)
	}
}

// TestServer_HandleProxy_RouteNotFound verifies an unmatched path yields 502.
func TestServer_HandleProxy_RouteNotFound(t *testing.T) {
	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: "http://example.invalid", Kind: "sse"}
	auth := AuthConfig{APIKeys: []string{"sk-valid-key"}}
	srv := newTestServer(t, route, auth, RateLimitConfig{Enabled: false})

	req := httptest.NewRequest(http.MethodPost, "/no/such/path", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer sk-valid-key")
	rec := httptest.NewRecorder()
	srv.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "route not found") {
		t.Errorf("body = %q, want 'route not found'", rec.Body.String())
	}
}

// TestServer_HandleProxy_MaxBodySize verifies a request body larger than
// Server.MaxBodySize returns 413. handleProxy wraps the body with
// http.MaxBytesReader and proxy.Handle maps the resulting MaxBytesError to
// ErrRequestTooLarge, which the server reports as 413.
func TestServer_HandleProxy_MaxBodySize(t *testing.T) {
	upstream := fakeSSEUpstream()
	defer upstream.Close()

	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL, Kind: "sse"}
	auth := AuthConfig{APIKeys: []string{"sk-valid-key"}}
	srv := newTestServer(t, route, auth, RateLimitConfig{Enabled: false})

	// Body comfortably larger than the 1 MiB MaxBodySize.
	big := bytes.Repeat([]byte("x"), (1<<20)+1024)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(big))
	req.Header.Set("Authorization", "Bearer sk-valid-key")
	rec := httptest.NewRecorder()
	srv.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestServer_HandleProxy_SmallBodyAllowed confirms a body within the limit
// is accepted (control for the 413 test).
func TestServer_HandleProxy_SmallBodyAllowed(t *testing.T) {
	upstream := fakeSSEUpstream()
	defer upstream.Close()

	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL, Kind: "sse"}
	auth := AuthConfig{APIKeys: []string{"sk-valid-key"}}
	srv := newTestServer(t, route, auth, RateLimitConfig{Enabled: false})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewReader(bytes.Repeat([]byte("x"), 1024)))
	req.Header.Set("Authorization", "Bearer sk-valid-key")
	rec := httptest.NewRecorder()
	srv.engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestServer_Health covers the health and readiness endpoints.
func TestServer_Health(t *testing.T) {
	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: "http://up", Kind: "sse"}
	srv := newTestServer(t, route, AuthConfig{}, RateLimitConfig{})

	for _, path := range []string{"/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			srv.engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("%s status = %d, want 200", path, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "status") {
				t.Errorf("%s body missing 'status': %q", path, rec.Body.String())
			}
		})
	}
}

// TestServer_RequestIDHeader verifies the X-Request-ID header is propagated
// through the full server -> proxy path on a successful proxied request.
func TestServer_RequestIDHeader(t *testing.T) {
	upstream := fakeSSEUpstream()
	defer upstream.Close()

	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL, Kind: "sse"}
	auth := AuthConfig{APIKeys: []string{"sk-valid-key"}}
	srv := newTestServer(t, route, auth, RateLimitConfig{Enabled: false})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer sk-valid-key")
	rec := httptest.NewRecorder()
	srv.engine.ServeHTTP(rec, req)

	if rid := rec.Header().Get("X-Request-ID"); rid == "" {
		t.Error("X-Request-ID response header not set")
	}
}

// TestServer_Authenticate exercises the authenticate helper directly.
func TestServer_Authenticate(t *testing.T) {
	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: "http://up", Kind: "sse"}
	srv := newTestServer(t, route, AuthConfig{APIKeys: []string{"key-a", "key-b"}}, RateLimitConfig{})

	tests := []struct {
		name   string
		header string
		set    bool
		want   bool
	}{
		{"valid first key", "Bearer key-a", true, true},
		{"valid second key", "Bearer key-b", true, true},
		{"bare token without bearer prefix", "key-a", true, true},
		{"wrong key", "Bearer nope", true, false},
		{"no header", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/x", nil)
			if tt.set {
				req.Header.Set("Authorization", tt.header)
			}
			c.Request = req
			if got := srv.authenticate(c); got != tt.want {
				t.Errorf("authenticate() = %v, want %v", got, tt.want)
			}
		})
	}
}
