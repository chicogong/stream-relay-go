package main

import (
	"context"
	"flag"
	"fmt"
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

	// 加载配置
	config, err := internal.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化存储（暂时容错，允许存储失败）
	storage, err := internal.NewStorage(&config.Storage)
	if err != nil {
		fmt.Printf("WARNING: Storage initialization failed: %v\n", err)
		fmt.Println("Continuing without storage (转发功能仍然可用)")
		storage = nil
	} else {
		defer storage.Close()
	}

	// 初始化指标
	metrics := internal.NewMetrics()

	// 初始化限流器
	limiter := internal.NewRateLimiter(&config.RateLimit)

	// 初始化代理
	proxy := internal.NewProxy(config, storage, metrics)

	// 初始化服务器
	server := internal.NewServer(config, proxy, limiter)

	// 启动服务器（在 goroutine 中）
	go func() {
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// 等待信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", err)
	}

	fmt.Println("Server stopped")
}
