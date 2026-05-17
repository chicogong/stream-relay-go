package internal

import (
	"context"
	"testing"
	"time"
)

// TestNewStorage_EmptyConfig 验证空配置下 NewStorage 不会 panic，
// 且 ClickHouse 未启用时连接为 nil。
func TestNewStorage_EmptyConfig(t *testing.T) {
	s, err := NewStorage(&StorageConfig{})
	if err != nil {
		t.Fatalf("NewStorage returned error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil Storage")
	}
	if s.clickhouse != nil {
		t.Error("expected clickhouse to be nil when Addr is empty")
	}
}

// TestStorage_PingClose 验证 nil/空 Storage 上的 Ping 与 Close 是安全的。
func TestStorage_PingClose(t *testing.T) {
	tests := []struct {
		name    string
		storage *Storage
	}{
		{"nil storage", nil},
		{"empty storage", &Storage{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.storage.Ping(context.Background()); err != nil {
				t.Errorf("Ping returned error: %v", err)
			}
			if err := tt.storage.Close(); err != nil {
				t.Errorf("Close returned error: %v", err)
			}
		})
	}
}

// TestNewStorage_PingCloseAfterEmptyConfig 验证空配置创建的 Storage 可正常 Ping/Close。
func TestNewStorage_PingCloseAfterEmptyConfig(t *testing.T) {
	s, err := NewStorage(&StorageConfig{})
	if err != nil {
		t.Fatalf("NewStorage returned error: %v", err)
	}
	if err := s.Ping(context.Background()); err != nil {
		t.Errorf("Ping returned error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// TestSaveLog_NoClickHouse 验证未启用 ClickHouse 时 SaveLog 退化为日志输出且不报错。
func TestSaveLog_NoClickHouse(t *testing.T) {
	s := &Storage{}
	log := &StreamLog{
		RequestID:  "req-1",
		Route:      "openai",
		StatusCode: 200,
		DurationMs: 123,
		BytesOut:   456,
	}
	if err := s.SaveLog(context.Background(), log); err != nil {
		t.Errorf("SaveLog returned error: %v", err)
	}
}

// TestStreamLogToRow 验证 StreamLog 到 ClickHouse 行的映射，
// 列顺序与 insertSQL 一致，可空字段与切片正确处理。
func TestStreamLogToRow(t *testing.T) {
	ttft := int64(50)
	tokensIn := int64(10)
	created := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		log  *StreamLog
		want []interface{}
	}{
		{
			name: "full row with nullable values",
			log: &StreamLog{
				RequestID:      "req-1",
				TenantID:       "tenant-a",
				CreatedAt:      created,
				Route:          "openai",
				Provider:       "openai",
				Model:          "gpt-4",
				Kind:           "sse",
				RequestBody:    `{"k":"v"}`,
				StatusCode:     200,
				ResponseChunks: []string{"chunk-1", "chunk-2"},
				DurationMs:     123,
				TTFTMs:         &ttft,
				BytesIn:        100,
				BytesOut:       200,
				ChunksCount:    2,
				TokensIn:       &tokensIn,
				ErrorType:      "",
				ErrorMessage:   "",
			},
			want: []interface{}{
				"req-1", "tenant-a", created,
				"openai", "openai", "gpt-4", "sse",
				`{"k":"v"}`,
				int16(200), []string{"chunk-1", "chunk-2"},
				int64(123), &ttft, (*int64)(nil), int64(100), int64(200), int32(2),
				&tokensIn, (*int64)(nil),
				"", "",
			},
		},
		{
			name: "nil response chunks become empty slice",
			log: &StreamLog{
				RequestID:      "req-2",
				CreatedAt:      created,
				StatusCode:     500,
				ResponseChunks: nil,
				ErrorType:      "upstream_error",
				ErrorMessage:   "boom",
			},
			want: []interface{}{
				"req-2", "", created,
				"", "", "", "",
				"",
				int16(500), []string{},
				int64(0), (*int64)(nil), (*int64)(nil), int64(0), int64(0), int32(0),
				(*int64)(nil), (*int64)(nil),
				"upstream_error", "boom",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := streamLogToRow(tt.log)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d columns, got %d", len(tt.want), len(got))
			}
			for i := range tt.want {
				if !rowValueEqual(got[i], tt.want[i]) {
					t.Errorf("column %d: expected %#v, got %#v", i, tt.want[i], got[i])
				}
			}
		})
	}
}

// rowValueEqual 比较两个行值，特殊处理 *int64 与 []string。
func rowValueEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case *int64:
		bv, ok := b.(*int64)
		if !ok {
			return false
		}
		if av == nil || bv == nil {
			return av == bv
		}
		return *av == *bv
	case []string:
		bv, ok := b.([]string)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
