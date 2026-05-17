package internal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrRequestTooLarge 请求体超过 max_body_size 限制
var ErrRequestTooLarge = errors.New("request body too large")

// maxSSELineSize SSE 单行最大字节数（默认 bufio.Scanner 上限仅 64KB，
// 大 JSON chunk 会触发 ErrTooLong，这里放宽到 1MB）
const maxSSELineSize = 1024 * 1024

const (
	// maxUpstreamRetries 上游连接错误的默认最大重试次数
	maxUpstreamRetries = 2
	// retryBaseDelay 重试退避基准（第 n 次重试等待 n*base）
	retryBaseDelay = 100 * time.Millisecond
)

// Proxy 核心转发器 - 只做一件事：转发流并收集元数据
type Proxy struct {
	config     *Config
	storage    *Storage
	metrics    *Metrics
	client     *http.Client
	maxRetries int
}

// NewProxy 创建代理
func NewProxy(config *Config, storage *Storage, metrics *Metrics) *Proxy {
	return &Proxy{
		config:     config,
		storage:    storage,
		metrics:    metrics,
		maxRetries: maxUpstreamRetries,
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

// Handle 处理请求 - 核心逻辑。tenantID 由调用方从已鉴权的 API Key 派生，
// 不可由客户端请求头伪造。
func (p *Proxy) Handle(w http.ResponseWriter, r *http.Request, tenantID string) error {
	// 1. 创建请求上下文
	ctx := &RequestContext{
		RequestID: uuid.New().String(),
		TenantID:  tenantID,
		StartTime: time.Now(),
	}

	// 2. 路由匹配
	route := p.config.GetRouteByPath(r.URL.Path)
	if route == nil {
		return fmt.Errorf("route not found for path: %s", r.URL.Path)
	}
	ctx.Route = route

	// 活跃连接计数（按路由）
	p.metrics.IncActiveConnections(route.Name)
	defer p.metrics.DecActiveConnections(route.Name)

	// 3. 读取请求体（需要重放给上游）
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		r.Body.Close()
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			ctx.ErrorType = "request_too_large"
			ctx.ErrorMessage = err.Error()
			p.saveLog(ctx, "")
			return ErrRequestTooLarge
		}
		return fmt.Errorf("read request body: %w", err)
	}
	ctx.BytesIn = int64(len(requestBody))
	r.Body.Close()

	// 4. 发起上游请求（对连接错误做有限重试）
	upstreamResp, err := p.doWithRetry(r, route, requestBody)
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

// doWithRetry 发起上游请求，对连接错误做有限重试。
// 仅在尚未收到任何响应时重试，因此对幂等的流式请求是安全的；
// 客户端取消（context 结束）则立即放弃。
func (p *Proxy) doWithRetry(r *http.Request, route *RouteConfig, body []byte) (*http.Response, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		req, err := p.buildUpstreamRequest(r, route, body)
		if err != nil {
			return nil, err
		}

		resp, err := p.client.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// 重试次数耗尽或客户端已取消则放弃
		if attempt >= p.maxRetries || r.Context().Err() != nil {
			return nil, lastErr
		}

		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-time.After(retryBaseDelay * time.Duration(attempt+1)):
		}
	}
}

// forwardSSE 转发 SSE 流
func (p *Proxy) forwardSSE(w http.ResponseWriter, body io.Reader, ctx *RequestContext) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("response writer does not support flushing")
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSELineSize)
	firstToken := true

	// detailed_logs 开启时保留完整响应用于存储；关闭时仅做增量统计，
	// 内存占用与响应长度无关（token 用量仍可增量提取）
	collectAll := p.config.Observability.DetailedLogs
	var chunks []string
	var usage Usage
	usageFound := false

	for scanner.Scan() {
		line := scanner.Text()

		// 记录 TTFT
		if firstToken && strings.HasPrefix(line, "data:") && !strings.Contains(line, "[DONE]") {
			ttft := time.Since(ctx.StartTime).Milliseconds()
			ctx.TTFTMs = &ttft
			firstToken = false
		}

		// 增量提取 token 用量
		if mergeUsageLine(&usage, line) {
			usageFound = true
		}

		// 仅在需要完整存储时收集 chunks
		if collectAll {
			chunks = append(chunks, line)
		}

		// 写入并立刻 flush
		fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()

		ctx.BytesOut += int64(len(line) + 1)
		ctx.ChunksCount++
	}

	if collectAll {
		ctx.ResponseChunks = chunks
	}
	if usageFound {
		finalizeUsage(&usage)
		ctx.Usage = &usage
	}

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

// saveLog 保存日志并更新指标（同步）
func (p *Proxy) saveLog(ctx *RequestContext, requestBody string) {
	log := ctx.ToStreamLog(requestBody)

	// 同步写入存储（简单可靠），并记录写入延迟
	if p.storage != nil {
		start := time.Now()
		err := p.storage.SaveLog(context.Background(), log)
		p.metrics.RecordStorageWrite(time.Since(start))
		if err != nil {
			p.metrics.RecordStorageError()
		}
	}

	// 更新 Prometheus 指标
	p.metrics.RecordRequest(log)
}
