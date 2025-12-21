package internal

import (
	"context"
	"fmt"
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
	engine  *gin.Engine
}

// NewServer 创建服务器
func NewServer(config *Config, proxy *Proxy, limiter *RateLimiter) *Server {
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
		engine:  engine,
	}

	s.setupRoutes()
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
	// 1. 鉴权
	if !s.authenticate(c) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	// 2. 限流
	tenantID := c.GetHeader("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	if !s.limiter.Allow(tenantID) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "rate limit exceeded",
		})
		return
	}

	// 3. 转发
	if err := s.proxy.Handle(c.Writer, c.Request); err != nil {
		// 错误已经在 proxy.Handle 中记录
		if !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": err.Error(),
			})
		}
	}
}

// authenticate 鉴权
func (s *Server) authenticate(c *gin.Context) bool {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return false
	}

	// Bearer token
	token := strings.TrimPrefix(auth, "Bearer ")
	for _, key := range s.config.Auth.APIKeys {
		if token == key {
			return true
		}
	}

	return false
}

// handleHealth 健康检查
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

// handleReady 就绪检查
func (s *Server) handleReady(c *gin.Context) {
	// TODO: 检查依赖项（Redis, ClickHouse）
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"time":   time.Now().Unix(),
	})
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Server.Port)
	fmt.Printf("Starting server on %s\n", addr)
	return s.engine.Run(addr)
}

// Shutdown 优雅关闭
func (s *Server) Shutdown(ctx context.Context) error {
	// Gin 没有内置 Shutdown，需要用标准库
	// 这里简化处理
	return nil
}
