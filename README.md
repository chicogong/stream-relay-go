# stream-relay-go

A Go-based streaming relay/gateway for LLM and TTS APIs. It transparently proxies SSE and raw byte streams while adding production-grade observability and policy controls.

## 核心特性

- **流式转发** - 低延迟透传 SSE 和二进制流
- **完整存储** - ClickHouse 存储完整请求/响应（可回溯）
- **实时观测** - Prometheus 指标 + TTFT/TTFA 统计
- **简单限流** - 基于 Token Bucket 的租户限流
- **零解析成本** - 不做实时解析，存储完整数据后离线分析

## 设计理念

与传统 API Gateway 不同，stream-relay-go 的设计理念是：

1. **数据优先于实时指标** - 存储完整数据，未来可回溯任何问题
2. **同步写入优于异步队列** - 简单可靠，失败立刻可知
3. **ClickHouse 优于 Prometheus** - 复杂分析用数据库，不用内存指标
4. **实用主义** - 只做确实需要的功能，不做"可能需要"的

## 快速开始

### 1. 环境要求

- Go 1.21+
- Docker & Docker Compose（用于运行 Redis 和 ClickHouse）

### 2. 启动依赖服务

```bash
make docker-up
```

这会启动：
- Redis (localhost:6379)
- ClickHouse (localhost:9000)

### 3. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env，填入你的 API keys
```

### 4. 运行 Relay

```bash
make dev
```

服务将在 `http://localhost:8080` 启动。

### 5. 测试

```bash
# OpenAI Chat Completions
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-relay-test-key-123" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: test-tenant" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

### 6. 查看指标

访问 `http://localhost:8080/metrics` 查看 Prometheus 指标。

## 项目结构

```
stream-relay-go/
├── cmd/relay/          # 入口程序
├── internal/           # 核心逻辑（5 个文件）
│   ├── config.go       # 配置加载
│   ├── models.go       # 数据模型
│   ├── proxy.go        # 转发核心（200 行）
│   ├── server.go       # HTTP 服务器
│   ├── storage.go      # Redis + ClickHouse
│   ├── metrics.go      # Prometheus（5 个指标）
│   └── limiter.go      # 限流
├── configs/            # 配置文件
└── deployments/docker/ # Docker Compose
```

**极简设计 - 核心只有 5 个 Go 文件！**

## 配置说明

参考 `configs/config.yaml`：

```yaml
server:
  port: 8080
  timeout: 300s

routes:
  - name: openai
    path: /v1/chat/completions
    upstream: https://api.openai.com
    auth_env: OPENAI_API_KEY
    kind: sse

storage:
  redis:
    addr: localhost:6379
  clickhouse:
    addr: localhost:9000
    database: relay

rate_limit:
  default: 100  # 每分钟 100 请求
```

## 数据查询

所有数据都在 ClickHouse 的 `stream_logs` 表中，可以直接查询：

```sql
-- 查询 TTFT P95（最近 10 分钟）
SELECT
    route,
    quantile(0.95)(ttft_ms) as p95_ttft
FROM stream_logs
WHERE created_at > now() - INTERVAL 10 MINUTE
  AND ttft_ms IS NOT NULL
GROUP BY route;

-- 查询 token 使用量（按租户）
SELECT
    tenant_id,
    sum(tokens_in) as total_in,
    sum(tokens_out) as total_out
FROM stream_logs
WHERE created_at > today()
GROUP BY tenant_id;

-- 查看完整的请求/响应（调试用）
SELECT
    request_id,
    request_body,
    response_chunks
FROM stream_logs
WHERE request_id = 'xxx';
```

## Prometheus 指标

只提供 5 个核心指标（聚焦）：

1. `relay_requests_total{route, status}` - 请求总量
2. `relay_duration_ms{route}` - 延迟分布
3. `relay_errors_total{route, type}` - 错误统计
4. `relay_active_connections{route}` - 活跃连接
5. `relay_storage_write_ms` - 存储写入延迟

其他指标（TTFT/TPS/tokens）直接从 ClickHouse 查询！

## 与参考方案的差异

| 方面 | 参考方案 | 本方案 |
|------|---------|--------|
| 数据存储 | 只存元数据 | 存完整请求/响应 |
| 写入方式 | 异步队列 | 同步写入 |
| Token 统计 | 3 层优先级 | 只从响应提取 |
| Prometheus | 15+ 指标 | 5 个核心指标 |
| 复杂度 | 20+ 文件 | 5 个核心文件 |
| MVP 时间 | 6-8 周 | 2-3 周 |

**核心思想：简单 > 完美，实用 > 全面**

## 实施路线图

### Week 1: 核心转发 ✅
- [x] 配置系统
- [x] SSE/RAW 转发
- [x] ClickHouse 存储

### Week 2: 观测 + 限流
- [ ] Prometheus 集成
- [ ] 限流机制
- [ ] Grafana Dashboard

### Week 3: 生产就绪
- [ ] 性能优化（目标 QPS > 500）
- [ ] 错误处理完善
- [ ] 部署文档

## 常见问题

### Q: 为什么存储完整响应？不是很浪费空间吗？

A: ClickHouse 压缩比很高（10:1），1M 请求约 10GB。相比"解析失败无法回溯"的风险，这点成本完全值得。

### Q: 同步写入会不会影响性能？

A: ClickHouse 单次写入通常 10-50ms，相比 LLM 生成时间（几秒到几十秒）可忽略。而且失败立刻可知，比异步队列悄悄丢数据好得多。

### Q: 为什么不做实时 token 统计？

A: 因为解析复杂（每个 provider 格式不同），容易失败。存储完整数据后，可以随时重新解析，甚至用脚本批量修正。

### Q: 后续会加熔断/灰度/本地tokenizer吗？

A: 会，但要等生产环境跑一段时间，确认真的需要。不根据"可能需要"来设计。

## 贡献

欢迎提 Issue 和 PR！

## License

MIT
