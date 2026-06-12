// Package main 是 Kongming SDK 最小可运行示例（main 入口）。
//
// 用法：
//
//	# 1. 启动（默认读 configs/kongming.yaml）
//	go run ./examples/quickstart/cmd
//
//	# 2. 指定自定义配置路径
//	KONGMING_CONFIG=./examples/quickstart/testdata/minimal.yaml go run ./examples/quickstart/cmd
//
// 本 main 仅做 4 件事：
//  1. 调用 quickstart.Run 装配 + 派单
//  2. 打印 BattleReport 概览
//  3. 错误时打 stderr 并 os.Exit(1)
//
// 核心装配逻辑写在 quickstart.Run 中（可被 test 复用），保持 main 极简。
//
// 目录布局说明：
//   - examples/quickstart/quickstart.go   装配+派单核心库（package quickstart）
//   - examples/quickstart/quickstart_test.go  单元测试（同 package）
//   - examples/quickstart/cmd/main.go     当前文件：main 入口（package main）
//
// 把 main 单独放进 cmd/ 子目录是因为 Go 强制同目录必须是同 package，
// 而 main 需要 `package main`，库代码则希望作为可被 import 的库存在。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/zhuge/kongming/examples/quickstart"
)

func main() {
	report, err := quickstart.Run(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "quickstart failed:", err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal report:", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
