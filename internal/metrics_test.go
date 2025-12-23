package internal

import (
	"sync"
	"testing"
)

var (
	testMetrics     *Metrics
	testMetricsOnce sync.Once
)

// getTestMetrics returns a singleton metrics instance for tests
// This avoids duplicate prometheus registration errors
func getTestMetrics() *Metrics {
	testMetricsOnce.Do(func() {
		testMetrics = NewMetrics()
	})
	return testMetrics
}

func TestNewMetrics(t *testing.T) {
	m := getTestMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	if m.requestsTotal == nil {
		t.Error("requestsTotal should be initialized")
	}
	if m.durationMs == nil {
		t.Error("durationMs should be initialized")
	}
	if m.errorsTotal == nil {
		t.Error("errorsTotal should be initialized")
	}
	if m.activeConnections == nil {
		t.Error("activeConnections should be initialized")
	}
	if m.storageWriteMs == nil {
		t.Error("storageWriteMs should be initialized")
	}
}

func TestMetrics_RecordStorageError(t *testing.T) {
	m := getTestMetrics()

	// Should not panic
	m.RecordStorageError()
}
