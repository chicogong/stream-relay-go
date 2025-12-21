.PHONY: build run test clean docker-up docker-down

# 变量
BINARY_NAME=relay
CONFIG_PATH=configs/config.yaml

# 构建
build:
	go build -o bin/$(BINARY_NAME) cmd/relay/main.go

# 运行
run: build
	./bin/$(BINARY_NAME) -config $(CONFIG_PATH)

# 测试
test:
	go test -v ./...

# 清理
clean:
	rm -rf bin/
	go clean

# 格式化
fmt:
	go fmt ./...

# 代码检查
lint:
	golangci-lint run

# Docker 启动依赖
docker-up:
	docker-compose -f deployments/docker/docker-compose.yml up -d

# Docker 停止
docker-down:
	docker-compose -f deployments/docker/docker-compose.yml down

# 初始化 ClickHouse 表
init-db:
	@echo "ClickHouse 表会在首次运行时自动创建"

# 开发环境（启动依赖 + 运行）
dev: docker-up
	@sleep 3  # 等待服务启动
	$(MAKE) run

# 性能测试
bench:
	@echo "TODO: 添加性能测试"

# 帮助
help:
	@echo "可用命令："
	@echo "  make build      - 编译二进制"
	@echo "  make run        - 运行服务"
	@echo "  make test       - 运行测试"
	@echo "  make clean      - 清理构建"
	@echo "  make docker-up  - 启动依赖服务（Redis + ClickHouse）"
	@echo "  make docker-down- 停止依赖服务"
	@echo "  make dev        - 启动开发环境"
