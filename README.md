# Stream Relay Go

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

English | [简体中文](README_zh.md)

A lightweight, high-performance streaming relay for LLM and TTS APIs with built-in observability.

## 🎬 Demo

![Streaming Demo](docs/images/streaming-demo.gif)

*Real-time token-by-token streaming with sub-100ms latency*

## ✨ Features

- **🚀 Low-latency Streaming** - Token-by-token SSE streaming with immediate flush
- **🔐 Auto Authentication** - Automatic Bearer token injection for upstream APIs
- **📊 Real-time Monitoring** - Prometheus metrics + beautiful Grafana dashboards
- **🎯 Multi-provider Support** - SiliconFlow, OpenAI, Anthropic, Azure TTS
- **⚡ Zero Dependency** - Optional Redis/ClickHouse, works standalone
- **🛡️ Production Ready** - Rate limiting, health checks, graceful shutdown

## 🏗️ Architecture

```
Client Request
     ↓
API Key Auth
     ↓
Rate Limiting
     ↓
Upstream Auth Injection
     ↓
SSE Streaming Proxy ← → Upstream API
     ↓
Metrics Collection
     ↓
Client Response
```

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- (Optional) Docker for monitoring stack

### Installation

```bash
# Clone the repository
git clone https://github.com/chicogong/stream-relay-go.git
cd stream-relay-go

# Build
make build

# Run
./bin/relay -config configs/config.yaml
```

### Configuration

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Add your API keys to `.env`:
```bash
SILICONFLOW_API_KEY=sk-your-key-here
OPENAI_API_KEY=sk-your-key-here
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

3. Start the relay:
```bash
make dev
```

The relay will start on `http://localhost:8080`

### Testing

```bash
# Health check
curl http://localhost:8080/healthz

# Streaming request
curl -N http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer sk-relay-test-key-123' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen/Qwen2.5-7B-Instruct",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true,
    "max_tokens": 20
  }'
```

## 📊 Monitoring

### Start Grafana + Prometheus

```bash
cd deployments/grafana
docker-compose up -d
```

### Access Dashboards

- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Metrics Endpoint**: http://localhost:8080/metrics

### Beautiful Dashboard

![Grafana Dashboard](docs/images/grafana-dashboard.png)

The dashboard provides real-time insights:
- 📊 **Total Requests** - Cumulative request count
- ✅ **Success Rate** - Real-time success percentage (color-coded: 🟢 >99%, 🟡 >95%, 🟠 >90%, 🔴 <90%)
- 📈 **Request Rate** - Requests per minute with smooth curves
- ⏱️ **Response Time** - p50/p95/p99 latency percentiles
- 🔥 **Heatmap** - Visual latency distribution
- 🚨 **Error Monitoring** - Instant error detection with alerts

### 🚀 Enhanced Dashboard with Logs

![Enhanced Grafana Dashboard](docs/images/enhanced-grafana-dashboard.png)

The enhanced dashboard (`enhanced-dashboard.json`) includes **15 comprehensive panels** with integrated log viewing:

**Metrics Panels:**
- 📊 Total Requests, Success Rate, Avg Response Time
- 🔗 Active Connections, Error Count, Storage Latency
- 📈 Request Rate Trend & Response Time Percentiles (p50/p95/p99)
- 🎯 Requests by Route (donut chart)
- 📊 Status Code Distribution (2xx/4xx/5xx bar gauge)
- 🚨 Error Types Table & Active Connections Over Time
- 🔥 Request Latency Heatmap
- 📋 Recent Activity Log Table

**Log Integration (Loki):**
- 📝 Live application logs with filtering
- 🔍 Search logs by level (ERROR, INFO, DEBUG)
- 📊 Unified metrics + logs view for faster debugging

**Setup:**
The enhanced monitoring stack includes Loki + Promtail for log aggregation. See [deployments/grafana/README.md](deployments/grafana/README.md) for full setup instructions.

### Generate Demo Traffic

