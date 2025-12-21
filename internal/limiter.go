package internal

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter 简单的限流器 - 基于 token bucket
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   *RateLimitConfig
}

// NewRateLimiter 创建限流器
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   config,
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(tenantID string) bool {
	if !rl.config.Enabled {
		return true
	}

	limiter := rl.getLimiter(tenantID)
	return limiter.Allow()
}

// getLimiter 获取或创建 limiter
func (rl *RateLimiter) getLimiter(tenantID string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[tenantID]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 双重检查
	if limiter, exists := rl.limiters[tenantID]; exists {
		return limiter
	}

	// 创建新的 limiter
	// rate.Limit 表示每秒的请求数
	r := rate.Limit(float64(rl.config.Default) / 60.0) // 转换为每秒
	limiter = rate.NewLimiter(r, rl.config.Burst)
	rl.limiters[tenantID] = limiter

	// 定期清理（可选）
	go rl.cleanup(tenantID)

	return limiter
}

// cleanup 清理不活跃的 limiter（避免内存泄漏）
func (rl *RateLimiter) cleanup(tenantID string) {
	time.Sleep(1 * time.Hour)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.limiters, tenantID)
}
