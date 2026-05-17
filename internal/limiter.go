package internal

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// limiterIdleTTL 租户 limiter 空闲多久后被回收
	limiterIdleTTL = 10 * time.Minute
	// limiterSweepInterval 清理协程的扫描间隔
	limiterSweepInterval = 1 * time.Minute
)

// limiterEntry 单个租户的限流器及其最近活跃时间
type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter 基于 token bucket 的按租户限流器
type RateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.Mutex
	config   *RateLimitConfig
	stop     chan struct{}
	stopOnce sync.Once
}

// NewRateLimiter 创建限流器；启用时会启动单个后台清理协程
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*limiterEntry),
		config:   config,
		stop:     make(chan struct{}),
	}
	if config.Enabled {
		go rl.sweepLoop()
	}
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(tenantID string) bool {
	if !rl.config.Enabled {
		return true
	}
	return rl.getLimiter(tenantID).Allow()
}

// getLimiter 获取或创建租户 limiter，并刷新其活跃时间
func (rl *RateLimiter) getLimiter(tenantID string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[tenantID]
	if !exists {
		// rate.Limit 表示每秒请求数，Default 配置为每分钟，故除以 60
		r := rate.Limit(float64(rl.config.Default) / 60.0)
		entry = &limiterEntry{limiter: rate.NewLimiter(r, rl.config.Burst)}
		rl.limiters[tenantID] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// sweepLoop 后台清理协程：定期回收空闲 limiter，直到 Stop 被调用
func (rl *RateLimiter) sweepLoop() {
	ticker := time.NewTicker(limiterSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.sweep()
		}
	}
}

// sweep 回收超过空闲 TTL 未使用的 limiter，避免内存随租户数无限增长
func (rl *RateLimiter) sweep() {
	cutoff := time.Now().Add(-limiterIdleTTL)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for id, entry := range rl.limiters {
		if entry.lastSeen.Before(cutoff) {
			delete(rl.limiters, id)
		}
	}
}

// Stop 停止后台清理协程（幂等，供优雅关闭调用）
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stop)
	})
}
