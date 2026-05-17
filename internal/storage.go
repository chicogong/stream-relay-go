package internal

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/redis/go-redis/v9"
)

// Storage 存储层 - Redis + ClickHouse
type Storage struct {
	redis      *redis.Client
	clickhouse clickhouse.Conn
	config     *StorageConfig
}

// NewStorage 创建存储（Redis 和 ClickHouse 均为可选，初始化失败不影响核心转发功能）
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

	// 初始化 ClickHouse（可选，失败不影响核心功能）
	if config.ClickHouse.Addr != "" {
		s.initClickHouse()
	}

	return s, nil
}

// initClickHouse 初始化 ClickHouse 连接（任何步骤失败都只警告并将连接置空）
func (s *Storage) initClickHouse() {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{s.config.ClickHouse.Addr},
		Auth: clickhouse.Auth{
			Database: s.config.ClickHouse.Database,
			Username: s.config.ClickHouse.Username,
			Password: s.config.ClickHouse.Password,
		},
	})
	if err != nil {
		fmt.Printf("WARNING: ClickHouse connection failed: %v\n", err)
		return
	}

	if err := conn.Ping(context.Background()); err != nil {
		fmt.Printf("WARNING: ClickHouse ping failed: %v\n", err)
		_ = conn.Close()
		return
	}

	s.clickhouse = conn

	if err := s.ensureTables(context.Background()); err != nil {
		fmt.Printf("WARNING: ClickHouse ensureTables failed: %v\n", err)
		_ = conn.Close()
		s.clickhouse = nil
		return
	}

	fmt.Println("INFO: ClickHouse connected")
}

// createTableSQL 建表语句（列定义与 StreamLog 字段一一对应）
const createTableSQL = `
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

// insertSQL 插入语句（列顺序与 streamLogToRow 返回值一致）
const insertSQL = `INSERT INTO stream_logs (
	request_id, tenant_id, created_at,
	route, provider, model, kind,
	request_body,
	status_code, response_chunks,
	duration_ms, ttft_ms, ttfa_ms, bytes_in, bytes_out, chunks_count,
	tokens_in, tokens_out,
	error_type, error_message
)`

// ensureTables 创建表（如果不存在）
func (s *Storage) ensureTables(ctx context.Context) error {
	return s.clickhouse.Exec(ctx, createTableSQL)
}

// streamLogToRow 将 StreamLog 映射为 ClickHouse INSERT 的参数切片
// 列顺序必须与 insertSQL 保持一致。可空字段保留 *int64 以写入 NULL。
func streamLogToRow(log *StreamLog) []interface{} {
	chunks := log.ResponseChunks
	if chunks == nil {
		chunks = []string{}
	}
	return []interface{}{
		log.RequestID,
		log.TenantID,
		log.CreatedAt,
		log.Route,
		log.Provider,
		log.Model,
		log.Kind,
		log.RequestBody,
		int16(log.StatusCode), //nolint:gosec // HTTP 状态码范围有限，不会溢出 int16
		chunks,
		log.DurationMs,
		log.TTFTMs,
		log.TTFAMs,
		log.BytesIn,
		log.BytesOut,
		int32(log.ChunksCount), //nolint:gosec // chunk 计数远小于 int32 上限
		log.TokensIn,
		log.TokensOut,
		log.ErrorType,
		log.ErrorMessage,
	}
}

// SaveLog 保存日志：ClickHouse 可用时写入数据库，否则打印摘要日志
func (s *Storage) SaveLog(ctx context.Context, log *StreamLog) error {
	if s.clickhouse == nil {
		fmt.Printf("SESSION: request_id=%s route=%s status=%d duration=%dms bytes_out=%d\n",
			log.RequestID, log.Route, log.StatusCode, log.DurationMs, log.BytesOut)
		return nil
	}

	if s.config != nil && s.config.ClickHouse.Async {
		// 异步插入：拼接为单条 INSERT VALUES 语句，由 ClickHouse 异步落盘
		return s.clickhouse.AsyncInsert(ctx, insertSQL+" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			false, streamLogToRow(log)...)
	}

	batch, err := s.clickhouse.PrepareBatch(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	if err := batch.Append(streamLogToRow(log)...); err != nil {
		return fmt.Errorf("append row: %w", err)
	}
	return batch.Send()
}

// Ping 检查存储连接（ClickHouse 优先，未启用时返回 nil）
func (s *Storage) Ping(ctx context.Context) error {
	if s == nil || s.clickhouse == nil {
		return nil
	}
	return s.clickhouse.Ping(ctx)
}

// Ping 检查存储依赖是否可达（供 /readyz 使用）
func (s *Storage) Ping(ctx context.Context) error {
	if s == nil || s.redis == nil {
		return nil
	}
	return s.redis.Ping(ctx).Err()
}

// Close 关闭连接
func (s *Storage) Close() error {
	if s == nil {
		return nil
	}
	if s.redis != nil {
		if err := s.redis.Close(); err != nil {
			return err
		}
	}
	if s.clickhouse != nil {
		if err := s.clickhouse.Close(); err != nil {
			return err
		}
	}
	return nil
}
