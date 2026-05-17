# Stream Relay Go

[![CI](https://github.com/chicogong/stream-relay-go/actions/workflows/ci.yml/badge.svg)](https://github.com/chicogong/stream-relay-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/chicogong/stream-relay-go)](https://goreportcard.com/report/github.com/chicogong/stream-relay-go)
[![codecov](https://codecov.io/gh/chicogong/stream-relay-go/branch/master/graph/badge.svg)](https://codecov.io/gh/chicogong/stream-relay-go)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[English](README.md) | 简体中文

一个轻量级、高性能的 LLM 和 TTS API 流式代理网关，内置生产级可观测性。

## ✨ 特性

- **🚀 低延迟流式传输** - 逐 token 的 SSE 流式传输，即时刷新
- **🔐 自动认证** - 自动为上游 API 注入 Bearer token
- **📊 实时监控** - Prometheus 指标 + 精美的 Grafana 仪表板
- **🎯 多服务商支持** - SiliconFlow、OpenAI、Anthropic、Azure TTS
- **⚡ 零依赖** - Redis/ClickHouse 可选，可独立运行
- **🛡️ 生产就绪** - 限流、健康检查、优雅关闭

## 🏗️ 架构

```
客户端请求
     ↓
API Key 认证
     ↓
流量限制
     ↓
上游认证注入
     ↓
SSE 流式代理 ← → 上游 API
     ↓
指标收集
     ↓
客户端响应
```

## 🚀 快速开始

### 前置要求

- Go 1.23+
- （可选）Docker 用于监控栈

### 安装

```bash
# 克隆仓库
git clone https://github.com/chicogong/stream-relay-go.git
cd stream-relay-go

# 构建
make build

# 运行
./bin/relay -config configs/config.yaml
```

### 配置

1. 复制示例环境文件：
```bash
cp .env.example .env
```

2. 将你的 API 密钥添加到 `.env`：
```bash
SILICONFLOW_API_KEY=sk-your-key-here
OPENAI_API_KEY=sk-your-key-here
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

3. 启动代理：
```bash
make dev
```

代理将在 `http://localhost:8080` 启动

### 测试

```bash
# 健康检查
curl http://localhost:8080/healthz

# 流式请求
curl -N http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer sk-relay-test-key-123' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen/Qwen2.5-7B-Instruct",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true,
    "max_tokens": 20
  }'
```

## 📊 监控

### 启动 Grafana + Prometheus

```bash
cd deployments/grafana
docker-compose up -d
```

### 访问仪表板

- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **指标端点**: http://localhost:8080/metrics

### 精美仪表板

![Grafana 仪表板](docs/images/grafana-dashboard.png)

仪表板提供实时洞察：
- 📊 **总请求数** - 累计请求计数
- ✅ **成功率** - 实时成功百分比（颜色编码：🟢 >99%、🟡 >95%、🟠 >90%、🔴 <90%）
- 📈 **请求速率** - 每分钟请求数，带平滑曲线
- ⏱️ **响应时间** - p50/p95/p99 延迟百分位数
- 🔥 **热力图** - 可视化延迟分布
- 🚨 **错误监控** - 即时错误检测与告警

### 🚀 增强版仪表板（含日志）

![增强版 Grafana 仪表板](docs/images/enhanced-grafana-dashboard.png)

增强版仪表板（`enhanced-dashboard.json`）包含 **15 个综合面板**，集成日志查看功能：

**指标面板：**
- 📊 总请求数、成功率、平均响应时间
- 🔗 活跃连接数、错误计数、存储延迟
- 📈 请求速率趋势 & 响应时间百分位（p50/p95/p99）
- 🎯 按路由分布请求（环形图）
- 📊 状态码分布（2xx/4xx/5xx 柱状图）
- 🚨 错误类型表格 & 活跃连接时序图
- 🔥 请求延迟热力图
- 📋 最近活动日志表格

**日志集成（Loki）：**
- 📝 实时应用日志，支持过滤
- 🔍 按日志级别搜索（ERROR、INFO、DEBUG）
- 📊 统一的指标 + 日志视图，加速问题排查

**设置说明：**
增强版监控栈包含 Loki + Promtail 用于日志聚合。完整设置说明请参见 [deployments/grafana/README.md](deployments/grafana/README.md)

### 生成演示流量

```bash
# 运行测试脚本生成示例请求
./scripts/generate-demo.sh

# 或手动发送请求
for i in {1..10}; do
  curl -N http://localhost:8080/v1/chat/completions \
    -H 'Authorization: Bearer sk-relay-test-key-123' \
    -H 'Content-Type: application/json' \
    -d "{\"model\": \"Qwen/Qwen2.5-7B-Instruct\", \"messages\": [{\"role\": \"user\", \"content\": \"数到 $i\"}], \"stream\": true, \"max_tokens\": 20}"
done
```

在 http://localhost:3000 实时观察指标更新

> 💡 **提示**：使用 `./scripts/generate-demo.sh` 生成演示流量填充仪表板！

## 📁 项目结构

```
stream-relay-go/
├── cmd/relay/          # 应用程序入口
├── internal/           # 核心实现
│   ├── config.go       # 配置管理
│   ├── proxy.go        # 流式代理逻辑
│   ├── server.go       # HTTP 服务器设置
│   ├── metrics.go      # Prometheus 指标
│   ├── limiter.go      # 限流
│   └── storage.go      # 可选存储层
├── configs/            # 配置文件
├── deployments/        # Docker 和 Grafana 配置
└── docs/              # 文档
```

## ⚙️ 配置

### 服务器

```yaml
server:
  port: 8080
  timeout: 300s
  max_body_size: 10485760  # 10MB
```

### 路由

```yaml
routes:
  - name: siliconflow
    path: /v1/chat/completions
    upstream: https://api.siliconflow.cn
    auth_header: Authorization
    auth_env: SILICONFLOW_API_KEY
    kind: sse
```

### 限流

```yaml
rate_limit:
  enabled: true
  default: 100  # 每租户每分钟请求数
  burst: 20
```

## 🔧 高级用法

### 自定义路由

在 `configs/config.yaml` 中添加自定义路由：

```yaml
routes:
  - name: custom-provider
    path: /custom/path
    upstream: https://api.custom.com
    auth_header: X-API-Key
    auth_env: CUSTOM_API_KEY
    kind: sse
```

### 存储后端

启用可选存储用于详细日志记录：

```yaml
storage:
  redis:
    addr: localhost:6379
    password: ""
    db: 0
```

## 📈 指标

代理在 `/metrics` 端点暴露全面的 Prometheus 指标：

### 核心指标

| 指标名称 | 类型 | 描述 | 标签 |
|---------|------|------|------|
| `relay_requests_total` | Counter | 已处理的请求总数 | `route`、`status` (2xx/4xx/5xx) |
| `relay_duration_ms` | Histogram | 请求持续时间（毫秒） | `route` |
| `relay_errors_total` | Counter | 错误总数 | `route`、`type` |
| `relay_active_connections` | Gauge | 当前活跃连接数 | `route` |
| `relay_tokens_total` | Counter | 处理的 token 总数（从流式 usage 中提取） | `route`、`direction`（input/output） |
| `relay_storage_write_ms` | Histogram | 存储写入延迟（毫秒） | - |

### 直方图桶

- **持续时间桶**：100ms、500ms、1s、2s、5s、10s、30s、60s
- **存储写入桶**：1ms、5ms、10ms、50ms、100ms、500ms、1s

### 示例查询

```promql
# 请求速率（每分钟请求数）
rate(relay_requests_total[1m]) * 60

# 平均延迟
rate(relay_duration_ms_sum[1m]) / rate(relay_duration_ms_count[1m])

# P95 延迟
histogram_quantile(0.95, rate(relay_duration_ms_bucket[1m]))

# 成功率
sum(relay_requests_total{status="2xx"}) / sum(relay_requests_total) * 100

# 错误率
rate(relay_errors_total[1m])

# 按路由的活跃连接数
relay_active_connections

# Token 吞吐量（每分钟 token 数，按方向区分）
rate(relay_tokens_total[1m]) * 60
```

### Grafana 仪表板

导入 `deployments/grafana/beautiful-dashboard.json` 获取预配置的仪表板，包含：
- 实时请求速率
- 延迟百分位数（p50、p95、p99）
- 成功率仪表盘
- 错误监控
- 请求热力图
- 最近活动表

## 🤝 贡献

欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

## 📝 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

- 使用 [Gin](https://github.com/gin-gonic/gin) 构建
- 监控由 [Prometheus](https://prometheus.io) 和 [Grafana](https://grafana.com) 提供支持
- 灵感来自 API 网关设计最佳实践

## 📮 支持

- 🐛 [报告 Bug](https://github.com/chicogong/stream-relay-go/issues)
- 💡 [请求功能](https://github.com/chicogong/stream-relay-go/issues)
- 📧 邮箱：your-email@example.com

---

**用 ❤️ 为 LLM 社区打造**
