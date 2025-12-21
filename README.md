# Stream Relay Go

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

A lightweight, high-performance streaming relay for LLM and TTS APIs with built-in observability.

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

The Grafana dashboard includes:
- 📊 Total Requests
- ✅ Success Rate Gauge
- 📈 Request Rate Chart
- ⏱️ Response Time Distribution (p50/p95/p99)
- 🔥 Response Time Heatmap
- 🚨 Error Rate Monitoring

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

The relay exposes the following Prometheus metrics:

- `relay_requests_total` - Total number of requests
- `relay_duration_ms` - Request duration histogram
- `relay_errors_total` - Total number of errors
- `relay_active_connections` - Current active connections
- `relay_storage_write_ms` - Storage write latency

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
