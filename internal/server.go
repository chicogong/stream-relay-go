package internal

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server HTTP 服务器
type Server struct {
	config  *Config
	proxy   *Proxy
	limiter *RateLimiter
	storage *Storage
	engine  *gin.Engine
	http    *http.Server
}

// NewServer 创建服务器
func NewServer(config *Config, proxy *Proxy, limiter *RateLimiter, storage *Storage) *Server {
	// 设置 Gin 模式
	if config.Observability.Logging.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		config:  config,
		proxy:   proxy,
		limiter: limiter,
		storage: storage,
		engine:  engine,
	}

	s.setupRoutes()

	s.http = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Server.Port),
		Handler: engine,
		// 防止慢速 header 攻击；请求体读取超时由 proxy 的 http.Client 控制
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// Prometheus metrics
	if s.config.Observability.Prometheus.Enabled {
		s.engine.GET(s.config.Observability.Prometheus.Path, gin.WrapH(promhttp.Handler()))
	}

	// 健康检查
	s.engine.GET("/healthz", s.handleHealth)
	s.engine.GET("/readyz", s.handleReady)

	// 代理路由 - 使用 NoRoute 处理所有未匹配的请求
	s.engine.NoRoute(s.handleProxy)
}

// handleProxy 处理代理请求
func (s *Server) handleProxy(c *gin.Context) {
	// 1. 鉴权，并从 API Key 派生租户标识
	tenantID, ok := s.authenticate(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	// 2. 限流（按租户；租户来自已鉴权的 API Key，客户端无法伪造）
	if !s.limiter.Allow(tenantID) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "rate limit exceeded",
		})
		return
	}

	// 3. 限制请求体大小（防止内存耗尽）
	if maxBytes := s.config.Server.MaxBodySize; maxBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	}

	// 4. 转发
	if err := s.proxy.Handle(c.Writer, c.Request, tenantID); err != nil {
		// 错误已经在 proxy.Handle 中记录
		if !c.Writer.Written() {
			status := http.StatusBadGateway
			if errors.Is(err, ErrRequestTooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			c.JSON(status, gin.H{
				"error": err.Error(),
			})
		}
	}
}

// authenticate 校验 Bearer API Key；成功时返回由该 Key 派生的租户标识
func (s *Server) authenticate(c *gin.Context) (string, bool) {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return "", false
	}

	// Bearer token
	token := strings.TrimPrefix(auth, "Bearer ")
	for _, key := range s.config.Auth.APIKeys {
		// 常量时间比较，避免计时侧信道
		if subtleConstantTimeEq(token, key) {
			return tenantFromKey(key), true
		}
	}

	return "", false
}

// tenantFromKey 从 API Key 派生稳定且不可逆的租户标识，
// 避免把原始 Key 写入日志/指标，同时保证同一 Key 始终映射到同一租户。
func tenantFromKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "t-" + hex.EncodeToString(sum[:4])
}

// handleHealth 健康检查 - 进程存活即返回 OK
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

// handleReady 就绪检查 - 校验可选依赖（Redis）是否可达
func (s *Server) handleReady(c *gin.Context) {
	if s.storage != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := s.storage.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  err.Error(),
				"time":   time.Now().Unix(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"time":   time.Now().Unix(),
	})
}

// Start 启动服务器（阻塞直到关闭）
func (s *Server) Start() error {
	slog.Info("HTTP server listening", "addr", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown 优雅关闭 - 停止接收新请求并等待进行中的请求完成
func (s *Server) Shutdown(ctx context.Context) error {
	s.limiter.Stop()
	return s.http.Shutdown(ctx)
}

// subtleConstantTimeEq 常量时间字符串比较，避免 API Key 鉴权的计时侧信道
func subtleConstantTimeEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
