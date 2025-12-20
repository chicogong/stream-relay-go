package internal

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics Prometheus 指标 - 只保留核心的 5 个
type Metrics struct {
	requestsTotal      *prometheus.CounterVec
	durationMs         *prometheus.HistogramVec
	errorsTotal        *prometheus.CounterVec
	activeConnections  *prometheus.GaugeVec
	storageWriteMs     prometheus.Histogram
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

		// 5. 存储写入延迟
		storageWriteMs: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "relay_storage_write_ms",
				Help:    "Storage write duration in milliseconds",
				Buckets: []float64{1, 5, 10, 50, 100, 500, 1000},
			},
		),
	}
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(ctx *RequestContext) {
	route := ctx.Route.Name
	status := "2xx"
	if ctx.StatusCode >= 400 && ctx.StatusCode < 500 {
		status = "4xx"
	} else if ctx.StatusCode >= 500 {
		status = "5xx"
	}

	m.requestsTotal.WithLabelValues(route, status).Inc()
	m.durationMs.WithLabelValues(route).Observe(float64(ctx.ToStreamLog("").DurationMs))

	if ctx.ErrorType != "" {
		m.errorsTotal.WithLabelValues(route, ctx.ErrorType).Inc()
	}
}

// RecordStorageError 记录存储错误
func (m *Metrics) RecordStorageError() {
	m.errorsTotal.WithLabelValues("storage", "write_failed").Inc()
}
