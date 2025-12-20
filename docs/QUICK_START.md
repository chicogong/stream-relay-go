# 快速开始指南

## 第一次运行（5 分钟）

### 1. 修复依赖

```bash
# 初始化 Go modules
go mod tidy

# 添加缺少的包
go get github.com/google/uuid
```

### 2. 启动依赖服务

```bash
make docker-up
```

等待 Redis 和 ClickHouse 启动（约 10 秒）。

### 3. 配置环境变量

```bash
# 复制示例配置
cp .env.example .env

# 编辑 .env，至少配置一个 API key
# 比如：
echo "OPENAI_API_KEY=sk-your-key-here" >> .env
source .env
```

### 4. 构建并运行

```bash
make build
./bin/relay -config configs/config.yaml
```

你应该看到：
```
Starting server on :8080
```

### 5. 测试

新开一个终端：

```bash
# 测试健康检查
curl http://localhost:8080/healthz

# 测试 OpenAI 转发（流式）
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-relay-test-key-123" \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: test-tenant" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Say hello"}],
    "stream": true
  }'
```

### 6. 查看数据

```bash
# 查看 Prometheus 指标
curl http://localhost:8080/metrics | grep relay_

# 查询 ClickHouse
docker exec -it relay-clickhouse clickhouse-client

# 在 clickhouse-client 中执行：
USE relay;
SELECT count() FROM stream_logs;
SELECT request_id, route, duration_ms, ttft_ms FROM stream_logs LIMIT 5;
```

## 常见问题

### Q: go mod tidy 报错

确保你的 Go 版本 >= 1.21:
```bash
go version
```

### Q: ClickHouse 连接失败

检查容器是否启动：
```bash
docker ps | grep clickhouse
```

### Q: 请求返回 401

检查配置文件中的 `auth.api_keys`，确保请求的 Bearer token 匹配。

### Q: 转发到 OpenAI 失败

1. 检查环境变量是否设置：`echo $OPENAI_API_KEY`
2. 检查配置文件中的 `auth_env` 是否正确
3. 查看服务日志

## 下一步

- 查看 [README.md](../README.md) 了解完整功能
- 查看 [IMPLEMENTATION.md](./IMPLEMENTATION.md) 了解开发计划
- 开始第一个 Issue：[Week1-1] 基础框架完善