```bash
# Run the test script to generate sample requests
./test_relay.sh

# Or manually send requests
for i in {1..10}; do
  curl -N http://localhost:8080/v1/chat/completions \
    -H 'Authorization: Bearer sk-relay-test-key-123' \
    -H 'Content-Type: application/json' \
    -d "{\"model\": \"Qwen/Qwen2.5-7B-Instruct\", \"messages\": [{\"role\": \"user\", \"content\": \"Count to $i\"}], \"stream\": true, \"max_tokens\": 20}"
done
```

Watch the metrics update in real-time at http://localhost:3000

> 💡 **Tip**: Use `./scripts/generate-demo.sh` to populate the dashboard with demo traffic!
>
> 📸 For creating your own screenshots and demos, see [docs/DEMO.md](docs/DEMO.md)

## 📁 Project Structure

```
stream-relay-go/
├── cmd/relay/          # Application entry point
├── internal/           # Core implementation
│   ├── config.go       # Configuration management
│   ├── proxy.go        # Streaming proxy logic
│   ├── server.go       # HTTP server setup
│   ├── metrics.go      # Prometheus metrics
│   ├── limiter.go      # Rate limiting
│   └── storage.go      # Optional storage layer
├── configs/            # Configuration files
├── deployments/        # Docker & Grafana configs
└── docs/              # Documentation
```

## ⚙️ Configuration

### Server

```yaml
server:
  port: 8080
  timeout: 300s
  max_body_size: 10485760  # 10MB
```

### Routes

```yaml
routes:
  - name: siliconflow
    path: /v1/chat/completions
    upstream: https://api.siliconflow.cn
    auth_header: Authorization
    auth_env: SILICONFLOW_API_KEY
    kind: sse
```

### Rate Limiting

```yaml
rate_limit:
  enabled: true
  default: 100  # requests per minute per tenant
  burst: 20
```

## 🔧 Advanced Usage

### Custom Routes

Add custom routes in `configs/config.yaml`:

```yaml
routes:
  - name: custom-provider
    path: /custom/path
    upstream: https://api.custom.com
    auth_header: X-API-Key
    auth_env: CUSTOM_API_KEY
    kind: sse
```

### Storage Backend

Enable optional storage for detailed logging:

```yaml
storage:
  redis:
    addr: localhost:6379
    password: ""
    db: 0
```

## 📈 Metrics

The relay exposes comprehensive Prometheus metrics at `/metrics` endpoint:

### Core Metrics

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `relay_requests_total` | Counter | Total number of requests processed | `route`, `status` (2xx/4xx/5xx) |
| `relay_duration_ms` | Histogram | Request duration in milliseconds | `route` |
| `relay_errors_total` | Counter | Total number of errors | `route`, `type` |
| `relay_active_connections` | Gauge | Current number of active connections | `route` |
| `relay_storage_write_ms` | Histogram | Storage write latency in milliseconds | - |

### Histogram Buckets

- **Duration Buckets**: 100ms, 500ms, 1s, 2s, 5s, 10s, 30s, 60s
- **Storage Write Buckets**: 1ms, 5ms, 10ms, 50ms, 100ms, 500ms, 1s

### Example Queries

```promql
# Request rate (requests per minute)
rate(relay_requests_total[1m]) * 60

# Average latency
rate(relay_duration_ms_sum[1m]) / rate(relay_duration_ms_count[1m])

# P95 latency
histogram_quantile(0.95, rate(relay_duration_ms_bucket[1m]))

# Success rate
sum(relay_requests_total{status="2xx"}) / sum(relay_requests_total) * 100

# Error rate
rate(relay_errors_total[1m])

# Active connections by route
relay_active_connections
```

### Grafana Dashboard

Import `deployments/grafana/beautiful-dashboard.json` for a pre-configured dashboard with:
- Real-time request rate
- Latency percentiles (p50, p95, p99)
- Success rate gauge
- Error monitoring
- Request heatmap
- Recent activity table

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Gin](https://github.com/gin-gonic/gin)
- Monitoring powered by [Prometheus](https://prometheus.io) and [Grafana](https://grafana.com)
- Inspired by best practices in API gateway design

## 📮 Support

- 🐛 [Report Bug](https://github.com/chicogong/stream-relay-go/issues)
- 💡 [Request Feature](https://github.com/chicogong/stream-relay-go/issues)
- 📧 Email: your-email@example.com

---

**Made with ❤️ for the LLM community**
