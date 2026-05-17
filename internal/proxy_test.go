package internal

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// httpGet issues a context-aware GET (satisfies the noctx linter).
func httpGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	return resp
}

// newTestProxy builds a Proxy wired to the given routes with nil storage
// (Proxy.saveLog tolerates nil storage) and the shared test metrics.
func newTestProxy(routes []RouteConfig) *Proxy {
	cfg := &Config{
		Server: ServerConfig{
			Timeout:     5 * time.Second,
			MaxBodySize: 1 << 20,
		},
		Routes: routes,
	}
	return NewProxy(cfg, nil, getTestMetrics())
}

// TestProxy_ForwardSSE verifies SSE streaming: data lines + [DONE] are
// forwarded verbatim, TTFT is captured, chunks are counted and the
// X-Request-ID response header is set.
func TestProxy_ForwardSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Error("upstream writer does not support flushing")
			return
		}
		for _, line := range []string{
			"data: {\"delta\":\"hello\"}",
			"data: {\"delta\":\"world\"}",
			"data: [DONE]",
		} {
			if _, err := io.WriteString(w, line+"\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}))
	defer upstream.Close()

	proxy := newTestProxy([]RouteConfig{
		{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL, Kind: "sse"},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"q":1}`))
	rec := httptest.NewRecorder()

	if err := proxy.Handle(rec, req); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if rid := res.Header.Get("X-Request-ID"); rid == "" {
		t.Error("X-Request-ID header not set")
	}

	body, _ := io.ReadAll(res.Body)
	bs := string(body)
	for _, want := range []string{`data: {"delta":"hello"}`, `data: {"delta":"world"}`, "data: [DONE]"} {
		if !strings.Contains(bs, want) {
			t.Errorf("response body missing %q; got %q", want, bs)
		}
	}
}

// TestProxy_SSEContext checks the RequestContext populated during SSE
// streaming via the metadata that Handle records.
func TestProxy_SSEContext(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		// brief delay so TTFT is measurably > 0
		time.Sleep(2 * time.Millisecond)
		for _, line := range []string{"data: a", "data: b", "data: [DONE]"} {
			io.WriteString(w, line+"\n")
			fl.Flush()
		}
	}))
	defer upstream.Close()

	route := RouteConfig{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL, Kind: "sse"}
	proxy := newTestProxy([]RouteConfig{route})

	// Exercise forwardSSE directly so we can inspect the context.
	ctx := &RequestContext{Route: &route, StartTime: time.Now()}
	resp := httpGet(t, upstream.URL+"/v1/chat")
	defer resp.Body.Close()

	rec := httptest.NewRecorder()
	if err := proxy.forwardSSE(rec, resp.Body, ctx); err != nil {
		t.Fatalf("forwardSSE: %v", err)
	}

	if ctx.TTFTMs == nil {
		t.Fatal("TTFTMs should be captured")
	}
	if *ctx.TTFTMs < 0 {
		t.Errorf("TTFTMs = %d, want >= 0", *ctx.TTFTMs)
	}
	// 3 lines: data:a, data:b, data:[DONE]
	if ctx.ChunksCount != 3 {
		t.Errorf("ChunksCount = %d, want 3", ctx.ChunksCount)
	}
	if len(ctx.ResponseChunks) != 3 {
		t.Errorf("len(ResponseChunks) = %d, want 3", len(ctx.ResponseChunks))
	}
	if ctx.BytesOut == 0 {
		t.Error("BytesOut should be > 0")
	}
}

// TestProxy_ForwardRaw verifies raw (binary) passthrough and TTFA capture.
func TestProxy_ForwardRaw(t *testing.T) {
	payload := []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x10, 0x20}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		if _, err := w.Write(payload); err != nil {
			return
		}
	}))
	defer upstream.Close()

	proxy := newTestProxy([]RouteConfig{
		{Name: "tts", Path: "/v1/tts", Upstream: upstream.URL, Kind: "raw"},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/tts/speech", strings.NewReader("text"))
	rec := httptest.NewRecorder()

	if err := proxy.Handle(rec, req); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	res := rec.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	if string(body) != string(payload) {
		t.Errorf("raw body = %v, want %v", body, payload)
	}
	if res.Header.Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header not set")
	}
}

// TestProxy_ForwardRawTTFA verifies TTFA is captured for raw streams.
func TestProxy_ForwardRawTTFA(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Millisecond)
		w.Write([]byte("audio-bytes"))
	}))
	defer upstream.Close()

	route := RouteConfig{Name: "tts", Path: "/v1/tts", Upstream: upstream.URL, Kind: "raw"}
	proxy := newTestProxy([]RouteConfig{route})

	resp := httpGet(t, upstream.URL+"/v1/tts")
	defer resp.Body.Close()

	ctx := &RequestContext{Route: &route, StartTime: time.Now()}
	rec := httptest.NewRecorder()
	if err := proxy.forwardRaw(rec, resp.Body, ctx); err != nil {
		t.Fatalf("forwardRaw: %v", err)
	}

	if ctx.TTFAMs == nil {
		t.Fatal("TTFAMs should be captured")
	}
	if *ctx.TTFAMs < 0 {
		t.Errorf("TTFAMs = %d, want >= 0", *ctx.TTFAMs)
	}
	if ctx.ChunksCount == 0 {
		t.Error("ChunksCount should be > 0")
	}
	if ctx.BytesOut != int64(len("audio-bytes")) {
		t.Errorf("BytesOut = %d, want %d", ctx.BytesOut, len("audio-bytes"))
	}
}

// TestProxy_RouteNotFound verifies an unmatched path returns an error,
// which the server layer maps to 502.
func TestProxy_RouteNotFound(t *testing.T) {
	proxy := newTestProxy([]RouteConfig{
		{Name: "chat", Path: "/v1/chat", Upstream: "http://example.invalid", Kind: "sse"},
	})

	req := httptest.NewRequest(http.MethodPost, "/no/such/route", strings.NewReader("x"))
	rec := httptest.NewRecorder()

	err := proxy.Handle(rec, req)
	if err == nil {
		t.Fatal("expected error for unmatched route")
	}
	if !strings.Contains(err.Error(), "route not found") {
		t.Errorf("error = %q, want 'route not found'", err.Error())
	}
}

// TestProxy_UpstreamError verifies a connection failure is surfaced as an error.
func TestProxy_UpstreamError(t *testing.T) {
	proxy := newTestProxy([]RouteConfig{
		// Reserved TEST-NET-1 address that should not be reachable.
		{Name: "chat", Path: "/v1/chat", Upstream: "http://192.0.2.1:1", Kind: "sse"},
	})
	proxy.client.Timeout = 200 * time.Millisecond

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/x", strings.NewReader("x"))
	rec := httptest.NewRecorder()

	err := proxy.Handle(rec, req)
	if err == nil {
		t.Fatal("expected error for unreachable upstream")
	}
	if !strings.Contains(err.Error(), "upstream request") {
		t.Errorf("error = %q, want 'upstream request'", err.Error())
	}
}

// TestProxy_BuildUpstreamRequest covers upstream auth injection and inbound
// Authorization stripping in buildUpstreamRequest.
func TestProxy_BuildUpstreamRequest(t *testing.T) {
	tests := []struct {
		name          string
		route         RouteConfig
		envKey        string
		envVal        string
		inboundAuth   string
		wantHeader    string // header name to inspect
		wantHeaderVal string
	}{
		{
			name:          "authorization gets bearer prefix",
			route:         RouteConfig{Name: "r", Path: "/p", Upstream: "http://up", AuthHeader: "Authorization", AuthEnv: "UP_KEY_A", Kind: "sse"},
			envKey:        "UP_KEY_A",
			envVal:        "sk-secret",
			wantHeader:    "Authorization",
			wantHeaderVal: "Bearer sk-secret",
		},
		{
			name:          "authorization already prefixed not doubled",
			route:         RouteConfig{Name: "r", Path: "/p", Upstream: "http://up", AuthHeader: "Authorization", AuthEnv: "UP_KEY_B", Kind: "sse"},
			envKey:        "UP_KEY_B",
			envVal:        "Bearer sk-already",
			wantHeader:    "Authorization",
			wantHeaderVal: "Bearer sk-already",
		},
		{
			name:          "custom header no bearer prefix",
			route:         RouteConfig{Name: "r", Path: "/p", Upstream: "http://up", AuthHeader: "X-Api-Key", AuthEnv: "UP_KEY_C", Kind: "raw"},
			envKey:        "UP_KEY_C",
			envVal:        "raw-key",
			wantHeader:    "X-Api-Key",
			wantHeaderVal: "raw-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			proxy := newTestProxy([]RouteConfig{tt.route})

			r := httptest.NewRequest(http.MethodPost, tt.route.Path, strings.NewReader("body"))
			// inbound client Authorization that must be stripped
			r.Header.Set("Authorization", "Bearer client-token")
			r.Header.Set("X-Tenant-ID", "tenant-1")

			upReq, err := proxy.buildUpstreamRequest(r, &tt.route, []byte("body"))
			if err != nil {
				t.Fatalf("buildUpstreamRequest: %v", err)
			}

			if got := upReq.Header.Get(tt.wantHeader); got != tt.wantHeaderVal {
				t.Errorf("%s = %q, want %q", tt.wantHeader, got, tt.wantHeaderVal)
			}

			// Inbound Authorization must not leak through unless the route's
			// auth header happens to be Authorization (then it is overwritten).
			if tt.wantHeader != "Authorization" {
				if got := upReq.Header.Get("Authorization"); got == "Bearer client-token" {
					t.Error("inbound client Authorization was forwarded; should be stripped")
				}
			} else {
				if got := upReq.Header.Get("Authorization"); got == "Bearer client-token" {
					t.Error("inbound client Authorization leaked instead of upstream value")
				}
			}

			// Non-auth headers should be forwarded.
			if got := upReq.Header.Get("X-Tenant-ID"); got != "tenant-1" {
				t.Errorf("X-Tenant-ID = %q, want tenant-1", got)
			}
		})
	}
}

// TestProxy_BuildUpstreamRequest_StripsAuthWhenNoInjection verifies that when
// the route configures no upstream auth, the client's Authorization is still
// stripped and nothing replaces it.
func TestProxy_BuildUpstreamRequest_StripsAuthWhenNoInjection(t *testing.T) {
	route := RouteConfig{Name: "r", Path: "/p", Upstream: "http://up", Kind: "sse"}
	proxy := newTestProxy([]RouteConfig{route})

	r := httptest.NewRequest(http.MethodPost, "/p", strings.NewReader("b"))
	r.Header.Set("Authorization", "Bearer client-token")

	upReq, err := proxy.buildUpstreamRequest(r, &route, []byte("b"))
	if err != nil {
		t.Fatalf("buildUpstreamRequest: %v", err)
	}
	if got := upReq.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want empty (stripped)", got)
	}
}

// TestProxy_UpstreamReceivesInjectedAuth is an end-to-end check that the
// upstream actually observes the injected Bearer credential and never sees
// the client's token.
func TestProxy_UpstreamReceivesInjectedAuth(t *testing.T) {
	t.Setenv("UP_KEY_E2E", "upstream-secret")

	var gotAuth, gotURL string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "data: [DONE]\n")
	}))
	defer upstream.Close()

	proxy := newTestProxy([]RouteConfig{
		{Name: "chat", Path: "/v1/chat", Upstream: upstream.URL,
			AuthHeader: "Authorization", AuthEnv: "UP_KEY_E2E", Kind: "sse"},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?model=x", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer client-token")
	rec := httptest.NewRecorder()

	if err := proxy.Handle(rec, req); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if gotAuth != "Bearer upstream-secret" {
		t.Errorf("upstream Authorization = %q, want 'Bearer upstream-secret'", gotAuth)
	}
	if !strings.Contains(gotURL, "/v1/chat/completions") || !strings.Contains(gotURL, "model=x") {
		t.Errorf("upstream URL = %q, want path + query forwarded", gotURL)
	}
}
