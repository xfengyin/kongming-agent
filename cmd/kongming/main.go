// Kongming 命令行工具入口（CLI 客户端）。
//
// 设计要点：
//  1. 单一职责：本文件只负责 cobra 命令解析 + 错误码映射，不实现任何子命令逻辑；
//  2. 错误兜底：cobra 执行错误统一在 main 出口处理，子命令内不做 os.Exit；
//  3. 依赖倒置：所有子命令通过 cli.NewRootCmd() 注入 Service 容器（stage 5 注入 kongming 装配结果）。
package main

import (
	"fmt"
	"os"

	"github.com/zhuge/kongming/pkg/transport/cli"
)

func main() {
	root := cli.NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "kongming: %v\n", err)
		os.Exit(1)
	}
}
