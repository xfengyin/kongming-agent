// Kongming 服务器二进制入口：装配 + 启动 HTTP/gRPC，收到信号后优雅关闭。
//
// 设计要点：
//  1. 配置驱动：唯一参数 --config 指定 yaml 路径（默认 configs/kongming.yaml）。
//  2. 优雅停机：监听 SIGINT/SIGTERM，触发 ctx 取消 → Kongming.Shutdown。
//  3. 错误兜底：装配 / 运行失败时写 stderr 并以非零状态退出（避免 log.Fatalf）。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhuge/kongming/pkg/kongming"
)

func main() {
	// 1. 解析命令行参数。
	cfgPath := flag.String("config", "configs/kongming.yaml", "config file path")
	flag.Parse()

	// 2. 顶层装配（仅构造，不启动服务）。
	k, err := kongming.New(*cfgPath)
	if err != nil {
		// 装配阶段 logger 还没建好，直接 stderr + 退出码 1。
		fmt.Fprintf(os.Stderr, "kongming-server: init failed: %v\n", err)
		os.Exit(1)
	}

	// 3. 监听系统信号（SIGINT = Ctrl-C；SIGTERM = k8s 默认）。
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// 4. 启动并阻塞；Run 内部会处理 ctx 取消 → Shutdown。
	if err := k.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "kongming-server: run failed: %v\n", err)
		os.Exit(1)
	}
}
