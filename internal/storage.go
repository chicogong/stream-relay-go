package internal

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/redis/go-redis/v9"
)

// Storage 存储层 - Redis + ClickHouse
type Storage struct {
	redis  *redis.Client
	ch     clickhouse.Conn
	config *StorageConfig
}

// NewStorage 创建存储（暂时只初始化 Redis，ClickHouse 可选）
func NewStorage(config *StorageConfig) (*Storage, error) {
	s := &Storage{
		config: config,
	}

	// 初始化 Redis（可选，失败不影响核心功能）
	s.redis = redis.NewClient(&redis.Options{
		Addr:     config.Redis.Addr,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	})

	// 测试连接（失败只警告）
	if err := s.redis.Ping(context.Background()).Err(); err != nil {
		fmt.Printf("WARNING: Redis connection failed: %v\n", err)
		s.redis = nil
	}

	// 暂时跳过 ClickHouse 初始化
	fmt.Println("INFO: ClickHouse disabled (not needed for core relay functionality)")

	return s, nil
}

// ensureTables 创建表（如果不存在）
func (s *Storage) ensureTables(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS stream_logs (
		request_id String,
		tenant_id String,
		created_at DateTime64(3),

		route String,
		provider String,
		model String,
		kind String,

		request_body String,

		status_code Int16,
		response_chunks Array(String),

		duration_ms Int64,
		ttft_ms Nullable(Int64),
		ttfa_ms Nullable(Int64),
		bytes_in Int64,
		bytes_out Int64,
		chunks_count Int32,

		tokens_in Nullable(Int64),
		tokens_out Nullable(Int64),

		error_type String,
		error_message String
	) ENGINE = MergeTree()
	PARTITION BY toYYYYMM(created_at)
	ORDER BY (created_at, request_id)
	SETTINGS index_granularity = 8192
	`

	return s.ch.Exec(ctx, createTableSQL)
}

// SaveLog 保存日志（暂时只输出到日志，不写 ClickHouse）
func (s *Storage) SaveLog(ctx context.Context, log *StreamLog) error {
	// 暂时只打印摘要日志，不写数据库
	fmt.Printf("SESSION: request_id=%s route=%s status=%d duration=%dms bytes_out=%d\n",
		log.RequestID, log.Route, log.StatusCode, log.DurationMs, log.BytesOut)

	// TODO: 当 ClickHouse 可用时，写入数据库
	return nil
}

// Close 关闭连接
func (s *Storage) Close() error {
	if s.redis != nil {
		if err := s.redis.Close(); err != nil {
			return err
		}
	}
	// ClickHouse 暂时禁用，不需要关闭
	return nil
}
