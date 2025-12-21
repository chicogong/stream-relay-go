package internal

import "time"

// StreamLog 流式请求日志 - 存储到 ClickHouse 的完整记录
type StreamLog struct {
	// 基础信息
	RequestID string    `json:"request_id"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`

	// 路由信息
	Route    string `json:"route"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Kind     string `json:"kind"` // sse | raw

	// 请求（压缩存储）
	RequestBody string `json:"request_body"` // JSON string

	// 响应
	StatusCode     int      `json:"status_code"`
	ResponseChunks []string `json:"response_chunks"` // 完整的 SSE events 或 binary chunks

	// 元数据（边转边收集）
	DurationMs  int64  `json:"duration_ms"`
	TTFTMs      *int64 `json:"ttft_ms,omitempty"` // Time To First Token (LLM)
	TTFAMs      *int64 `json:"ttfa_ms,omitempty"` // Time To First Audio (TTS)
	BytesIn     int64  `json:"bytes_in"`
	BytesOut    int64  `json:"bytes_out"`
	ChunksCount int    `json:"chunks_count"`

	// Token（从响应提取，失败则为 null）
	TokensIn  *int64 `json:"tokens_in,omitempty"`
	TokensOut *int64 `json:"tokens_out,omitempty"`

	// 错误信息
	ErrorType    string `json:"error_type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// RequestContext 请求上下文 - 在处理过程中传递
type RequestContext struct {
	RequestID string
	TenantID  string
	Route     *RouteConfig
	StartTime time.Time

	// 收集的数据
	BytesIn        int64
	BytesOut       int64
	ChunksCount    int
	TTFTMs         *int64
	TTFAMs         *int64
	ResponseChunks []string
	StatusCode     int
	ErrorType      string
	ErrorMessage   string
}

// ToStreamLog 转换为 StreamLog
func (ctx *RequestContext) ToStreamLog(requestBody string) *StreamLog {
	duration := time.Since(ctx.StartTime).Milliseconds()

	log := &StreamLog{
		RequestID:      ctx.RequestID,
		TenantID:       ctx.TenantID,
		CreatedAt:      ctx.StartTime,
		Route:          ctx.Route.Name,
		Provider:       extractProvider(ctx.Route.Upstream),
		Kind:           ctx.Route.Kind,
		RequestBody:    requestBody,
		StatusCode:     ctx.StatusCode,
		ResponseChunks: ctx.ResponseChunks,
		DurationMs:     duration,
		TTFTMs:         ctx.TTFTMs,
		TTFAMs:         ctx.TTFAMs,
		BytesIn:        ctx.BytesIn,
		BytesOut:       ctx.BytesOut,
		ChunksCount:    ctx.ChunksCount,
		ErrorType:      ctx.ErrorType,
		ErrorMessage:   ctx.ErrorMessage,
	}

	// 尝试从最后一个 chunk 提取 token
	if ctx.Route.Kind == "sse" && len(ctx.ResponseChunks) > 0 {
		usage := extractUsage(ctx.ResponseChunks)
		if usage != nil {
			log.TokensIn = &usage.InputTokens
			log.TokensOut = &usage.OutputTokens
		}
	}

	return log
}

// Usage Token 使用量
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

// extractProvider 从 upstream URL 提取 provider
func extractProvider(upstream string) string {
	if len(upstream) > 0 {
		// 简单提取：api.openai.com -> openai
		parts := []string{}
		for _, part := range []string{"openai", "anthropic", "azure", "google"} {
			if len(upstream) > 0 && len(part) > 0 {
				parts = append(parts, part)
			}
		}
		// 这里简化处理，实际应该用正则或更复杂的逻辑
		return "unknown"
	}
	return "unknown"
}

// extractUsage 从响应中提取 usage（简化版）
func extractUsage(chunks []string) *Usage {
	// TODO: 实现 JSON 解析逻辑
	// 从最后几个 chunk 中查找包含 "usage" 的部分
	// OpenAI: { "usage": { "prompt_tokens": 10, "completion_tokens": 20 } }
	// Anthropic: { "usage": { "input_tokens": 10, "output_tokens": 20 } }
	return nil
}
