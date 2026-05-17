package internal

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

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
		Model:          extractModel(requestBody),
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

// extractProvider 从 upstream URL 提取 provider 名称
// 例如: https://api.openai.com -> openai
func extractProvider(upstream string) string {
	u, err := url.Parse(upstream)
	if err != nil || u.Hostname() == "" {
		return "unknown"
	}
	host := strings.ToLower(u.Hostname())

	// 已知 provider 的关键字匹配
	for keyword, name := range map[string]string{
		"openai":      "openai",
		"anthropic":   "anthropic",
		"siliconflow": "siliconflow",
		"azure":       "azure",
		"microsoft":   "azure",
		"googleapis":  "google",
		"deepseek":    "deepseek",
	} {
		if strings.Contains(host, keyword) {
			return name
		}
	}

	// 回退：取二级域名（api.example.com -> example）
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return host
}

// extractModel 从请求体 JSON 中提取 model 字段
func extractModel(requestBody string) string {
	if requestBody == "" {
		return ""
	}
	var obj struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal([]byte(requestBody), &obj); err != nil {
		return ""
	}
	return obj.Model
}

// usagePayload 兼容 OpenAI 与 Anthropic 两种 usage 结构
type usagePayload struct {
	// OpenAI 风格
	PromptTokens     *int64 `json:"prompt_tokens"`
	CompletionTokens *int64 `json:"completion_tokens"`
	TotalTokens      *int64 `json:"total_tokens"`
	// Anthropic 风格
	InputTokens  *int64 `json:"input_tokens"`
	OutputTokens *int64 `json:"output_tokens"`
}

// extractUsage 从 SSE 响应 chunks 中提取 token 使用量
// 支持 OpenAI（prompt_tokens/completion_tokens，需 stream_options.include_usage）
// 与 Anthropic（input_tokens/output_tokens，分散在 message_start / message_delta 事件）
func extractUsage(chunks []string) *Usage {
	var usage Usage
	found := false

	apply := func(u *usagePayload) {
		if u == nil {
			return
		}
		if u.PromptTokens != nil {
			usage.InputTokens, found = *u.PromptTokens, true
		}
		if u.InputTokens != nil {
			usage.InputTokens, found = *u.InputTokens, true
		}
		if u.CompletionTokens != nil {
			usage.OutputTokens, found = *u.CompletionTokens, true
		}
		if u.OutputTokens != nil {
			usage.OutputTokens, found = *u.OutputTokens, true
		}
		if u.TotalTokens != nil {
			usage.TotalTokens, found = *u.TotalTokens, true
		}
	}

	for _, line := range chunks {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var obj struct {
			Usage *usagePayload `json:"usage"`
			// Anthropic message_start 把 usage 嵌在 message 下
			Message *struct {
				Usage *usagePayload `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(payload), &obj); err != nil {
			continue
		}
		apply(obj.Usage)
		if obj.Message != nil {
			apply(obj.Message.Usage)
		}
	}

	if !found {
		return nil
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	return &usage
}
