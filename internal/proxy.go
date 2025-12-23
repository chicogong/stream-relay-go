package internal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Proxy 核心转发器 - 只做一件事：转发流并收集元数据
type Proxy struct {
	config  *Config
	storage *Storage
	metrics *Metrics
	client  *http.Client
}

// NewProxy 创建代理
func NewProxy(config *Config, storage *Storage, metrics *Metrics) *Proxy {
	return &Proxy{
		config:  config,
		storage: storage,
		metrics: metrics,
		client: &http.Client{
			Timeout: config.Server.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Handle 处理请求 - 核心逻辑
func (p *Proxy) Handle(w http.ResponseWriter, r *http.Request) error {
	// 1. 创建请求上下文
	ctx := &RequestContext{
		RequestID: uuid.New().String(),
		TenantID:  r.Header.Get("X-Tenant-ID"),
		StartTime: time.Now(),
	}

	// 2. 路由匹配
	route := p.config.GetRouteByPath(r.URL.Path)
	if route == nil {
		return fmt.Errorf("route not found for path: %s", r.URL.Path)
	}
	ctx.Route = route

	// 3. 读取请求体（需要重放给上游）
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	ctx.BytesIn = int64(len(requestBody))
	r.Body.Close()

	// 4. 构造上游请求
	upstreamReq, err := p.buildUpstreamRequest(r, route, requestBody)
	if err != nil {
		return fmt.Errorf("build upstream request: %w", err)
	}

	// 5. 发起请求
	upstreamResp, err := p.client.Do(upstreamReq)
	if err != nil {
		ctx.ErrorType = "upstream_error"
		ctx.ErrorMessage = err.Error()
		p.saveLog(ctx, string(requestBody))
		return fmt.Errorf("upstream request: %w", err)
	}
	defer upstreamResp.Body.Close()

	ctx.StatusCode = upstreamResp.StatusCode

	// 6. 复制响应头
	for k, v := range upstreamResp.Header {
		w.Header()[k] = v
	}
	w.Header().Set("X-Request-ID", ctx.RequestID)
	w.WriteHeader(upstreamResp.StatusCode)

	// 7. 流式转发（根据 kind）
	if route.Kind == "sse" {
		err = p.forwardSSE(w, upstreamResp.Body, ctx)
	} else {
		err = p.forwardRaw(w, upstreamResp.Body, ctx)
	}

	// 8. 存储日志（同步）
	p.saveLog(ctx, string(requestBody))

	return err
}

// forwardSSE 转发 SSE 流
func (p *Proxy) forwardSSE(w http.ResponseWriter, body io.Reader, ctx *RequestContext) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer does not support flushing")
	}

	scanner := bufio.NewScanner(body)
	firstToken := true
	chunks := []string{}

	for scanner.Scan() {
		line := scanner.Text()

		// 记录 TTFT
		if firstToken && strings.HasPrefix(line, "data:") && !strings.Contains(line, "[DONE]") {
			ttft := time.Since(ctx.StartTime).Milliseconds()
			ctx.TTFTMs = &ttft
			firstToken = false
		}

		// 收集 chunks（完整存储）
		chunks = append(chunks, line)

		// 写入并立刻 flush
		fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()

		ctx.BytesOut += int64(len(line) + 1)
		ctx.ChunksCount++
	}

	ctx.ResponseChunks = chunks

	if err := scanner.Err(); err != nil {
		ctx.ErrorType = "stream_error"
		ctx.ErrorMessage = err.Error()
		return err
	}

	return nil
}

// forwardRaw 转发原始二进制流
func (p *Proxy) forwardRaw(w http.ResponseWriter, body io.Reader, ctx *RequestContext) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer does not support flushing")
	}

	buf := make([]byte, 32*1024) // 32KB buffer
	firstChunk := true

	for {
		n, err := body.Read(buf)
		if n > 0 {
			// 记录 TTFA
			if firstChunk {
				ttfa := time.Since(ctx.StartTime).Milliseconds()
				ctx.TTFAMs = &ttfa
				firstChunk = false
			}

			// 写入并 flush
			//nolint:errcheck // streaming write errors are handled by connection close
			w.Write(buf[:n])
			flusher.Flush()

			ctx.BytesOut += int64(n)
			ctx.ChunksCount++

			// RAW 模式不存储完整响应（太大），只存元数据
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			ctx.ErrorType = "stream_error"
			ctx.ErrorMessage = err.Error()
			return err
		}
	}

	return nil
}

// buildUpstreamRequest 构造上游请求
func (p *Proxy) buildUpstreamRequest(r *http.Request, route *RouteConfig, body []byte) (*http.Request, error) {
	// 构造完整 URL
	upstreamURL := route.Upstream + r.URL.Path
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	// 创建请求
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 复制 headers（除了 Authorization）
	for k, v := range r.Header {
		if k != "Authorization" {
			req.Header[k] = v
		}
	}

	// 注入上游认证
	if route.AuthHeader != "" && route.AuthEnv != "" {
		authValue := route.GetAuthValue()
		if authValue != "" {
			// 自动添加 Bearer 前缀（如果是 Authorization header 且还没有前缀）
			if route.AuthHeader == "Authorization" && !strings.HasPrefix(authValue, "Bearer ") {
				authValue = "Bearer " + authValue
			}
			req.Header.Set(route.AuthHeader, authValue)
		}
	}

	return req, nil
}

// saveLog 保存日志（同步）
func (p *Proxy) saveLog(ctx *RequestContext, requestBody string) {
	log := ctx.ToStreamLog(requestBody)

	// 同步写入存储（简单可靠）
	if p.storage != nil {
		if err := p.storage.SaveLog(context.Background(), log); err != nil {
			p.metrics.RecordStorageError()
		}
	}

	// 更新 Prometheus 指标
	p.metrics.RecordRequest(ctx)
}
