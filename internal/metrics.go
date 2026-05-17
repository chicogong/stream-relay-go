package internal

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics Prometheus 指标
type Metrics struct {
	requestsTotal     *prometheus.CounterVec
	durationMs        *prometheus.HistogramVec
	errorsTotal       *prometheus.CounterVec
	activeConnections *prometheus.GaugeVec
	tokensTotal       *prometheus.CounterVec
	storageWriteMs    prometheus.Histogram
}

// NewMetrics 创建指标
func NewMetrics() *Metrics {
	return &Metrics{
		// 1. 请求总量
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "relay_requests_total",
				Help: "Total number of requests",
			},
			[]string{"route", "status"},
		),

		// 2. 延迟分布
		durationMs: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "relay_duration_ms",
				Help:    "Request duration in milliseconds",
				Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 30000, 60000},
			},
			[]string{"route"},
		),

		// 3. 错误计数
		errorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "relay_errors_total",
				Help: "Total number of errors",
			},
			[]string{"route", "type"},
		),

		// 4. 活跃连接
		activeConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "relay_active_connections",
				Help: "Number of active connections",
			},
			[]string{"route"},
		),

		// 5. Token 使用量
		tokensTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "relay_tokens_total",
				Help: "Total number of tokens processed",
			},
			[]string{"route", "direction"},
		),

		// 6. 存储写入延迟
		storageWriteMs: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "relay_storage_write_ms",
				Help:    "Storage write duration in milliseconds",
				Buckets: []float64{1, 5, 10, 50, 100, 500, 1000},
			},
		),
	}
}

// RecordRequest 记录已完成请求的指标
func (m *Metrics) RecordRequest(log *StreamLog) {
	route := log.Route

	m.requestsTotal.WithLabelValues(route, statusClass(log.StatusCode)).Inc()
	m.durationMs.WithLabelValues(route).Observe(float64(log.DurationMs))

	if log.ErrorType != "" {
		m.errorsTotal.WithLabelValues(route, log.ErrorType).Inc()
	}
	if log.TokensIn != nil {
		m.tokensTotal.WithLabelValues(route, "input").Add(float64(*log.TokensIn))
	}
	if log.TokensOut != nil {
		m.tokensTotal.WithLabelValues(route, "output").Add(float64(*log.TokensOut))
	}
}

// IncActiveConnections 活跃连接 +1
func (m *Metrics) IncActiveConnections(route string) {
	m.activeConnections.WithLabelValues(route).Inc()
}

// DecActiveConnections 活跃连接 -1
func (m *Metrics) DecActiveConnections(route string) {
	m.activeConnections.WithLabelValues(route).Dec()
}

// RecordStorageWrite 记录存储写入延迟
func (m *Metrics) RecordStorageWrite(d time.Duration) {
	m.storageWriteMs.Observe(float64(d.Milliseconds()))
}

// RecordStorageError 记录存储错误
func (m *Metrics) RecordStorageError() {
	m.errorsTotal.WithLabelValues("storage", "write_failed").Inc()
}

// statusClass 将 HTTP 状态码归类为 2xx/4xx/5xx
// 状态码为 0（上游未返回响应）归为 5xx
func statusClass(code int) string {
	switch {
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 || code == 0:
		return "5xx"
	default:
		return "2xx"
	}
}
