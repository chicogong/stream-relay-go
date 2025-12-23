package internal

import (
	"sync"
	"testing"
)

func TestNewRateLimiter(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: true,
		Default: 60,
		Burst:   10,
	}

	rl := NewRateLimiter(config)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.limiters == nil {
		t.Error("limiters map should be initialized")
	}
	if rl.config != config {
		t.Error("config should be set")
	}
}

func TestRateLimiter_Allow_Disabled(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: false,
		Default: 1,
		Burst:   1,
	}

	rl := NewRateLimiter(config)

	// Should always allow when disabled
	for i := 0; i < 100; i++ {
		if !rl.Allow("tenant1") {
			t.Error("should always allow when rate limiting is disabled")
		}
	}
}

func TestRateLimiter_Allow_Enabled(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: true,
		Default: 60, // 60 per minute = 1 per second
		Burst:   5,
	}

	rl := NewRateLimiter(config)

	// First burst should be allowed
	allowedCount := 0
	for i := 0; i < 10; i++ {
		if rl.Allow("tenant1") {
			allowedCount++
		}
	}

	// Should allow approximately burst count
	if allowedCount < 4 || allowedCount > 6 {
		t.Errorf("expected ~5 allowed requests (burst), got %d", allowedCount)
	}
}

func TestRateLimiter_Allow_PerTenant(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: true,
		Default: 60,
		Burst:   3,
	}

	rl := NewRateLimiter(config)

	// Each tenant should have independent limits
	// Exhaust tenant1's burst
	for i := 0; i < 5; i++ {
		rl.Allow("tenant1")
	}

	// tenant2 should still have full burst available
	allowedCount := 0
	for i := 0; i < 5; i++ {
		if rl.Allow("tenant2") {
			allowedCount++
		}
	}

	if allowedCount < 2 {
		t.Errorf("tenant2 should have independent limit, got %d allowed", allowedCount)
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: true,
		Default: 600,
		Burst:   100,
	}

	rl := NewRateLimiter(config)

	var wg sync.WaitGroup
	var allowedCount int
	var mu sync.Mutex

	// Concurrent requests from multiple tenants
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(tenantID string) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if rl.Allow(tenantID) {
					mu.Lock()
					allowedCount++
					mu.Unlock()
				}
			}
		}("tenant-" + string(rune('a'+i)))
	}

	wg.Wait()

	// Some requests should be allowed
	if allowedCount == 0 {
		t.Error("expected some requests to be allowed")
	}
}

func TestRateLimiter_getLimiter_DoubleCheck(t *testing.T) {
	config := &RateLimitConfig{
		Enabled: true,
		Default: 60,
		Burst:   10,
	}

	rl := NewRateLimiter(config)

	// Get limiter twice for same tenant
	l1 := rl.getLimiter("tenant1")
	l2 := rl.getLimiter("tenant1")

	// Should return the same limiter instance
	if l1 != l2 {
		t.Error("getLimiter should return same instance for same tenant")
	}
}
