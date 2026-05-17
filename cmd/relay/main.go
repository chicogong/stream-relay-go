package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chicogong/stream-relay-go/internal"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "configs/config.yaml", "Path to config file")
	flag.Parse()

	// 先加载配置（日志级别来自配置）
	config, err := internal.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config (%s): %v\n", *configPath, err)
		os.Exit(1)
	}

	// 初始化日志系统（应用配置中的日志级别）
	if _, err = internal.SetupLogger("logs", config.Observability.Logging.Level); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	slog.Info("Starting Stream Relay Go", "version", "1.0.0")
	slog.Info("Configuration loaded", "path", *configPath,
		"log_level", config.Observability.Logging.Level)

	// 初始化存储（暂时容错，允许存储失败）
	storage, err := internal.NewStorage(&config.Storage)
	if err != nil {
		slog.Warn("Storage initialization failed, continuing without storage",
			"error", err,
			"note", "转发功能仍然可用")
		storage = nil
	} else {
		slog.Info("Storage initialized successfully")
		defer storage.Close()
	}

	// 初始化指标
	metrics := internal.NewMetrics()
	slog.Info("Metrics initialized")

	// 初始化限流器
	limiter := internal.NewRateLimiter(&config.RateLimit)
	slog.Info("Rate limiter initialized",
		"enabled", config.RateLimit.Enabled,
		"default_rpm", config.RateLimit.Default,
		"burst", config.RateLimit.Burst)

	// 初始化代理
	proxy := internal.NewProxy(config, storage, metrics)
	slog.Info("Proxy initialized")

	// 初始化服务器
	server := internal.NewServer(config, proxy, limiter, storage)
	slog.Info("Server initialized", "port", config.Server.Port)

	// 启动服务器（在 goroutine 中）
	go func() {
		slog.Info("Starting HTTP server", "address", fmt.Sprintf(":%d", config.Server.Port))
		if err := server.Start(); err != nil {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// 等待信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("Received shutdown signal", "signal", sig.String())

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slog.Info("Shutting down server gracefully", "timeout", "30s")
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	slog.Info("Server stopped successfully")
}
